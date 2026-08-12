package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/novassh/novassh/pkg/desktop"
	"github.com/novassh/novassh/pkg/models"
	"github.com/novassh/novassh/pkg/sftpclient"
	"github.com/novassh/novassh/pkg/sshclient"
	"github.com/novassh/novassh/pkg/storage"
)

//go:embed all:web/static
var embeddedWeb embed.FS

// Version build tag
var Version = "4.2-Enterprise-PRO"

type App struct {
	storage       *storage.Manager
	sshManager    *sshclient.Manager
	sftpClient    *sftpclient.ClientManager
	lastHeartbeat time.Time
	muHeartbeat   sync.Mutex
	hasConnected  bool
}

func main() {
	port := flag.Int("port", 8080, "Port for NovaSSH Enterprise Server")
	dataDir := flag.String("data", "./data", "Directory to store configuration json files")
	desktopMode := flag.Bool("desktop", true, "Launch native desktop application window on startup")
	flag.Parse()

	log.SetOutput(os.Stdout)
	log.Println("================================================================")
	log.Printf("   🚀 NovaSSH Enterprise Studio (%s) - SSH & SFTP Suite   ", Version)
	log.Println("   10+ Real DevOps Tools • True OLED Dark Mode • Native Desktop   ")
	log.Println("================================================================")

	localURL := fmt.Sprintf("http://127.0.0.1:%d", *port)

	// Port collision check: if an instance is already running on port 8080,
	// simply open a new app window pointing to the existing instance and exit cleanly.
	if isPortBusy(*port) {
		if *desktopMode {
			log.Printf("[Desktop] Existing NovaSSH server detected on port %d. Opening window...", *port)
			desktop.OpenWindow(localURL, 1280, 860, true)
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}
		log.Fatalf("[Error] Port %d is already in use by another process.", *port)
	}

	store, err := storage.NewManager(*dataDir)
	if err != nil {
		log.Fatalf("Error initializing storage directory: %v", err)
	}

	sm := sshclient.NewManager()
	cm := sftpclient.NewClientManager(sm)

	app := &App{
		storage:       store,
		sshManager:    sm,
		sftpClient:    cm,
		lastHeartbeat: time.Now(),
	}

	// Background Goroutine: Periodic health ping of all servers every 30s
	go app.startBackgroundMonitor()

	// Background Goroutine: Desktop window lifecycle monitor (Auto-shutdown when window closes)
	if *desktopMode {
		go app.startLifecycleMonitor()
	}

	// HTTP Routing
	mux := http.NewServeMux()

	// Static files (try physical directory first for instant development updates)
	fsys, err := fs.Sub(embeddedWeb, "web/static")
	if err != nil {
		log.Fatalf("embedded web fs error: %v", err)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat("web/static"); err == nil {
			http.FileServer(http.Dir("web/static")).ServeHTTP(w, r)
			return
		}
		http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
	})

	// API endpoints
	mux.HandleFunc("/api/servers", app.handleServers)
	mux.HandleFunc("/api/servers/ping", app.handlePingServer)
	mux.HandleFunc("/api/servers/ping-all", app.handlePingAll)
	mux.HandleFunc("/api/snippets", app.handleSnippets)
	mux.HandleFunc("/api/monitor", app.handleMonitor)
	mux.HandleFunc("/api/command", app.handleRunCommand)

	// Lifecycle & Heartbeat endpoints
	mux.HandleFunc("/api/heartbeat", app.handleHeartbeat)
	mux.HandleFunc("/api/shutdown", app.handleShutdown)

	// New 10+ Practical DevOps & Administration Endpoints
	mux.HandleFunc("/api/services", app.handleListServices)
	mux.HandleFunc("/api/services/action", app.handleServiceAction)
	mux.HandleFunc("/api/docker/containers", app.handleListContainers)
	mux.HandleFunc("/api/docker/action", app.handleContainerAction)
	mux.HandleFunc("/api/ports", app.handleListPorts)
	mux.HandleFunc("/api/logs/tail", app.handleTailLogs)
	mux.HandleFunc("/api/process/kill", app.handleKillProcess)
	mux.HandleFunc("/api/ssh/keygen", app.handleSSHKeyGen)
	mux.HandleFunc("/api/ssh/authorized_keys", app.handleAuthorizedKeys)
	mux.HandleFunc("/api/ssh/authorized_keys/add", app.handleAddAuthorizedKey)
	mux.HandleFunc("/api/cluster/exec", app.handleClusterExec)

	// Backup Export / Import
	mux.HandleFunc("/api/export", app.handleExportBackup)
	mux.HandleFunc("/api/import", app.handleImportBackup)

	// SFTP endpoints
	mux.HandleFunc("/api/sftp/list", app.handleSFTPList)
	mux.HandleFunc("/api/sftp/upload", app.handleSFTPUpload)
	mux.HandleFunc("/api/sftp/download", app.handleSFTPDownload)
	mux.HandleFunc("/api/sftp/mkdir", app.handleSFTPMkdir)
	mux.HandleFunc("/api/sftp/touch", app.handleSFTPTouch)
	mux.HandleFunc("/api/sftp/delete", app.handleSFTPDelete)
	mux.HandleFunc("/api/sftp/rename", app.handleSFTPRename)
	mux.HandleFunc("/api/sftp/read", app.handleSFTPRead)
	mux.HandleFunc("/api/sftp/write", app.handleSFTPWrite)

	// WebSocket Terminal
	mux.HandleFunc("/api/ws/terminal", app.handleTerminalWebSocket)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("🌟 [NovaSSH] Enterprise Server listening on http://%s", addr)

	// Launch Native Desktop Application Window
	desktop.OpenWindow(localURL, 1280, 860, *desktopMode)

	if err := http.ListenAndServe(addr, enableCORS(mux)); err != nil {
		log.Fatalf("Web server error: %v", err)
	}
}

