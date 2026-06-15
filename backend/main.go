package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
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

	// Build ssh args
	args := []string{
		"-tt",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", strconv.Itoa(params.Port),
	}

	// Key auth: write to temp file
	if params.PrivateKey != "" {
		f, err := os.CreateTemp("", "ssh-key-*")
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"key file error"}`))
			return
		}
		defer os.Remove(f.Name())
		f.WriteString(params.PrivateKey)
		f.Close()
		os.Chmod(f.Name(), 0600)
		args = append(args, "-i", f.Name())
	}

	args = append(args, params.Username+"@"+params.Host)

	var cmd *exec.Cmd
	if params.Password != "" && params.PrivateKey == "" {
		// sshpass for password auth
		cmd = exec.Command("sshpass", append([]string{"-p", params.Password, "ssh"}, args...)...)
	} else {
		cmd = exec.Command("ssh", args...)
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+err.Error()+`"}`))
		return
	}
	defer func() {
		cmd.Process.Kill()
		ptmx.Close()
	}()

	pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	// ssh → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// WebSocket → ssh (handle resize frames)
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
					pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
				}
			}
		} else {
			ptmx.Write(msg)
		}
	}
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

	http.Handle("/", http.FileServer(http.Dir(staticDir)))
	http.HandleFunc("/ws", wsHandler)

	log.Printf("listening :%s  static=%s", port, staticDir)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
