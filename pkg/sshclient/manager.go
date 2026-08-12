package sshclient

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/novassh/novassh/pkg/models"
	"golang.org/x/crypto/ssh"
)

type Manager struct {
	mu      sync.RWMutex
	clients map[string]*ssh.Client
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*ssh.Client),
	}
}

// BuildClientConfig constructs the ssh.ClientConfig from Server model
func BuildClientConfig(s *models.Server) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if s.AuthType == "key" || s.PrivateKey != "" {
		var signer ssh.Signer
		var err error

		// Check if private key is a file path or raw string
		keyData := []byte(s.PrivateKey)
		if _, statErr := os.Stat(s.PrivateKey); statErr == nil {
			keyData, err = os.ReadFile(s.PrivateKey)
			if err != nil {
				return nil, fmt.Errorf("خطا در خواندن فایل کلید خصوصی: %v", err)
			}
		}

		if s.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(s.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("کلید خصوصی معتبر نیست: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if s.AuthType == "password" || len(authMethods) == 0 {
		authMethods = append(authMethods, ssh.Password(s.Password))
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
			for range questions {
				answers = append(answers, s.Password)
			}
			return answers, nil
		}))
	}

	config := &ssh.ClientConfig{
		User: s.Username,
		Auth: authMethods,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Auto-accept host key for seamless user-friendly experience
			return nil
		},
		Timeout: 6 * time.Second,
	}
	return config, nil
}

// Connect establishes a cached SSH client connection
func (m *Manager) Connect(s *models.Server) (*ssh.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already connected and healthy
	if client, ok := m.clients[s.ID]; ok && client != nil {
		_, _, err := client.SendRequest("keepalive@novassh", true, nil)
		if err == nil {
			return client, nil
		}
		client.Close()
		delete(m.clients, s.ID)
	}

	config, err := BuildClientConfig(s)
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("خطای اتصال به سرور (%s): %v", addr, err)
	}

	m.clients[s.ID] = client
	return client, nil
}

// GetClient retrieves an existing active SSH connection or returns nil
func (m *Manager) GetClient(serverID string) *ssh.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[serverID]
}

// CloseConnection terminates a cached SSH client
func (m *Manager) CloseConnection(serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[serverID]; ok && c != nil {
		_ = c.Close()
		delete(m.clients, serverID)
	}
}

// RunCommand executes a single command on the server and returns output
func (m *Manager) RunCommand(s *models.Server, cmd string) (string, error) {
	client, err := m.Connect(s)
	if err != nil {
		return "", err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

// PingServer checks server latency concurrently
func (m *Manager) PingServer(s *models.Server) models.PingResult {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return models.PingResult{
			ID:     s.ID,
			Online: false,
			Error:  err.Error(),
		}
	}
	conn.Close()
	latency := time.Since(start).Milliseconds()
	if latency == 0 {
		latency = 1
	}
	return models.PingResult{
		ID:        s.ID,
		Online:    true,
		LatencyMS: latency,
	}
}
