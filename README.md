<p align="left">
  <img src="https://img.shields.io/badge/License-MPL%202.0-pink.svg" alt="License">
  <img src="https://img.shields.io/badge/Made%20with-Go-lightgreen.svg" alt="Go">
  <img src="https://img.shields.io/badge/Status-Alpha-blue.svg" alt="Status">
  <img src="https://img.shields.io/badge/Version-v0.3.0-orange.svg" alt="Version">
</p>

# What is FileRipper?

FileRipper is an open-source library that accelerates file uploads and downloads using the SFTP protocol. It is written in Go.

Although FileRipper is still in alpha, it already significantly increases upload and download speeds.

The FileRipper code is licensed under the Mozilla Public License 2.0 (MPL-2.0).

### **Important**

**FileRipper is still a very early-stage library.**

This means that its stability is not guaranteed, nor is it necessarily bad; it is simply a development version and still lacks many features before it can be used in production. BUT for testing, it works very well, as file corruption is negligible or incredibly low.

### Benchmarks

| Client | Data weight | Transferred folders | Transferred files | AVG Speed | Duration |
| :--- | :---: | :---: | :---: | :---: | :---: |
| FileRipper | 194.56 MiB | 497 | 3638 | 2.10 MiB/s | 1m 32s | 
| WinSCP | 194.56 MiB | 497 | 3638 | 0.21 MiB/s | 15m 33s |  

FileRipper is 10x faster

---

# Building FileRipper

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
go build -ldflags "-s -w" -o fileripper.exe ./cmd/fileripper
```

Result: If everything went well, you should see `fileripper.exe` (or the binary for your system) in your root directory. (It is recommended to compile for Windows at this time.)

---

# To use the program via the terminal:

## Syntax

```bash
./fileripper.exe transfer <host> <port> <user> <password> <operation_flag> [target]
```

Parameters (required)

- `<host>`: The IP address or hostname of the remote server.

- `<port>`: The remote SSH port.

- `<user>`: The remote SSH username.

- `<password>`: The user's password.

Operation flags (required)

The command requires one of the following flags to define the transfer direction:

- `--upload <local_folder_path>`: Recursively scans the local folder and uploads all contents to the remote root directory. This enables Boost mode (128 workers).

- `--download`: Downloads all files from the remote root directory. (/) to a local dump folder/. Enables Boost mode (128 workers).

### Feature Support and Roadmap

| Feature | Status | Notes |
| :--- | :---: | :--- |
| File Upload | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Boost mode active (64 workers). |
| File Download | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Downloads to the local */dump* folder. |
| SFTP Protocol | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Go-based implementation. |
| Transfer Stability | <img src="https://img.shields.io/badge/-%21-lightblue" height="20"> | 	Retry system (3 attempts) per worker. |
| Directory Creation | <img src="https://img.shields.io/badge/-%E2%9C%93-brightgreen" height="20"> | Recursive tree creation supported. |
| SSH Keys (Key Authentication) | <img src="https://img.shields.io/badge/-%E2%9C%95-red" height="20"> | Password authentication only. |
| OS Compatibility | <img src="https://upload.wikimedia.org/wikipedia/commons/8/87/Windows_logo_-_2021.svg" width="15"> | Primarily optimized for Windows. |
---