func isPortBusy(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

func (a *App) startLifecycleMonitor() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		a.muHeartbeat.Lock()
		connected := a.hasConnected
		last := a.lastHeartbeat
		a.muHeartbeat.Unlock()

		// Once the desktop app window has sent at least one heartbeat,
		// if 15 seconds pass without a heartbeat, close the backend server.
		if connected && time.Since(last) > 15*time.Second {
			log.Println("[Desktop] App window closed by user (no heartbeat). Shutting down NovaSSH server...")
			os.Exit(0)
		}
	}
}

func (a *App) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	a.muHeartbeat.Lock()
	a.lastHeartbeat = time.Now()
	a.hasConnected = true
	a.muHeartbeat.Unlock()
	sendJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleShutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("[Desktop] Shutdown beacon received from window close. Exiting...")
	sendJSON(w, http.StatusOK, map[string]string{"status": "shutting_down"})
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}()
}

// startBackgroundMonitor pings servers concurrently using Goroutines
func (a *App) startBackgroundMonitor() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		servers := a.storage.GetServers()
		var wg sync.WaitGroup
		for _, s := range servers {
			wg.Add(1)
			go func(srv *models.Server) {
				defer wg.Done()
				res := a.sshManager.PingServer(srv)
				status := "offline"
				if res.Online {
					status = "online"
				}
				a.storage.UpdateServerStatus(srv.ID, status, res.LatencyMS)
			}(s)
		}
		wg.Wait()
	}
}

// CORS middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sendJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func sendError(w http.ResponseWriter, status int, msg string) {
	sendJSON(w, status, map[string]string{"error": msg})
}

// ---- Core API Handlers ----

