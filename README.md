# ssh-term

Browser SSH client. Go backend bridges WebSocket → `ssh` subprocess with a local PTY. Frontend is vanilla TypeScript + [`@wterm/dom`](https://github.com/anomalyco/wterm) for terminal rendering.

## How it works

```
Browser ──WS──► Go server ──pty──► ssh ──► remote host
```

1. Browser opens a WebSocket to `/ws`
2. First message is a JSON object with connection params
3. Go spawns `ssh -tt` (or `sshpass … ssh`) in a local PTY
4. All output is forwarded as binary WebSocket frames
5. Keystrokes and `\x1b[RESIZE:W;H]` control frames come back the other way

## Development

```sh
# Frontend (from frontend/)
npm install
npm run dev       # runs Vite dev server

# Backend (from backend/)
STATIC_DIR=../frontend/dist PORT=8080 go run .
```

## k8s / Docker

```sh
docker build -t ghcr.io/whexy/ssh-term:latest .
docker push ghcr.io/whexy/ssh-term:latest
kubectl apply -f k8s/
```

The image contains `openssh-client` and `sshpass`. No credentials are stored — they come from the browser on each connection.
