<p align="left">
  <img src="https://img.shields.io/badge/License-Apache%202.0-pink.svg" alt="License">
  <img src="https://img.shields.io/badge/Made%20with-Go-lightgreen.svg" alt="Go">
  <img src="https://img.shields.io/badge/Status-Alpha-blue.svg" alt="Status">
  <img src="https://img.shields.io/badge/Version-v0.3.0-orange.svg" alt="Version">
</p>

# What is HydraSFTP?

HydraSFTP is an open-source library that accelerates file uploads and downloads using the SFTP protocol. It is written in Go.

Although HydraSFTP is still in alpha, it already significantly increases upload and download speeds.

The HydraSFTP code is licensed under the Apache-2.0 license.

### **Important**

**HydraSFTP is still a very early-stage library.**

This means that its stability is not guaranteed, nor is it necessarily bad; it is simply a development version and still lacks many features before it can be used in production. BUT for testing, it works very well, as file corruption is negligible or incredibly low.

### Benchmarks

| Client | Data weight | Transferred folders | Transferred files | AVG Speed | Duration |
| :--- | :---: | :---: | :---: | :---: | :---: |
| HydraSFTP | 194.56 MiB | 497 | 3638 | 2.10 MiB/s | 1m 32s | 
| WinSCP | 194.56 MiB | 497 | 3638 | 0.21 MiB/s | 15m 33s |  

HydraSFTP is 10x faster

---

# Building HydraSFTP

<p align="left">
  <img src="https://img.shields.io/badge/Build-welcome-red.svg">
  <img src="https://img.shields.io/badge/Go%20-1.25.0 AMD64-purple.svg">
</p>

To compile the library, it is strongly recommended to use the Go version specified in the requirements.

Other versions are not verified for use.

## Prerequisites

Make sure you have the following tools installed on your operating system:

* Git: For version control.

* Go (1.25.0): To compile the core. [Download Go](https://go.dev/dl/).

---

## Compiling the Library

The main library is an executable that acts as a server or a CLI.

1. Go to the project root (where `go.mod` is located).

2. Install the necessary dependencies:
```bash
go mod tidy
```
3. Compile the production binary. We use flags to remove debug symbols and minimize the space used (this is for final versions).

```bash
go build -ldflags "-s -w" -o build/hydrasftp.exe ./cmd/hydrasftp
```

Result: If everything went well, you should see `hydrasftp.exe` (or the binary for your system) in your root directory. (It is recommended to compile for Windows at this time.)

---

# Terminal Usage Guide

## Execution Permissions in Linux
Before running the program in Linux, you must grant execute permissions to the binary. Otherwise, you will receive a "Permission denied" error. Run the following command:

```bash
chmod +x build/hydrasftp_linux
```

## Syntax

```bash
./[HydraSFTP_Executable] transfer <host> <port> <user> <password> <operation_flag>
```

### Parameters

| Parameter | Description |
| :--- | :--- |
| host | IP address or hostname of the remote server. |
| port | Remote SSH/SFTP port. |
| user | Remote SSH username. |
| password | Password for the remote user. | 

### Operation Flags
The command requires one of the following flags to define the transfer direction:

* `--upload <local_path>`: Recursively scans the specified local folder or file and uploads all contents to the remote directory. This enables Boost mode (128 concurrent workers).

* `--download <remote_path>`: Scans the specified remote path and downloads all contents to a local `./dump/` folder. This enables Boost mode (128 concurrent workers).

## Examples

### Upload
```bash
./hydrasftp_linux transfer 1.1.1.1 22 root mypassword --upload ./my_project
```

### Download
```bash
./hydrasftp_windows.exe transfer 1.1.1.1 22 root mypassword --download /example
```

---

### Feature Support and Roadmap

| Feature | Status | Notes |
| :--- | :---: | :--- |
| File Upload | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Boost mode active (64 workers). |
| File Download | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Downloads to the local */dump* folder. |
| SFTP Protocol | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Go-based implementation. |
| Transfer Stability | <img src="https://img.shields.io/badge/-%21-lightblue" height="20"> | 	Retry system (3 attempts) per worker. |
| Directory Creation | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Recursive tree creation supported. |
| SSH Keys (Key Authentication) | <img src="https://img.shields.io/badge/-%E2%9C%95-red" height="20"> | Password authentication only. |
| OS Compatibility | <img src="https://upload.wikimedia.org/wikipedia/commons/8/87/Windows_logo_-_2021.svg" width="15"> <img src="https://upload.wikimedia.org/wikipedia/commons/thumb/3/35/Tux.svg/960px-Tux.svg.png?20251226180606" width="15"> | Windows/Linux | 
---

