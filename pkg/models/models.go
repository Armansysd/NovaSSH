package models

import "time"

// Server represents an SSH remote host configuration
type Server struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	Username       string    `json:"username"`
	AuthType       string    `json:"auth_type"` // "password" or "key"
	Password       string    `json:"password,omitempty"`
	PrivateKey     string    `json:"private_key,omitempty"` // Path or content of RSA/Ed25519 key
	Passphrase     string    `json:"passphrase,omitempty"`
	Tags           []string  `json:"tags"`
	Group          string    `json:"group"` // e.g. "Production", "Staging", "Lab"
	Notes          string    `json:"notes,omitempty"`
	Status         string    `json:"status"` // "online", "offline", "unknown"
	LatencyMS      int64     `json:"latency_ms"`
	LastConnected  time.Time `json:"last_connected,omitempty"`
}

// Snippet represents a reusable SSH command snippet
type Snippet struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Command     string   `json:"command"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
}

// SystemStats holds real-time Linux server metrics
type SystemStats struct {
	ServerID    string  `json:"server_id"`
	Hostname    string  `json:"hostname"`
	OSInfo      string  `json:"os_info"`
	Uptime      string  `json:"uptime"`
	CPUPercent  float64 `json:"cpu_percent"`
	RAMTotalMB  float64 `json:"ram_total_mb"`
	RAMUsedMB   float64 `json:"ram_used_mb"`
	RAMPercent  float64 `json:"ram_percent"`
	DiskTotalGB float64 `json:"disk_total_gb"`
	DiskUsedGB  float64 `json:"disk_used_gb"`
	DiskPercent float64 `json:"disk_percent"`
	LoadAvg     string  `json:"load_avg"`
	Error       string  `json:"error,omitempty"`
}

// SFTPFile represents a remote file or folder in SFTP
type SFTPFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SizeStr     string `json:"size_str"`
	IsDir       bool   `json:"is_dir"`
	ModTime     string `json:"mod_time"`
	Permissions string `json:"permissions"`
}

// SFTPListResult wraps the current directory path, parent path, and sorted files
type SFTPListResult struct {
	CurrentPath string     `json:"current_path"`
	ParentPath  string     `json:"parent_path"`
	Files       []SFTPFile `json:"files"`
}

// TerminalSize holds terminal columns and rows for resize
type TerminalSize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// PingResult represents the outcome of a server health check
type PingResult struct {
	ID        string `json:"id"`
	Online    bool   `json:"online"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// ServiceInfo holds Linux systemd unit details
type ServiceInfo struct {
	Name        string `json:"name"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
}

// DockerContainer holds Docker container details
type DockerContainer struct {
	ID      string `json:"id"`
	Names   string `json:"names"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Ports   string `json:"ports"`
	State   string `json:"state"`
}

// PortInfo holds listening TCP/UDP port details
type PortInfo struct {
	Proto       string `json:"proto"`
	LocalAddr   string `json:"local_addr"`
	ForeignAddr string `json:"foreign_addr"`
	State       string `json:"state"`
	Process     string `json:"process"`
}

// SSHKeyPair holds locally generated RSA/Ed25519 keys
type SSHKeyPair struct {
	KeyType    string `json:"key_type"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// ClusterExecResult represents multi-server broadcast command execution
type ClusterExecResult struct {
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}
