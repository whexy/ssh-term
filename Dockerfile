# Stage 1: build frontend
FROM node:24-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ .
# Apply patched WASM — fixes vercel-labs/wterm#86 (stale SGR from alt-screen
# leaks into primary screen after DECRST 1049, i.e. ghost vim background).
# npm install runs before this COPY so we patch the installed binary in-place.
# Rebuild the WASM: clone vercel-labs/wterm, fix wasm_api.zig per issue #86,
# run scripts/build-wasm.sh (requires zig_0_15), copy output here.
RUN cp patches/ghostty-vt.wasm node_modules/@wterm/ghostty/wasm/ghostty-vt.wasm
RUN npm run build

# Stage 2: build Go backend
FROM golang:alpine AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ssh-backend .

# Stage 3: minimal runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=backend  /app/ssh-backend /app/ssh-backend
COPY --from=frontend /app/dist        /app/dist
ENV STATIC_DIR=/app/dist PORT=8080
EXPOSE 8080
CMD ["/app/ssh-backend"]
