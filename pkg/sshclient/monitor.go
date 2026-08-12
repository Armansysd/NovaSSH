package sshclient

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/novassh/novassh/pkg/models"
)

// CollectStats queries Linux server metrics via SSH without needing any agent on the server
func (m *Manager) CollectStats(s *models.Server) (*models.SystemStats, error) {
	// A single efficient composite shell command to gather stats in 1 RTT
	cmd := `
echo "---HOSTNAME---" && hostname
echo "---OS---" && (grep -E '^(PRETTY_NAME)=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"' || uname -sr)
echo "---UPTIME---" && uptime -p 2>/dev/null || uptime
echo "---CPU_LOAD---" && cat /proc/loadavg 2>/dev/null || uptime
echo "---MEM---" && free -m | grep -E '^Mem:'
echo "---DISK---" && df -B1M / | tail -1
`
	out, err := m.RunCommand(s, cmd)
	if err != nil {
		return &models.SystemStats{
			ServerID: s.ID,
			Error:    fmt.Sprintf("خطا در دریافت اطلاعات: %v", err),
		}, nil
	}

	stats := &models.SystemStats{
		ServerID: s.ID,
	}

	lines := strings.Split(out, "\n")
	var section string
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "---") && strings.HasSuffix(line, "---") {
			section = strings.Trim(line, "-")
			continue
		}
		if line == "" {
			continue
		}

		switch section {
		case "HOSTNAME":
			if stats.Hostname == "" {
				stats.Hostname = line
			}
		case "OS":
			if stats.OSInfo == "" {
				stats.OSInfo = line
			}
		case "UPTIME":
			if stats.Uptime == "" {
				stats.Uptime = line
			}
		case "CPU_LOAD":
			// parse loadavg e.g. "0.15 0.12 0.08 1/435 12345"
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				stats.LoadAvg = fmt.Sprintf("%s / %s / %s", parts[0], parts[1], parts[2])
				if val, err := strconv.ParseFloat(parts[0], 64); err == nil {
					// Approximate CPU % from 1min load avg
					cp := val * 25.0
					if cp > 100 {
						cp = 99.8
					}
					stats.CPUPercent = math.Round(cp*10) / 10
				}
			}
		case "MEM":
			// Mem: 7950 3410 ...
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				tot, _ := strconv.ParseFloat(fields[1], 64)
				used, _ := strconv.ParseFloat(fields[2], 64)
				stats.RAMTotalMB = tot
				stats.RAMUsedMB = used
				if tot > 0 {
					stats.RAMPercent = math.Round((used/tot)*1000) / 10
				}
			}
		case "DISK":
			// /dev/sda1 50000 20000 ...
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				totMB, _ := strconv.ParseFloat(strings.TrimSuffix(fields[1], "M"), 64)
				usedMB, _ := strconv.ParseFloat(strings.TrimSuffix(fields[2], "M"), 64)
				stats.DiskTotalGB = math.Round((totMB/1024)*10) / 10
				stats.DiskUsedGB = math.Round((usedMB/1024)*10) / 10
				if totMB > 0 {
					stats.DiskPercent = math.Round((usedMB/totMB)*1000) / 10
				}
			}
		}
	}

	if stats.Hostname == "" {
		stats.Hostname = s.Host
	}
	if stats.OSInfo == "" {
		stats.OSInfo = "Linux Server"
	}
	return stats, nil
}
