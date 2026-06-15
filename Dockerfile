# Stage 1: build frontend
FROM node:24-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ .
RUN npm run build

# Stage 2: build Go backend
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ssh-backend .

# Stage 3: minimal runtime
FROM alpine:3.21
RUN apk add --no-cache openssh-client sshpass
COPY --from=backend  /app/ssh-backend /app/ssh-backend
COPY --from=frontend /app/dist        /app/dist
ENV STATIC_DIR=/app/dist PORT=8080
EXPOSE 8080
CMD ["/app/ssh-backend"]
