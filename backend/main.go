package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

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

	// Build ssh args. -tt forces remote PTY allocation even without a local tty.
	args := []string{
		"-tt",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", strconv.Itoa(params.Port),
	}

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
		cmd = exec.Command("sshpass", append([]string{"-p", params.Password, "ssh"}, args...)...)
	} else {
		cmd = exec.Command("ssh", args...)
	}

	// Use plain pipes — no local PTY, so no local echo or line-discipline interference.
	// The remote PTY (allocated by -tt) handles all terminal processing.
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+err.Error()+`"}`))
		return
	}

	// Merge stdout+stderr into a single pipe so the browser sees everything.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+err.Error()+`"}`))
		return
	}
	defer func() {
		cmd.Process.Kill()
		stdinPipe.Close()
		pw.Close()
	}()

	// Close write end of pipe when ssh exits so the reader goroutine unblocks.
	go func() {
		cmd.Wait()
		pw.Close()
	}()

	// ssh output → WebSocket
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
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	// WebSocket → ssh stdin
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s := string(msg)
		if strings.HasPrefix(s, "\x1b[RESIZE:") {
			// Resize not yet supported in pipe mode — remote PTY stays at initial size.
			// TODO: send SSH channel window-change request.
			continue
		}
		if _, err := stdinPipe.Write(msg); err != nil {
			break
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