func (a *App) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		servers := a.storage.GetServers()
		sendJSON(w, http.StatusOK, servers)
	case http.MethodPost:
		var s models.Server
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			sendError(w, http.StatusBadRequest, "Invalid server payload")
			return
		}
		if s.AuthType == "" {
			s.AuthType = "password"
		}
		if s.Group == "" {
			s.Group = "Default"
		}
		s.Status = "unknown"
		if err := a.storage.SaveServer(&s); err != nil {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, s)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			sendError(w, http.StatusBadRequest, "Server id required")
			return
		}
		a.sshManager.CloseConnection(id)
		if err := a.storage.DeleteServer(id); err != nil {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (a *App) handlePingServer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	res := a.sshManager.PingServer(s)
	status := "offline"
	if res.Online {
		status = "online"
	}
	a.storage.UpdateServerStatus(s.ID, status, res.LatencyMS)
	sendJSON(w, http.StatusOK, res)
}

func (a *App) handlePingAll(w http.ResponseWriter, r *http.Request) {
	servers := a.storage.GetServers()
	results := make([]models.PingResult, len(servers))
	var wg sync.WaitGroup
	for i, s := range servers {
		wg.Add(1)
		go func(idx int, srv *models.Server) {
			defer wg.Done()
			res := a.sshManager.PingServer(srv)
			status := "offline"
			if res.Online {
				status = "online"
			}
			a.storage.UpdateServerStatus(srv.ID, status, res.LatencyMS)
			results[idx] = res
		}(i, s)
	}
	wg.Wait()
	sendJSON(w, http.StatusOK, results)
}

func (a *App) handleSnippets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		snippets := a.storage.GetSnippets()
		sendJSON(w, http.StatusOK, snippets)
	case http.MethodPost:
		var sn models.Snippet
		if err := json.NewDecoder(r.Body).Decode(&sn); err != nil {
			sendError(w, http.StatusBadRequest, "Invalid snippet payload")
			return
		}
		if err := a.storage.SaveSnippet(&sn); err != nil {
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, sn)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		_ = a.storage.DeleteSnippet(id)
		sendJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func (a *App) handleMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	stats, err := a.sshManager.CollectStats(s)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, stats)
}

func (a *App) handleRunCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		Command  string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	out, err := a.sshManager.RunCommand(s, req.Command)
	if err != nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"output": out,
			"error":  err.Error(),
		})
		return
	}
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"output": out,
		"error":  "",
	})
}

// ---- New 10+ Practical DevOps Handlers ----

func (a *App) handleListServices(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	services, err := a.sshManager.ListServices(s)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, services)
}

func (a *App) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		Action   string `json:"action"` // start, stop, restart, status
		Service  string `json:"service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	out, err := a.sshManager.ServiceAction(s, req.Action, req.Service)
	if err != nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"output": out,
			"error":  err.Error(),
		})
		return
	}
	sendJSON(w, http.StatusOK, map[string]interface{}{"output": out, "error": ""})
}

func (a *App) handleListContainers(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	containers, err := a.sshManager.ListContainers(s)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, containers)
}

func (a *App) handleContainerAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID    string `json:"server_id"`
		Action      string `json:"action"` // start, stop, restart, logs
		ContainerID string `json:"container_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	out, err := a.sshManager.ContainerAction(s, req.Action, req.ContainerID)
	if err != nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{"output": out, "error": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]interface{}{"output": out, "error": ""})
}

func (a *App) handleListPorts(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	ports, err := a.sshManager.ListListeningPorts(s)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, ports)
}

func (a *App) handleTailLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	file := r.URL.Query().Get("file")
	if file == "" {
		file = "/var/log/syslog"
	}
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	out, err := a.sshManager.TailLogFile(s, file, 50)
	if err != nil {
		sendJSON(w, http.StatusOK, map[string]string{"output": out, "error": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"output": out, "error": ""})
}

