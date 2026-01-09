// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
// If a copy of the MPL was not distributed with this file, You can obtain one at
// https://mozilla.org/MPL/2.0/.

package pfte

import (
	"fmt"
	"io"
	"os"
	"time"

	"fileripper/internal/network"
)

// UploadFile sends a local file to the remote server.
func UploadFile(session *network.SftpSession, localPath, remotePath string) error {
	fmt.Printf(">> Transfer: Uploading %s -> %s\n", localPath, remotePath)
	start := time.Now()

	// 1. Open Local
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// 2. Create Remote
	dst, err := session.SftpClient.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 3. The Transfer (Stream bytes)
	bytes, err := io.Copy(dst, src)
	if err != nil {
		return err
	}

	duration := time.Since(start)
	fmt.Printf(">> Transfer: Done. Sent %d bytes in %v.\n", bytes, duration)
	return nil
}

// DownloadFile pulls a remote file to the local disk.
func DownloadFile(session *network.SftpSession, remotePath, localPath string) error {
	fmt.Printf(">> Transfer: Downloading %s -> %s\n", remotePath, localPath)
	start := time.Now()

	// 1. Open Remote
	src, err := session.SftpClient.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	// 2. Create Local
	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 3. The Transfer
	bytes, err := io.Copy(dst, src)
	if err != nil {
		return err
	}

	// Force write to disk to ensure checksum will be valid later
	dst.Sync()

	duration := time.Since(start)
	fmt.Printf(">> Transfer: Done. Received %d bytes in %v.\n", bytes, duration)
	return nil
}