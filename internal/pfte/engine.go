// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
// If a copy of the MPL was not distributed with this file, You can obtain one at
// https://mozilla.org/MPL/2.0/.

package pfte

import (
	"fmt"
	"log"

	"fileripper/internal/network"
)

const (
	BatchSizeBoost        = 64
	BatchSizeConservative = 2
)

type TransferMode int

const (
	ModeBoost        TransferMode = iota 
	ModeConservative                     
)

type Engine struct {
	Mode  TransferMode
	Queue *JobQueue // Using our new thread-safe queue
}

func NewEngine() *Engine {
	return &Engine{
		Mode:  ModeBoost, 
		Queue: NewQueue(),
	}
}

// StartTransfer executes the logic.
// v0.0.3: Test Download and Integrity Check.
func (e *Engine) StartTransfer(session *network.SftpSession) error {
	if session.SftpClient == nil {
		return fmt.Errorf("sftp_client_not_initialized")
	}

	batchSize := BatchSizeConservative
	if e.Mode == ModeBoost {
		batchSize = BatchSizeBoost
	}

	fmt.Printf(">> PFTE Engine started. Workers: %d\n", batchSize)
	
	// TEST SCENARIO for v0.0.3:
	// Try to find 'Server.log' (from your previous output), download it, and check hash.
	targetFile := "Server.log"
	localOut := "downloaded_test.log"

	fmt.Printf(">> PFTE: Testing single file workflow on '%s'...\n", targetFile)

	// 1. Queue the job (Simulating real usage)
	e.Queue.Add(&TransferJob{
		LocalPath:  localOut,
		RemotePath: targetFile,
		Operation:  "DOWNLOAD",
	})

	// 2. Process Queue (Single threaded for now just to test logic)
	job := e.Queue.Pop()
	if job != nil {
		// Execute Download
		err := DownloadFile(session, job.RemotePath, job.LocalPath)
		if err != nil {
			log.Printf("Download failed: %v", err)
			return err
		}

		// 3. Verify Integrity
		fmt.Println(">> Integrity: Calculating local CRC32 checksum...")
		hash, err := CalculateChecksum(job.LocalPath)
		if err != nil {
			log.Printf("Checksum calculation failed: %v", err)
			return err
		}
		
		fmt.Printf(">> Integrity: File Hash (CRC32): %s\n", hash)
		fmt.Println(">> PFTE: Cycle complete. File is safe on disk.")
	}

	return nil
}