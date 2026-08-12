package sshclient

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/novassh/novassh/pkg/models"
	"golang.org/x/crypto/ssh"
)

// 1. Linux Systemd Service Manager
func (m *Manager) ListServices(s *models.Server) ([]models.ServiceInfo, error) {
	cmd := `systemctl list-units --type=service --all --no-pager --no-legend | head -n 25`
	out, err := m.RunCommand(s, cmd)
	if err != nil {
		return nil, err
	}
	var services []models.ServiceInfo
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			name := fields[0]
			load := fields[1]
			active := fields[2]
			sub := fields[3]
			desc := strings.Join(fields[4:], " ")
			services = append(services, models.ServiceInfo{
				Name:        name,
				Load:        load,
				Active:      active,
				Sub:         sub,
				Description: desc,
			})
		}
	}
	return services, nil
}

func (m *Manager) ServiceAction(s *models.Server, action string, serviceName string) (string, error) {
	allowed := map[string]bool{"start": true, "stop": true, "restart": true, "status": true, "enable": true, "disable": true}
	if !allowed[action] {
		return "", fmt.Errorf("invalid service action: %s", action)
	}
	cmd := fmt.Sprintf("sudo systemctl %s %s --no-pager", action, serviceName)
	return m.RunCommand(s, cmd)
}

// 2. Docker Active Containers & DevOps Explorer
func (m *Manager) ListContainers(s *models.Server) ([]models.DockerContainer, error) {
	cmd := `docker ps -a --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}\t{{.State}}' | head -n 25`
	out, err := m.RunCommand(s, cmd)
	if err != nil {
		return nil, err
	}
	var containers []models.DockerContainer
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) >= 6 {
			containers = append(containers, models.DockerContainer{
				ID:     fields[0],
				Names:  fields[1],
				Image:  fields[2],
				Status: fields[3],
				Ports:  fields[4],
				State:  fields[5],
			})
		}
	}
	return containers, nil
}

func (m *Manager) ContainerAction(s *models.Server, action string, containerID string) (string, error) {
	allowed := map[string]bool{"start": true, "stop": true, "restart": true, "logs": true}
	if !allowed[action] {
		return "", fmt.Errorf("invalid container action: %s", action)
	}
	var cmd string
	if action == "logs" {
		cmd = fmt.Sprintf("docker logs --tail 40 %s", containerID)
	} else {
		cmd = fmt.Sprintf("docker %s %s", action, containerID)
	}
	return m.RunCommand(s, cmd)
}

// 3. Listening Network Ports & Security Scanner
func (m *Manager) ListListeningPorts(s *models.Server) ([]models.PortInfo, error) {
	cmd := `ss -tulnp || netstat -tulnp`
	out, err := m.RunCommand(s, cmd)
	if err != nil {
		return nil, err
	}
	var ports []models.PortInfo
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if i == 0 || strings.Contains(line, "State") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			ports = append(ports, models.PortInfo{
				Proto:       fields[0],
				State:       fields[1],
				LocalAddr:   fields[4],
				ForeignAddr: "-",
				Process:     "-",
			})
		}
	}
	return ports, nil
}

// 4. Remote Log Streamer (tail -n)
func (m *Manager) TailLogFile(s *models.Server, filePath string, lines int) (string, error) {
	if lines <= 0 || lines > 500 {
		lines = 50
	}
	cmd := fmt.Sprintf("tail -n %d %s 2>&1", lines, filePath)
	return m.RunCommand(s, cmd)
}

// 5. Remote Process Killer
func (m *Manager) KillProcess(s *models.Server, pid string) (string, error) {
	cmd := fmt.Sprintf("sudo kill -9 %s 2>&1", pid)
	return m.RunCommand(s, cmd)
}

// 6. SSH Authorized_Keys Manager
func (m *Manager) ReadAuthorizedKeys(s *models.Server) (string, error) {
	cmd := `cat ~/.ssh/authorized_keys 2>/dev/null || cat /root/.ssh/authorized_keys 2>/dev/null || echo "No authorized_keys found."`
	return m.RunCommand(s, cmd)
}

func (m *Manager) AddAuthorizedKey(s *models.Server, pubKey string) (string, error) {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return "", fmt.Errorf("public key cannot be empty")
	}
	cmd := fmt.Sprintf(`mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo "%s" >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && echo "Public key added successfully."`, pubKey)
	return m.RunCommand(s, cmd)
}

// 7. Pure Go SSH Key Generator (RSA 4096 / Ed25519)
func GenerateSSHKeyPair(keyType string) (*models.SSHKeyPair, error) {
	if keyType == "ed25519" {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		privKeyPEM, err := ssh.MarshalPrivateKey(priv, "novassh-key")
		if err != nil {
			return nil, err
		}
		sshPub, err := ssh.NewPublicKey(pub)
		if err != nil {
			return nil, err
		}
		return &models.SSHKeyPair{
			KeyType:    "ed25519",
			PrivateKey: string(pem.EncodeToMemory(privKeyPEM)),
			PublicKey:  string(ssh.MarshalAuthorizedKey(sshPub)),
		}, nil
	}

	// Default RSA 4096
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privBlock := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	}
	sshPub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	return &models.SSHKeyPair{
		KeyType:    "rsa",
		PrivateKey: string(pem.EncodeToMemory(&privBlock)),
		PublicKey:  string(ssh.MarshalAuthorizedKey(sshPub)),
	}, nil
}

// 8. Multi-Server Cluster Command Broadcast
func (m *Manager) BroadcastCommand(servers []*models.Server, command string) []models.ClusterExecResult {
	results := make([]models.ClusterExecResult, len(servers))
	var wg sync.WaitGroup
	for i, s := range servers {
		wg.Add(1)
		go func(idx int, srv *models.Server) {
			defer wg.Done()
			start := time.Now()
			out, err := m.RunCommand(srv, command)
			dur := time.Since(start).Milliseconds()
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			results[idx] = models.ClusterExecResult{
				ServerID:   srv.ID,
				ServerName: srv.Name,
				Output:     out,
				Error:      errStr,
				DurationMS: dur,
			}
		}(i, s)
	}
	wg.Wait()
	return results
}
