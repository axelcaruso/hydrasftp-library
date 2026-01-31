/*
 * Copyright 2026 The FileRipper Team
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pfte

import (
	"context"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"sync"
	"time"

	"hydrasftp/internal/network"
)

const (
	BufferSize         = 64 * 1024        // 64KB for standard streams
	MultipartThreshold = 10 * 1024 * 1024 // 10MB. Files larger than this get split.
	MultipartChunks    = 16               // The user requested 16 chunks for the tail file.
)

// ProgressTracker wraps an io.Reader to update monitor and compute hash simultaneously.
type ProgressTracker struct {
	Reader io.Reader
	Hasher hash.Hash32
	// Lock needed because multiple chunks might update metrics concurrently
	Mu sync.Mutex
}

func (pt *ProgressTracker) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	if n > 0 {
		GlobalMonitor.AddBytes(int64(n))

		// Hasher is not thread-safe, so if we used this in multipart we'd need locking.
		// For multipart, we might skip hashing or handle it differently.
		// For now, we lock just in case.
		pt.Mu.Lock()
		pt.Hasher.Write(p[:n])
		pt.Mu.Unlock()
	}
	return n, err
}

// DownloadFileWithProgress pulls a remote file safely.
func DownloadFileWithProgress(ctx context.Context, session *network.SftpSession, remotePath, localPath string) error {
	var lastErr error
	buf := make([]byte, BufferSize)

	for attempt := 1; attempt <= 3; attempt++ {
		lastErr = func() error {
			src, err := session.SftpClient.Open(remotePath)
			if err != nil {
				return err
			}
			defer src.Close()

			dst, err := os.Create(localPath)
			if err != nil {
				return err
			}
			defer dst.Close()

			tracker := &ProgressTracker{
				Reader: src,
				Hasher: crc32.NewIEEE(),
			}

			// (We use a custom copy loop to support context cancellation)
			_, err = copyWithContext(ctx, dst, tracker, buf)
			if err != nil {
				return err
			}

			// Preserve mtime if possible
			if stat, err := session.SftpClient.Stat(remotePath); err == nil {
				_ = os.Chtimes(localPath, time.Now(), stat.ModTime())
			}
			return nil
		}()

		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

// UploadFileWithProgress decides whether to use Single Stream or Multipart Swarm.
func UploadFileWithProgress(ctx context.Context, session *network.SftpSession, localPath, remotePath string) error {
	// 1. Check file size
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	fileSize := info.Size()

	// 2. Decision Matrix
	if fileSize >= MultipartThreshold {
		// Try Multipart upload for large files to kill the "tail effect"
		err := uploadMultipart(ctx, session, localPath, remotePath, fileSize)
		if err == nil {
			return nil
		}
		// If multipart fails (e.g. server locks file), fall back silently to single stream
		// fmt.Printf(">> Turbo Failed (%v). Reverting to single stream.\n", err)
	}

	// 3. Fallback / Standard Upload
	return uploadSingleStream(ctx, session, localPath, remotePath)
}

// uploadSingleStream is the robust, standard upload logic.
func uploadSingleStream(ctx context.Context, session *network.SftpSession, localPath, remotePath string) error {
	var lastErr error
	buf := make([]byte, BufferSize)

	for attempt := 1; attempt <= 3; attempt++ {
		lastErr = func() error {
			src, err := os.Open(localPath)
			if err != nil {
				return err
			}
			defer src.Close()

			info, err := src.Stat()
			if err != nil {
				return err
			}

			dst, err := session.SftpClient.Create(remotePath)
			if err != nil {
				return err
			}
			defer dst.Close()

			tracker := &ProgressTracker{
				Reader: src,
				Hasher: crc32.NewIEEE(),
			}

			_, err = copyWithContext(ctx, dst, tracker, buf)
			if err != nil {
				return err
			}

			// Sync timestamps and permissions
			_ = session.SftpClient.Chtimes(remotePath, time.Now(), info.ModTime())
			_ = session.SftpClient.Chmod(remotePath, info.Mode())

			return nil
		}()

		if lastErr == nil {
			break
		}
	}
	return lastErr
}

// uploadMultipart splits the file and uploads parts in parallel.
func uploadMultipart(ctx context.Context, session *network.SftpSession, localPath, remotePath string, size int64) error {
	// Calculate chunk size
	chunkSize := size / int64(MultipartChunks)

	// Create the remote file once to ensure it exists and is truncated
	f, err := session.SftpClient.Create(remotePath)
	if err != nil {
		return err
	}
	f.Close() // Close it, workers will open their own handles

	var wg sync.WaitGroup
	errChan := make(chan error, MultipartChunks)

	// Launch 16 mini-workers
	for i := 0; i < MultipartChunks; i++ {
		wg.Add(1)

		start := int64(i) * chunkSize
		end := start + chunkSize
		if i == MultipartChunks-1 {
			end = size // Last chunk takes the remainder
		}

		go func(offset, length int64) {
			defer wg.Done()

			// Each worker needs its own file handle for thread safety on Seek
			// NOTE: Some SFTP servers dislike multiple handles to the same file.
			remoteFile, err := session.SftpClient.OpenFile(remotePath, os.O_WRONLY)
			if err != nil {
				errChan <- err
				return
			}
			defer remoteFile.Close()

			localFile, err := os.Open(localPath)
			if err != nil {
				errChan <- err
				return
			}
			defer localFile.Close()

			// Seek to position
			_, err = remoteFile.Seek(offset, 0)
			if err != nil {
				errChan <- err
				return
			}
			_, err = localFile.Seek(offset, 0)
			if err != nil {
				errChan <- err
				return
			}

			// Limit the reader to this chunk's length
			partReader := io.LimitReader(localFile, length-offset) // logic fix below
			// Actually LimitReader takes size, not end pos.
			partReader = io.LimitReader(localFile, length)

			// We wrap for stats updating
			// Note: Hasher is skipped in multipart for speed/complexity reasons
			// (Merging 16 partial hashes is complex). Integrity relies on TCP/SSH here.
			buf := make([]byte, 32*1024)

			// Custom copy loop to update monitor
			for {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
				}

				n, readErr := partReader.Read(buf)
				if n > 0 {
					// Write to remote
					_, writeErr := remoteFile.Write(buf[:n])
					if writeErr != nil {
						errChan <- writeErr
						return
					}
					// Update global stats
					GlobalMonitor.AddBytes(int64(n))
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					errChan <- readErr
					return
				}
			}
		}(start, end-start)
	}

	wg.Wait()
	close(errChan)

	// If any chunk failed, return error so we fall back to single stream
	if len(errChan) > 0 {
		return <-errChan
	}

	// Sync metadata after successful multipart swarm
	if info, err := os.Stat(localPath); err == nil {
		_ = session.SftpClient.Chtimes(remotePath, time.Now(), info.ModTime())
		_ = session.SftpClient.Chmod(remotePath, info.Mode())
	}

	return nil
}

// Legacy wrappers (now with context)
func UploadFile(ctx context.Context, session *network.SftpSession, localPath, remotePath string) error {
	return UploadFileWithProgress(ctx, session, localPath, remotePath)
}

func DownloadFile(ctx context.Context, session *network.SftpSession, remotePath, localPath string) error {
	return DownloadFileWithProgress(ctx, session, remotePath, localPath)
}

// copyWithContext is a helper to allow cancellation during io.Copy
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	var written int64
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}
