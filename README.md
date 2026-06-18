# Flashbacks

A web application for finding and managing duplicate images in local media libraries.

## Features

- Scan one or more directories for duplicate images
- Detect duplicates by file size and MD5 checksum matching
- Web interface with image thumbnails (up to 192px)
- Direct file deletion or move to trash
- Generate bash/PowerShell scripts for file relocation
- Batch deduplication by folder patterns
- Asynchronous scanning with progress display
- Metadata caching in PostgreSQL for faster rescans
- OCR classification for document detection
- Geolocation extraction and clustering on map
- AI-powered image tagging
- User authentication with session management
- Multi-language support (English, Russian)
- Dark/light theme

## Supported Formats

JPG, JPEG, PNG, GIF, BMP, TIFF, TIF, WEBP

## Requirements

- Go 1.25 or higher
- Node.js 18 or higher (for frontend)
- PostgreSQL 12 or higher
- exiftool (for EXIF metadata extraction)



## Configuration

### Backend (`backend/.env`)

| Variable       | Description                              | Default                  |
|----------------|------------------------------------------|--------------------------|
| `DB_HOST`      | PostgreSQL host                          | `localhost`              |
| `DB_PORT`      | PostgreSQL port                          | `5432`                   |
| `DB_USER`      | PostgreSQL user                          | `postgres`               |
| `DB_PASSWORD`  | PostgreSQL password                      | `postgres`               |
| `DB_NAME`      | Database name                            | `image_toolkit`          |
| `SERVER_HOST`  | API server bind address                  | `0.0.0.0`                |
| `SERVER_PORT`  | API server port                          | `5170`                   |
| `CORS_ORIGINS` | Allowed origins (comma-separated), or `*` | `http://localhost:5173` |

### Frontend (`frontend/.env`)

| Variable       | Description                          | Default |
|----------------|--------------------------------------|---------|
| `VITE_API_URL` | Backend API URL                      | (empty -- uses Vite proxy) |

In development mode, Vite proxies `/api/*` requests to the backend (`http://localhost:5170`).
For production, specify the full backend URL in `VITE_API_URL`.

## Build & Run

### 1. Create PostgreSQL Database

```sql
CREATE DATABASE image_toolkit;
```

### 2. Setup Environment

```bash
# Backend
cp backend/.env.example backend/.env
# Edit backend/.env -- specify database connection parameters

# Frontend
cp frontend/.env.example frontend/.env
# For development, VITE_API_URL can be left empty (uses proxy)
```

### 3. Build Backend 

#### For server deployment without Docker
```bash
cd backend
go mod tidy
go build -o image-toolkit.exe ./cmd/server/    # Windows
go build -o image-toolkit ./cmd/server/        # Linux/macOS
```

#### For Docker container deployment
docker build -t localhost:5000/image-tool:<X.Y> .
docker push localhost:5000/image-tool:<X.Y>

### 4. Build Frontend

```bash
cd frontend
npm install
npm run build    # Builds to frontend/dist/
```

### 5. Run (Development Mode)

**Terminal 1 -- Backend:**

```bash
cd backend
go run .
```

Backend will start on `http://0.0.0.0:5170` by default.

**Terminal 2 -- Frontend:**

```bash
cd frontend
npm run dev
```

Open in browser: `http://localhost:5173`

### 6. Run (Production)

```bash
# Backend
cd backend
./image-toolkit      # Windows: image-toolkit.exe

# Frontend -- static files from frontend/dist/
# Serve via nginx, Caddy, or any other web server
```

## Remote Access (Local Network Testing)

Both servers (backend and frontend) listen on `0.0.0.0` by default, making them accessible from any machine on the local network.

### 1. Find Server IP Address

On the machine where servers are running:

```bash
# Windows
ipconfig

# Linux / macOS
ip addr        # или: hostname -I
```

Remember the IPv4 address (e.g., `192.168.1.100`).

### 2. Configure Backend

In `backend/.env`, ensure these are set:

```env
SERVER_HOST=0.0.0.0
CORS_ORIGINS=*
```

- `SERVER_HOST=0.0.0.0` -- backend listens on all network interfaces
- `CORS_ORIGINS=*` -- allows requests from any origin (suitable for development in trusted network)

For stricter configuration, list specific origins:

```env
CORS_ORIGINS=http://192.168.1.100:5173,http://localhost:5173
```

### 3. Configure Frontend

In `frontend/.env`, leave `VITE_API_URL` empty:

```env
VITE_API_URL=
```

This enables proxying through Vite -- all `/api/*` requests will be forwarded to the backend on the same machine. The Vite dev server already listens on `0.0.0.0` (`host: true` in `vite.config.ts`).

### 4. Start Both Servers

```bash
# Терминал 1 -- бэкенд
cd backend
go run .

# Терминал 2 -- фронтенд
cd frontend
npm run dev
```

### 5. Access from Remote Machine

On the remote machine, open in browser:

```
http://192.168.1.100:5173
```

Replace `192.168.1.100` with the actual server IP address.

### 6. Firewall

If the remote machine cannot connect, check that the firewall on the server machine allows incoming connections on ports **5170** (backend) and **5173** (frontend).

Windows (PowerShell от администратора):

```powershell
netsh advfirewall firewall add rule name="Flashbacks Backend" dir=in action=allow protocol=TCP localport=5170
netsh advfirewall firewall add rule name="Flashbacks Frontend" dir=in action=allow protocol=TCP localport=5173
```

Linux:

```bash
sudo ufw allow 5170/tcp
sudo ufw allow 5173/tcp
```

## API Reference

All routes are prefixed with `/api/`. Responses are in JSON format.

| Method | Route                | Description                             |
|--------|----------------------|-----------------------------------------|
| GET    | `/api/duplicates`    | Duplicate groups with pagination        |
| POST   | `/api/scan`          | Start asynchronous scan                 |
| GET    | `/api/status`        | Current scan status                     |
| GET    | `/api/thumbnail`     | Thumbnail for file                      |
| POST   | `/api/generate-script`| Generate deletion script              |
| POST   | `/api/delete-files`  | Direct file deletion                    |
| GET    | `/api/folder-patterns`| Folder patterns for batch deduplication|
| POST   | `/api/batch-delete`  | Batch deletion by rules                 |

## Лицензия

MIT
