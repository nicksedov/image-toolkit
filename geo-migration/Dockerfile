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

# Install runtime dependencies
RUN apk add --no-cache sed proxychains-ng

# Configure remote proxy for proxychains4
RUN sed -i 's/^\s*socks4\s\+\S\+\s\+[0-9]\+\s*$/socks5 167.17.182.105 9080 proxy WOg54ih_Gsq2/' /etc/proxychains/proxychains.conf

RUN proxychains4 apk add --no-cache perl exiftool

COPY --from=builder /build/geo-migration /usr/local/bin/geo-migration

WORKDIR /app

# The app expects .env at this relative path
COPY .env .

ENTRYPOINT ["geo-migration"]
