NovaSSH
A lightweight, cross-platform SSH terminal and SFTP manager written in Go.


Overview
NovaSSH is a self-contained desktop and web-based SSH/SFTP management application designed for Linux and Windows system administration. It combines an interactive terminal, file manager, and diagnostic tools into a single binary with minimal memory footprint and zero external runtime dependencies.

Features
Multi-Session SSH Terminal

Interactive terminal sessions (xterm.js + Go PTY) with 256-color support, resize handling, and session tabs.
Dedicated TCP connections per terminal session to prevent channel interference with background operations.
Configurable font size and dark theme palettes.
SFTP File Explorer

Full filesystem operations: file upload/download, directory creation, rename, and recursive delete.
Path navigation (.., ~, /, and direct address bar input).
Built-in remote text editor for configuration files (e.g., nginx.conf, .bashrc).
System Administration & Diagnostic Tools

Systemd Service Manager: Query, start, stop, and restart Linux systemd unit files.
Docker Engine Explorer: Inspect container status, ports, and execution logs over SSH.
Network & Firewall Scanner: Display active TCP/UDP listening sockets and security policies.
SSH Key Management: Generate local RSA-4096 / Ed25519 key pairs and deploy public keys to remote authorized_keys files.
Cluster Broadcast: Concurrently execute Linux commands across multiple selected server targets.
System Telemetry: Non-invasive monitoring of CPU utilization, memory, disk usage, and host OS distribution.
Internationalization (i18n)

Full localization support for 9 languages: English (en), Persian (fa), German (de), French (fr), Spanish (es), Russian (ru), Chinese (zh), Japanese (ja), and Arabic (ar).
Automatic RTL/LTR layout adjustment.
Desktop & Web Modes

Works as a standard local web server (0.0.0.0:8080) or opens directly as a native borderless application window on Windows and Linux using WebView2 / App Mode.
Getting Started
1. Download Releases
Pre-compiled binaries are available for Windows and Linux in the project archive (novassh-enterprise-v3.0.zip):


novassh-win-amd64.exe
 (Windows x64)

novassh-linux-amd64
 (Linux x64)
2. Run from Command Line
Bash

# Launch on default port 8080
./novassh-linux-amd64 -port=8080 -data=./data

# Run on Windows without launching desktop window
novassh-win-amd64.exe -port=8080 -desktop=false
3. Build from Source
Requirements:

Go 1.21 or later
Bash

# Clone repository
git clone https://github.com/novassh/novassh.git
cd novassh

# Build Linux binary
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o novassh-linux ./main.go

# Build Windows binary (without background command console)
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui" -o novassh.exe ./main.go
Configuration & Storage
Server profiles, custom command snippets, and preferences are stored as standard JSON files inside the specified data directory (default ./data):

servers.json: SSH connection profiles, host addresses, authentication methods, and tags.
snippets.json: Custom Linux command snippets.
Configuration data can be exported and imported as a JSON archive from the Settings tab.

License
This project is licensed under GPLv3.