func (a *App) handleKillProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		PID      string `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	out, err := a.sshManager.KillProcess(s, req.PID)
	if err != nil {
		sendJSON(w, http.StatusOK, map[string]string{"output": out, "error": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"output": out, "error": ""})
}

func (a *App) handleSSHKeyGen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		KeyType string `json:"key_type"` // "rsa" or "ed25519"
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.KeyType == "" {
		req.KeyType = "rsa"
	}
	keyPair, err := sshclient.GenerateSSHKeyPair(req.KeyType)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, keyPair)
}

func (a *App) handleAuthorizedKeys(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	out, err := a.sshManager.ReadAuthorizedKeys(s)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"keys": out})
}

func (a *App) handleAddAuthorizedKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID  string `json:"server_id"`
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	out, err := a.sshManager.AddAuthorizedKey(s, req.PublicKey)
	if err != nil {
		sendJSON(w, http.StatusOK, map[string]string{"output": out, "error": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"output": out, "error": ""})
}

func (a *App) handleClusterExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerIDs []string `json:"server_ids"`
		Command   string   `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	var targetServers []*models.Server
	for _, id := range req.ServerIDs {
		if srv, ok := a.storage.GetServer(id); ok {
			targetServers = append(targetServers, srv)
		}
	}
	results := a.sshManager.BroadcastCommand(targetServers, req.Command)
	sendJSON(w, http.StatusOK, results)
}

func (a *App) handleExportBackup(w http.ResponseWriter, r *http.Request) {
	data := a.storage.ExportData()
	w.Header().Set("Content-Disposition", `attachment; filename="novassh-backup.json"`)
	sendJSON(w, http.StatusOK, data)
}

func (a *App) handleImportBackup(w http.ResponseWriter, r *http.Request) {
	var backup storage.BackupData
	if err := json.NewDecoder(r.Body).Decode(&backup); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid backup file format")
		return
	}
	if err := a.storage.ImportData(backup); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"status": "imported"})
}

// ---- SFTP Handlers ----

func (a *App) handleSFTPList(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	path := r.URL.Query().Get("path")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	files, err := a.sftpClient.ListFiles(s, path)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, files)
}

func (a *App) handleSFTPUpload(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	dirPath := r.URL.Query().Get("path")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		sendError(w, http.StatusBadRequest, "Error parsing file upload")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "No file selected")
		return
	}
	defer file.Close()

	remotePath := filepath.Join(dirPath, header.Filename)
	if err := a.sftpClient.UploadFile(s, remotePath, file); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"status": "uploaded", "path": remotePath})
}

func (a *App) handleSFTPDownload(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	filePath := r.URL.Query().Get("path")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}

	filename := filepath.Base(filePath)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	if err := a.sftpClient.DownloadFile(s, filePath, w); err != nil {
		log.Printf("[SFTP] Download error: %v", err)
	}
}

func (a *App) handleSFTPMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		Path     string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	if err := a.sftpClient.CreateDir(s, req.Path); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"status": "created"})
}

func (a *App) handleSFTPTouch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		Path     string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	if err := a.sftpClient.CreateFile(s, req.Path); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"status": "created"})
}

func (a *App) handleSFTPDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		Path     string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	if err := a.sftpClient.Delete(s, req.Path); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *App) handleSFTPRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		OldPath  string `json:"old_path"`
		NewPath  string `json:"new_path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	if err := a.sftpClient.Rename(s, req.OldPath, req.NewPath); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"status": "renamed"})
}

func (a *App) handleSFTPRead(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	filePath := r.URL.Query().Get("path")
	s, ok := a.storage.GetServer(id)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	content, err := a.sftpClient.ReadFileContent(s, filePath)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"content": content})
}

func (a *App) handleSFTPWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		Path     string `json:"path"`
		Content  string `json:"content"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s, ok := a.storage.GetServer(req.ServerID)
	if !ok {
		sendError(w, http.StatusNotFound, "Server not found")
		return
	}
	if err := a.sftpClient.WriteFileContent(s, req.Path, req.Content); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (a *App) handleTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("serverId")
	s, ok := a.storage.GetServer(id)
	if !ok {
		return
	}
	a.sshManager.HandleTerminalWebSocket(w, r, s)
}
