package sshclient

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/novassh/novassh/pkg/models"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 1024, // 1MB read buffer for large pastes
	WriteBufferSize: 1024 * 1024, // 1MB write buffer for large terminal outputs
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TerminalMessage struct {
	Type string `json:"type"` // "input", "resize", "ping"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// HandleTerminalWebSocket upgrades HTTP request to WebSocket and bridges xterm.js to an interactive SSH PTY
func (m *Manager) HandleTerminalWebSocket(w http.ResponseWriter, r *http.Request, s *models.Server) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Terminal] WebSocket upgrade error: %v", err)
		return
	}
	defer ws.Close()

	// Set 10MB read limit to safely allow large script/text pastes
	ws.SetReadLimit(10 * 1024 * 1024)

	config, err := BuildClientConfig(s)
	if err != nil {
		msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] Authentication Error: %v\x1b[0m\r\n", err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] Connection Error (%s): %v\x1b[0m\r\n", addr, err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] Session Creation Error: %v\x1b[0m\r\n", err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,      // enable echoing
		ssh.TTY_OP_ISPEED: 115200, // input speed = 115.2kbaud
		ssh.TTY_OP_OSPEED: 115200, // output speed = 115.2kbaud
	}

	// Request pseudo terminal (try xterm-256color first, fallback to xterm)
	err = session.RequestPty("xterm-256color", 24, 80, modes)
	if err != nil {
		err = session.RequestPty("xterm", 24, 80, modes)
		if err != nil {
			msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] PTY Allocation Error: %v\x1b[0m\r\n", err)
			_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
			return
		}
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] Stdin Pipe Error: %v\x1b[0m\r\n", err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] Stdout Pipe Error: %v\x1b[0m\r\n", err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] Stderr Pipe Error: %v\x1b[0m\r\n", err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}

	if err := session.Shell(); err != nil {
		msg := fmt.Sprintf("\r\n\x1b[31;1m[NovaSSH] Shell Execution Error: %v\x1b[0m\r\n", err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
		return
	}

	// Send welcome banner
	welcome := fmt.Sprintf("\r\n\x1b[36;1m====================================================\x1b[0m\r\n"+
		"\x1b[32;1m   NovaSSH Enterprise Terminal — Connected to %s   \x1b[0m\r\n"+
		"\x1b[36;1m====================================================\x1b[0m\r\n\r\n", s.Name)
	_ = ws.WriteMessage(websocket.TextMessage, []byte(welcome))

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Read SSH stdout and write to WebSocket
	go func() {
		defer wg.Done()
		buf := make([]byte, 16384) // 16KB buffer for high-throughput output
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if werr := ws.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("[Terminal] stdout read closed: %v", err)
				}
				break
			}
		}
	}()

	// Goroutine 2: Read SSH stderr and write to WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				_ = ws.WriteMessage(websocket.TextMessage, buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// Goroutine 3: Read WebSocket messages from xterm.js and write to SSH stdin / resize
	go func() {
		defer wg.Done()
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				break
			}
			var tm TerminalMessage
			if err := json.Unmarshal(data, &tm); err == nil {
				switch tm.Type {
				case "input":
					writeStdinChunked(stdin, []byte(tm.Data))
				case "resize":
					if tm.Cols > 0 && tm.Rows > 0 {
						_ = session.WindowChange(tm.Rows, tm.Cols)
					}
				case "ping":
					// ignore heartbeat
				}
			} else {
				// Raw text fallback
				writeStdinChunked(stdin, data)
			}
		}
	}()

	wg.Wait()
	time.Sleep(150 * time.Millisecond) // flush remaining output
	_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33;1m=== SSH Session Terminated ===\x1b[0m\r\n"))
}

// writeStdinChunked writes input data in safe chunks to prevent kernel TTY buffer overflows on large pastes.
func writeStdinChunked(stdin io.Writer, data []byte) {
	chunkSize := 1024
	for len(data) > 0 {
		n := len(data)
		if n > chunkSize {
			n = chunkSize
		}
		_, err := stdin.Write(data[:n])
		if err != nil {
			break
		}
		data = data[n:]
		if len(data) > 0 {
			time.Sleep(1 * time.Millisecond) // micro-delay for kernel TTY backpressure
		}
	}
}
