# Stage 1: build the web UI
FROM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: build the Go binary
FROM golang:1.26.4-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /src/web/dist ./web/dist
ARG VERSION=dev
ARG COMMIT_SHA=none
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/0funct0ry/squad/cmd.Version=${VERSION} -X github.com/0funct0ry/squad/cmd.CommitSHA=${COMMIT_SHA}" \
    -o /out/squad main.go

# Stage 3: runtime
FROM alpine:latest AS runtime
RUN apk add --no-cache ca-certificates
COPY --from=go-builder /out/squad /usr/local/bin/squad
ENTRYPOINT ["squad"]
