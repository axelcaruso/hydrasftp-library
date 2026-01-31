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

package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"hydrasftp/internal/network"
	"hydrasftp/internal/pfte"
)

// Global state for the API daemon.
// We keep the session alive so the UI can browse directories without reconnecting.
var (
	activeSession *network.SftpSession
	sessionMu     sync.Mutex
)

// StartDaemon initializes the local REST API.
// Flutter will talk to this port to command the Core.
func StartDaemon(port int) {
	fmt.Printf(">> Daemon: Starting REST API on localhost:%d...\n", port)

	// Auth & Session Management
	http.HandleFunc("/api/connect", handleConnect)
	http.HandleFunc("/api/disconnect", handleDisconnect)

	// File System Operations
	http.HandleFunc("/api/files", handleListFiles)

	// Real-time Monitoring
	http.HandleFunc("/api/progress", handleProgress)

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// This blocks forever
	log.Fatal(http.ListenAndServe(addr, nil))
}

// -- Request/Response Structs --

type ConnectRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type FileResponse struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

type ApiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// -- Handlers --

func handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, false, "Invalid JSON body", nil)
		return
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()

	// Clean up previous session if the user reconnects
	if activeSession != nil {
		activeSession.Close()
	}

	fmt.Printf(">> API: Connect request to %s@%s:%d\n", req.User, req.Host, req.Port)

	// 1. Init Session
	session := network.NewSession(req.Host, req.Port, req.User, req.Password)

	// 2. SSH Handshake
	if err := session.Connect(); err != nil {
		sendJSON(w, false, "Connection failed: "+err.Error(), nil)
		return
	}

	// 3. SFTP Subsystem
	if err := session.OpenSFTP(); err != nil {
		// Close SSH if SFTP fails
		session.Close()
		sendJSON(w, false, "SFTP subsystem failed: "+err.Error(), nil)
		return
	}

	activeSession = session
	sendJSON(w, true, "Connected successfully", nil)
}

func handleDisconnect(w http.ResponseWriter, r *http.Request) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if activeSession != nil {
		activeSession.Close()
		activeSession = nil
	}
	sendJSON(w, true, "Disconnected", nil)
}

func handleListFiles(w http.ResponseWriter, r *http.Request) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if activeSession == nil || activeSession.SftpClient == nil {
		sendJSON(w, false, "Not connected", nil)
		return
	}

	// Get path from query param (e.g., /api/files?path=/var/www)
	// Default to root (.)
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	fmt.Printf(">> API: Listing files in '%s'\n", path)

	files, err := activeSession.SftpClient.ReadDir(path)
	if err != nil {
		sendJSON(w, false, "Failed to list directory: "+err.Error(), nil)
		return
	}

	// Map generic FileInfo to JSON struct
	var fileList []FileResponse
	for _, f := range files {
		fileList = append(fileList, FileResponse{
			Name:  f.Name(),
			Size:  f.Size(),
			IsDir: f.IsDir(),
		})
	}

	sendJSON(w, true, "OK", fileList)
}

func handleProgress(w http.ResponseWriter, r *http.Request) {
	// Flutter will poll this endpoint frequently (e.g. 200ms).
	// We return a snapshot of the atomic counters from the engine.
	stats := pfte.GlobalMonitor.GetStats()

	sendJSON(w, true, "OK", stats)
}

// -- Helpers --

func sendJSON(w http.ResponseWriter, success bool, message string, data any) {
	w.Header().Set("Content-Type", "application/json")

	// Prevent CORS issues during local dev (Flutter web/debug)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(ApiResponse{
		Success: success,
		Message: message,
		Data:    data,
	})
}
