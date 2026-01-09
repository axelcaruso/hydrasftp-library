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
	Queue []string 
}

func NewEngine() *Engine {
	return &Engine{
		Mode:  ModeBoost, 
		Queue: make([]string, 0),
	}
}

// StartTransfer executes the main logic.
// For v0.0.2, we perform a "Discovery" pass to list remote files.
func (e *Engine) StartTransfer(session *network.SftpSession) error {
	// Sanity check
	if session.SftpClient == nil {
		return fmt.Errorf("sftp_client_not_initialized")
	}

	batchSize := BatchSizeConservative
	if e.Mode == ModeBoost {
		batchSize = BatchSizeBoost
	}

	fmt.Printf(">> PFTE Engine started. Workers: %d\n", batchSize)
	
	// 1. Discovery Phase (Test SFTP Protocol)
	fmt.Println(">> PFTE: Running Discovery on remote root (.)...")
	
	cwd, err := session.SftpClient.Getwd()
	if err != nil {
		log.Printf("Failed to get CWD: %v", err)
		return err
	}
	fmt.Printf(">> Remote CWD: %s\n", cwd)

	files, err := session.SftpClient.ReadDir(".")
	if err != nil {
		log.Printf("Failed to list directory: %v", err)
		return err
	}

	fmt.Printf(">> Discovery Complete. Found %d items:\n", len(files))
	
	// Just list the first 10 items to avoid spamming the console
	limit := 10
	if len(files) < limit {
		limit = len(files)
	}

	for i := 0; i < limit; i++ {
		file := files[i]
		icon := "📄"
		if file.IsDir() {
			icon = "ZE"
		}
		fmt.Printf("   %s %-20s %d bytes\n", icon, file.Name(), file.Size())
	}
	
	if len(files) > limit {
		fmt.Printf("   ... and %d more.\n", len(files)-limit)
	}

	return nil
}