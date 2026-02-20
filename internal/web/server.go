package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"
)

//go:embed static
var staticFS embed.FS

// Config holds web terminal server configuration.
type Config struct {
	Addr    string // HTTP listen address, e.g. ":8080"
	SSHAddr string // local SSH server address for bridging, e.g. "127.0.0.1:2222"
	Logger  *slog.Logger
}

// Server serves the xterm.js web terminal and bridges WebSocket connections
// to the local SSH server.
type Server struct {
	httpSrv *http.Server
	cfg     Config
}

// New creates a new web terminal server.
func New(cfg Config) *Server {
	sub, _ := fs.Sub(staticFS, "static")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/ws", handleWebSocket(cfg))

	return &Server{
		httpSrv: &http.Server{Addr: cfg.Addr, Handler: mux},
		cfg:     cfg,
	}
}

// Start begins serving HTTP on the configured address.
func (s *Server) Start() error {
	s.cfg.Logger.Info("web terminal listening", "addr", s.cfg.Addr)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.cfg.Logger.Error("web server stopped", "error", err)
		}
	}()
	return nil
}

// Serve begins serving HTTP on the given listener (e.g. from tsnet).
func (s *Server) Serve(ln net.Listener) error {
	s.cfg.Logger.Info("web terminal listening", "addr", ln.Addr().String())
	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.cfg.Logger.Error("web server stopped", "error", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the web server.
func (s *Server) Stop() error {
	return s.httpSrv.Shutdown(context.Background())
}

var upgrader = websocket.Upgrader{
	CheckOrigin: websocketCheckOrigin,
}

type resizeMsg struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// handleWebSocket bridges a WebSocket connection to the local SSH server.
// Each connection gets its own SSH session with a PTY, providing a full TUI.
//
// Protocol:
//   - Client text messages  -> terminal input (stdin)
//   - Client binary messages -> JSON control (resize: {"cols":N,"rows":N})
//   - Server binary messages -> terminal output (stdout)
func handleWebSocket(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			cfg.Logger.Error("websocket upgrade failed", "error", err)
			return
		}
		defer ws.Close()

		// Connect to local SSH server.
		sshConfig := &gossh.ClientConfig{
			User:            "web",
			Auth:            []gossh.AuthMethod{gossh.Password("")},
			HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		}

		sshClient, err := gossh.Dial("tcp", cfg.SSHAddr, sshConfig)
		if err != nil {
			cfg.Logger.Error("ssh dial failed", "error", err, "addr", cfg.SSHAddr)
			return
		}
		defer sshClient.Close()

		session, err := sshClient.NewSession()
		if err != nil {
			cfg.Logger.Error("ssh session failed", "error", err)
			return
		}
		defer session.Close()

		// Read initial resize from client to get correct terminal dimensions.
		cols, rows := 80, 24
		ws.SetReadDeadline(deadlineForInitialResize())
		msgType, msg, err := ws.ReadMessage()
		ws.SetReadDeadline(noDeadline())
		if err == nil && msgType == websocket.BinaryMessage {
			var sz resizeMsg
			if json.Unmarshal(msg, &sz) == nil && sz.Cols > 0 && sz.Rows > 0 {
				cols, rows = sz.Cols, sz.Rows
			}
		}

		// Request PTY with the client's terminal dimensions.
		if err := session.RequestPty("xterm-256color", rows, cols, gossh.TerminalModes{
			gossh.ECHO: 1,
		}); err != nil {
			cfg.Logger.Error("ssh pty request failed", "error", err)
			return
		}

		stdin, err := session.StdinPipe()
		if err != nil {
			cfg.Logger.Error("ssh stdin pipe failed", "error", err)
			return
		}

		stdout, err := session.StdoutPipe()
		if err != nil {
			cfg.Logger.Error("ssh stdout pipe failed", "error", err)
			return
		}

		if err := session.Shell(); err != nil {
			cfg.Logger.Error("ssh shell failed", "error", err)
			return
		}

		// Bridge: SSH stdout -> WebSocket.
		done := make(chan struct{})
		go func() {
			defer close(done)
			buf := make([]byte, 32*1024)
			for {
				n, err := stdout.Read(buf)
				if n > 0 {
					if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()

		// Bridge: WebSocket -> SSH stdin (text=input, binary=resize).
		for {
			msgType, msg, err := ws.ReadMessage()
			if err != nil {
				break
			}
			if len(msg) == 0 {
				continue
			}

			switch msgType {
			case websocket.TextMessage:
				// Terminal input.
				if _, err := stdin.Write(msg); err != nil {
					goto cleanup
				}
			case websocket.BinaryMessage:
				// Control message (resize).
				var resize resizeMsg
				if json.Unmarshal(msg, &resize) == nil && resize.Cols > 0 && resize.Rows > 0 {
					_ = session.WindowChange(resize.Rows, resize.Cols)
				}
			}
		}

	cleanup:
		stdin.Close()
		session.Close()
		<-done
	}
}
