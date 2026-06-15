package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ConnectParams struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// First message: JSON connection params
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var params ConnectParams
	if err := json.Unmarshal(msg, &params); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid params"}`))
		return
	}
	if params.Port == 0 {
		params.Port = 22
	}

	// Build auth methods
	var authMethods []ssh.AuthMethod
	if params.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(params.PrivateKey))
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid private key: `+err.Error()+`"}`))
			return
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if params.Password != "" {
		authMethods = append(authMethods, ssh.Password(params.Password))
	}
	if len(authMethods) == 0 {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"no auth method provided"}`))
		return
	}

	// Dial SSH
	config := &ssh.ClientConfig{
		User:            params.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
	}
	addr := net.JoinHostPort(params.Host, strconv.Itoa(params.Port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+err.Error()+`"}`))
		return
	}
	defer client.Close()

	// Open session
	session, err := client.NewSession()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"session: `+err.Error()+`"}`))
		return
	}
	defer session.Close()

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"pty: `+err.Error()+`"}`))
		return
	}

	// Wire up stdin and merged stdout+stderr
	stdin, err := session.StdinPipe()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"stdin: `+err.Error()+`"}`))
		return
	}

	pr, pw := io.Pipe()
	session.Stdout = pw
	session.Stderr = pw

	if err := session.Shell(); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"shell: `+err.Error()+`"}`))
		return
	}

	// Close pipe write end when the session exits so the reader unblocks
	go func() {
		session.Wait()
		pw.Close()
	}()

	// SSH → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	// WebSocket → SSH (data + resize)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s := string(msg)
		if strings.HasPrefix(s, "\x1b[RESIZE:") {
			inner := strings.TrimSuffix(s[len("\x1b[RESIZE:"):], "]")
			parts := strings.SplitN(inner, ";", 2)
			if len(parts) == 2 {
				cols, _ := strconv.Atoi(parts[0])
				rows, _ := strconv.Atoi(parts[1])
				if cols > 0 && rows > 0 {
					session.WindowChange(rows, cols)
				}
			}
		} else {
			if _, err := stdin.Write(msg); err != nil {
				break
			}
		}
	}
}

// cacheHandler sets Cache-Control headers:
//   - HTML → no-cache (always revalidate; picks up new hashed asset URLs)
//   - Everything else → immutable 1-year (content-hashed by Vite, safe to cache forever)
func cacheHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".html") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./dist"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.Handle("/", cacheHandler(http.FileServer(http.Dir(staticDir))))
	http.HandleFunc("/ws", wsHandler)

	log.Printf("listening :%s  static=%s", port, staticDir)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
