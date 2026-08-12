package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/novassh/novassh/pkg/models"
)

type Manager struct {
	mu           sync.RWMutex
	serversPath  string
	snippetsPath string
	servers      map[string]*models.Server
	snippets     map[string]*models.Snippet
}

func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	m := &Manager{
		serversPath:  filepath.Join(dataDir, "servers.json"),
		snippetsPath: filepath.Join(dataDir, "snippets.json"),
		servers:      make(map[string]*models.Server),
		snippets:     make(map[string]*models.Snippet),
	}

	m.load()
	m.seedDefaults()
	m.save()
	return m, nil
}

func (m *Manager) seedDefaults() {
	// Seed useful default snippets if empty
	if len(m.snippets) == 0 {
		defaultSnippets := []models.Snippet{
			{
				ID:          "sn-sys-health",
				Title:       "📊 System Resource Quick View",
				Command:     "echo '=== OS ===' && uname -srm && echo '=== MEMORY ===' && free -h && echo '=== DISK ===' && df -h /",
				Tags:        []string{"System", "Monitoring"},
				Description: "Quick view of OS distribution, RAM usage and disk storage space",
			},
			{
				ID:          "sn-docker-ps",
				Title:       "🐳 Docker Active Containers",
				Command:     "docker ps --format 'table {{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Ports}}'",
				Tags:        []string{"Docker", "DevOps"},
				Description: "List all active Docker containers in table format",
			},
			{
				ID:          "sn-ports",
				Title:       "🌐 Listening Network Ports",
				Command:     "sudo ss -tuln || sudo netstat -tuln",
				Tags:        []string{"Network", "Security"},
				Description: "Display all open listening network ports on the server",
			},
			{
				ID:          "sn-top-cpu",
				Title:       "⚡ Top 5 CPU Consuming Processes",
				Command:     "ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%cpu | head -n 6",
				Tags:        []string{"Performance", "CPU"},
				Description: "View the top 5 CPU consuming processes on the server",
			},
			{
				ID:          "sn-top-mem",
				Title:       "💾 Top 5 RAM Consuming Processes",
				Command:     "ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%mem | head -n 6",
				Tags:        []string{"Performance", "RAM"},
				Description: "View the top 5 Memory consuming processes on the server",
			},
			{
				ID:          "sn-ufw-status",
				Title:       "🛡️ Firewall Rules (UFW / iptables)",
				Command:     "sudo ufw status verbose || sudo iptables -L -n -v | head -n 25",
				Tags:        []string{"Security", "Firewall"},
				Description: "Check active Linux firewall rules and security policies",
			},
		}
		for _, s := range defaultSnippets {
			sn := s
			m.snippets[s.ID] = &sn
		}
	}
}

func (m *Manager) load() {
	if data, err := os.ReadFile(m.serversPath); err == nil {
		var list []*models.Server
		if json.Unmarshal(data, &list) == nil {
			for _, s := range list {
				m.servers[s.ID] = s
			}
		}
	}

	if data, err := os.ReadFile(m.snippetsPath); err == nil {
		var list []*models.Snippet
		if json.Unmarshal(data, &list) == nil {
			for _, s := range list {
				m.snippets[s.ID] = s
			}
		}
	}
}

func (m *Manager) save() error {
	var srvList []*models.Server
	for _, s := range m.servers {
		srvList = append(srvList, s)
	}
	data, err := json.MarshalIndent(srvList, "", "  ")
	if err == nil {
		_ = os.WriteFile(m.serversPath, data, 0644)
	}

	var snList []*models.Snippet
	for _, s := range m.snippets {
		snList = append(snList, s)
	}
	dataSn, err := json.MarshalIndent(snList, "", "  ")
	if err == nil {
		_ = os.WriteFile(m.snippetsPath, dataSn, 0644)
	}
	return nil
}

// Server CRUD
func (m *Manager) GetServers() []*models.Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*models.Server
	for _, s := range m.servers {
		list = append(list, s)
	}
	return list
}

func (m *Manager) GetServer(id string) (*models.Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[id]
	return s, ok
}

func (m *Manager) SaveServer(s *models.Server) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.ID == "" {
		s.ID = "srv-" + time.Now().Format("20060102-150405")
	}
	m.servers[s.ID] = s
	return m.save()
}

func (m *Manager) DeleteServer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.servers, id)
	return m.save()
}

func (m *Manager) UpdateServerStatus(id string, status string, latency int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.servers[id]; ok {
		s.Status = status
		s.LatencyMS = latency
		_ = m.save()
	}
}

// Snippet CRUD
func (m *Manager) GetSnippets() []*models.Snippet {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*models.Snippet
	for _, s := range m.snippets {
		list = append(list, s)
	}
	return list
}

func (m *Manager) SaveSnippet(sn *models.Snippet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sn.ID == "" {
		sn.ID = "sn-" + time.Now().Format("20060102-150405")
	}
	m.snippets[sn.ID] = sn
	return m.save()
}

func (m *Manager) DeleteSnippet(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.snippets, id)
	return m.save()
}

// Export / Import Backup JSON
type BackupData struct {
	Servers  []*models.Server  `json:"servers"`
	Snippets []*models.Snippet `json:"snippets"`
}

func (m *Manager) ExportData() BackupData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var srvList []*models.Server
	for _, s := range m.servers {
		srvList = append(srvList, s)
	}
	var snList []*models.Snippet
	for _, sn := range m.snippets {
		snList = append(snList, sn)
	}
	return BackupData{
		Servers:  srvList,
		Snippets: snList,
	}
}

func (m *Manager) ImportData(b BackupData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range b.Servers {
		m.servers[s.ID] = s
	}
	for _, sn := range b.Snippets {
		m.snippets[sn.ID] = sn
	}
	return m.save()
}
