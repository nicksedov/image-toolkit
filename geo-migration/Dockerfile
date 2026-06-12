# ---- Stage 1: Build the Go binary (Linux) ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o geo-migration .

# ---- Stage 2: Runtime image with exiftool ----
FROM alpine:3.21

RUN apk add --no-cache perl exiftool

COPY --from=builder /build/geo-migration /usr/local/bin/geo-migration

# The app expects backend/.env at this relative path
WORKDIR /app

ENTRYPOINT ["geo-migration"]
