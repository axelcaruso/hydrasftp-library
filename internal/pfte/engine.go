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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"hydrasftp/internal/network"

	"github.com/pkg/sftp"
)

const (
	BatchSizeBoost        = 128
	BatchSizeConservative = 4
	DirCreationWorkers    = 8
)

type TransferMode int

const (
	ModeBoost TransferMode = iota
	ModeConservative
)

type Engine struct {
	Mode  TransferMode
	Queue *JobQueue
}

func NewEngine() *Engine {
	return &Engine{
		Mode:  ModeBoost,
		Queue: NewQueue(),
	}
}

// StartTransfer handles the heavy lifting (source can be local or remote)
func (e *Engine) StartTransfer(ctx context.Context, sessions []*network.SftpSession, operation string, sourcePath string, destPath string) error {
	if len(sessions) == 0 || sessions[0].SftpClient == nil {
		return fmt.Errorf("no_active_sessions")
	}
	mainSession := sessions[0]

	concurrency := BatchSizeConservative
	if e.Mode == ModeBoost {
		concurrency = BatchSizeBoost
	}

	// --- UPLOAD LOGIC ---
	if operation == "UPLOAD" {
		absSource, err := filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %v", err)
		}

		// Base dir is the parent of the source folder (e.g., C:\Users\...)
		baseDir := filepath.Dir(absSource)

		var foldersToCreate []string
		var filesToTransfer []*TransferJob
		totalBytes := int64(0)

		err = filepath.Walk(absSource, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // (We skip errors to keep the flow going)
			}

			// Handle Symlinks: We follow them to keep it simple across OS
			if info.Mode()&os.ModeSymlink != 0 {
				resolvedPath, err := filepath.EvalSymlinks(p)
				if err != nil {
					return nil
				}
				info, err = os.Stat(resolvedPath)
				if err != nil {
					return nil
				}
			}

			// Calculate relative path from local base
			relPath, err := filepath.Rel(baseDir, p)
			if err != nil {
				return err
			}

			// Cross-platform path normalization: SFTP always wants forward slashes
			remoteRel := filepath.ToSlash(relPath)
			finalRemotePath := path.Join(destPath, remoteRel)

			if info.IsDir() {
				if remoteRel != "." && remoteRel != "" {
					foldersToCreate = append(foldersToCreate, finalRemotePath)
				}
			} else {
				filesToTransfer = append(filesToTransfer, &TransferJob{
					LocalPath:  p,
					RemotePath: finalRemotePath,
					Operation:  "UPLOAD",
				})
				totalBytes += info.Size()
			}
			return nil
		})
		if err != nil {
			return err
		}

		sort.Slice(foldersToCreate, func(i, j int) bool {
			return len(foldersToCreate[i]) < len(foldersToCreate[j])
		})

		dirCount := len(foldersToCreate)
		if dirCount > 0 {
			dirChan := make(chan string, dirCount)
			var wg sync.WaitGroup
			var doneCount int32
			for _, d := range foldersToCreate {
				dirChan <- d
			}
			close(dirChan)

			for i := 0; i < DirCreationWorkers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for dir := range dirChan {
						select {
						case <-ctx.Done():
							return
						default:
							_ = mainSession.SftpClient.MkdirAll(dir)
							atomic.AddInt32(&doneCount, 1)
						}
					}
				}()
			}
			wg.Wait()
		}

		fileCount := int64(len(filesToTransfer))
		if fileCount == 0 {
			return nil
		}

		for _, job := range filesToTransfer {
			e.Queue.Add(job)
		}
		GlobalMonitor.Reset(fileCount, totalBytes)

		workerPool := NewWorkerPool(concurrency, e.Queue)
		workerPool.StartUnleash(ctx, sessions)
		return nil

		// --- DOWNLOAD LOGIC ---
	} else {
		return e.startDownload(ctx, sessions, mainSession, concurrency, sourcePath)
	}
}

