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

package main

import (
	"context"
	"fmt"
	"hydrasftp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hydrasftp/internal/core"
	"hydrasftp/internal/pfte"
	"hydrasftp/internal/server"
)

const SessionCount = 2 // Parallel SSH Sessions

func main() {
	fmt.Println("HydraSFTP v0.6.1 - Powered by PFTE ")

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "start-server":
		port := 2935
		if len(os.Args) > 2 {
			p, err := strconv.Atoi(os.Args[2])
			if err == nil {
				port = p
			}
		}
		server.StartDaemon(port)

	case "transfer":
		handleTransferCLI(os.Args)

	default:
		fmt.Printf("Error: %v: %s\n", core.ErrUnknownCommand, command)
		printUsage()
	}
}

func handleTransferCLI(args []string) {
	if len(args) < 6 {
		fmt.Println("Error: Missing arguments.")
		fmt.Println("Usage: hydrasftp transfer <host> <port> <user> <pass> [--upload <local> <remote_dest> | --download <remote>]")
		return
	}

	host := args[2]
	portStr := args[3]
	user := args[4]
	password := args[5]

	operation := "DOWNLOAD"
	sourcePath := "."
	destPath := "."

	if len(args) > 6 {
		mode := strings.ToLower(args[6])
		if mode == "--upload" {
			operation = "UPLOAD"
			if len(args) > 7 {
				sourcePath = args[7]
			}
			if len(args) > 8 {
				destPath = args[8]
			}
		} else if mode == "--download" {
			operation = "DOWNLOAD"
			if len(args) > 7 {
				rawPath := args[7]
				sourcePath = filepath.ToSlash(rawPath)
			}
		}
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Println("Error: Invalid port number.")
		return
	}

	targetDisplay := sourcePath
	if operation == "UPLOAD" {
		targetDisplay = fmt.Sprintf("%s -> %s", sourcePath, destPath)
	}

	fmt.Printf(">> CLI Mode: %s. Target: %s (%s@%s:%d)\n", operation, targetDisplay, user, host, port)

	// --- DUAL SESSION INIT ---
	var sessions []*hydrasftp.Session
	fmt.Printf(">> Network: Establishing %d parallel tunnels...\n", SessionCount)

	for i := 0; i < SessionCount; i++ {
		sess := hydrasftp.NewSession(host, port, user, password)
		if err := sess.Connect(); err != nil {
			fmt.Printf("Error connecting session #%d: %v\n", i+1, err)
			os.Exit(1)
		}
		sessions = append(sessions, sess)
	}

	defer func() {
		for _, s := range sessions {
			s.Close()
		}
	}()

	client := hydrasftp.NewClient()
	ctx := context.Background()
	startTime := time.Now()

	// --- CLI DASHBOARD GOROUTINE ---
	stopMonitor := make(chan bool)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopMonitor:
				return
			case <-ticker.C:
				stats := pfte.GlobalMonitor.GetStats()
				if stats.IsRunning {
					elapsed := time.Since(startTime).Round(time.Second)
					// \r moves cursor to start, \033[K clears the line forward to prevent stuttering
					fmt.Printf("\r\033[KTransferred: %s / %s, %.0f%%, %.2f MB/s, ETA %s | Files: %d/%d | %s",
						formatBytes(stats.BytesDone), formatBytes(stats.TotalBytes),
						stats.ProgressPercent, stats.SpeedMBs,
						calculateETA(stats.BytesDone, stats.TotalBytes, stats.SpeedMBs),
						stats.FilesDone, stats.TotalFiles,
						elapsed)
				}
			}
		}
	}()

	// Execute the Transfer
	errTransfer := client.Transfer(ctx, sessions, operation, sourcePath, destPath)

	// Stop monitor and wait a bit for the last print
	stopMonitor <- true
	time.Sleep(150 * time.Millisecond)

	// --- FINAL SUMMARY (Fixes the 2/3 bug and adds Rclone-style finish) ---
	stats := pfte.GlobalMonitor.GetStats()
	totalElapsed := time.Since(startTime).Round(time.Second)

	if errTransfer == nil {
		// Force the 100% display and correct file count
		fmt.Printf("\r\033[KTransferred: %s / %s, 100%%, %.2f MB/s, ETA 0s | Files: %d/%d | %s\n",
			formatBytes(stats.TotalBytes), formatBytes(stats.TotalBytes),
			stats.SpeedMBs, stats.TotalFiles, stats.TotalFiles, totalElapsed)

		fmt.Printf(">> Status: Finished %s successfully in %s.\n", strings.ToLower(operation), totalElapsed)
	} else {
		fmt.Printf("\n>> Status: Transfer failed after %s: %v\n", totalElapsed, errTransfer)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func calculateETA(done, total int64, speedMBs float64) string {
	if speedMBs <= 0 {
		return "---"
	}
	remainingBytes := total - done
	if remainingBytes <= 0 {
		return "0s"
	}
	seconds := float64(remainingBytes) / (speedMBs * 1024 * 1024)
	return time.Duration(seconds * float64(time.Second)).Round(time.Second).String()
}

func printUsage() {
	fmt.Println(`Usage: hydrasftp [command] [args]

Commands:
  start-server [port]   Start REST API Daemon
  transfer              <host> <port> <user> <pass> [--upload <local> <remote_dest> | --download <remote>]`)
}