// Helper to keep the file clean
func (e *Engine) startDownload(ctx context.Context, sessions []*network.SftpSession, mainSession *network.SftpSession, concurrency int, targetPath string) error {
	localBase := "dump"
	if _, err := os.Stat(localBase); os.IsNotExist(err) {
		os.Mkdir(localBase, 0755)
	}

	targetName := path.Base(targetPath)
	if targetPath == "" || targetPath == "." {
		targetName = ""
	}
	remoteSource := targetPath
	if remoteSource == "" {
		remoteSource = "."
	}

	info, err := mainSession.SftpClient.Stat(remoteSource) // (We follow symlinks if the target is one)

	if err != nil && targetName != "" {
		foundPath := findRemotePath(mainSession.SftpClient, ".", targetName, 3)
		if foundPath != "" {
			remoteSource = foundPath
			info, err = mainSession.SftpClient.Stat(remoteSource)
		} else {
			return fmt.Errorf("target not found")
		}
	}
	if err != nil {
		return err
	}

	queuedCount := int64(0)
	totalBytes := int64(0)

	walker := mainSession.SftpClient.Walk(remoteSource)
	for walker.Step() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if walker.Err() != nil {
			continue
		}
		remotePath := walker.Path()
		stat := walker.Stat()

		relPath, err := filepath.Rel(remoteSource, remotePath)
		if err != nil {
			relPath = filepath.Base(remotePath)
		}
		rootDirName := filepath.Base(remoteSource)
		if remoteSource == "." || remoteSource == "/" {
			rootDirName = "root_dump"
		}
		localPath := filepath.Join(localBase, rootDirName, relPath)

		if !info.IsDir() && remotePath == remoteSource {
			localPath = filepath.Join(localBase, rootDirName)
		}

		if stat.Mode()&os.ModeSymlink != 0 {
			realStat, err := mainSession.SftpClient.Stat(remotePath)
			if err != nil {
				continue
			}
			stat = realStat
		}

		if stat.IsDir() {
			os.MkdirAll(localPath, 0755)
			continue
		}

		e.Queue.Add(&TransferJob{
			LocalPath:  localPath,
			RemotePath: remotePath,
			Operation:  "DOWNLOAD",
		})
		queuedCount++
		totalBytes += stat.Size()
	}

	GlobalMonitor.Reset(queuedCount, totalBytes)

	if queuedCount > 0 {
		workerPool := NewWorkerPool(concurrency, e.Queue)
		workerPool.StartUnleash(ctx, sessions)
	}
	return nil
}

func findRemotePath(client *sftp.Client, root, targetName string, maxDepth int) string {
	if maxDepth < 0 {
		return ""
	}
	files, err := client.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, f := range files {
		if strings.EqualFold(f.Name(), targetName) {
			return path.Join(root, f.Name())
		}
	}
	for _, f := range files {
		if f.IsDir() {
			found := findRemotePath(client, path.Join(root, f.Name()), targetName, maxDepth-1)
			if found != "" {
				return found
			}
		}
	}
	return ""
}

func (e *Engine) UploadSpecificFile(ctx context.Context, sessions []*network.SftpSession, local, remote string) error {
	if len(sessions) == 0 || sessions[0].SftpClient == nil {
		return fmt.Errorf("no_active_sessions")
	}
	st, err := os.Stat(local)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("source_is_directory")
	}
	e.Queue.Add(&TransferJob{
		LocalPath:  local,
		RemotePath: remote,
		Operation:  "UPLOAD",
	})
	GlobalMonitor.Reset(1, st.Size())
	c := BatchSizeConservative
	if e.Mode == ModeBoost {
		c = BatchSizeBoost
	}
	NewWorkerPool(c, e.Queue).StartUnleash(ctx, sessions)
	return nil
}

func (e *Engine) DownloadSpecificFile(ctx context.Context, sessions []*network.SftpSession, remote, local string) error {
	if len(sessions) == 0 || sessions[0].SftpClient == nil {
		return fmt.Errorf("no_active_sessions")
	}
	st, err := sessions[0].SftpClient.Stat(remote)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("remote_is_directory")
	}
	ld := filepath.Dir(local)
	if _, err := os.Stat(ld); os.IsNotExist(err) {
		os.MkdirAll(ld, 0755)
	}
	e.Queue.Add(&TransferJob{
		LocalPath:  local,
		RemotePath: remote,
		Operation:  "DOWNLOAD",
	})
	GlobalMonitor.Reset(1, st.Size())
	c := BatchSizeConservative
	if e.Mode == ModeBoost {
		c = BatchSizeBoost
	}
	NewWorkerPool(c, e.Queue).StartUnleash(ctx, sessions)
	return nil
}
