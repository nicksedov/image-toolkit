# Image Toolkit - Agent Context

## Overview
Full-stack app for finding/managing duplicate images. Async scanning, MD5 detection, thumbnails, OCR, geolocation clustering, batch deduplication.

## Stack
**Backend**: Go 1.25+, Gin, PostgreSQL 12+ (GORM), exiftool, gocluster (geo clustering), imaging lib
**Frontend**: TypeScript 6+, React 19+, Vite 8+, Tailwind 4+, shadcn/ui (Radix), react-leaflet, lucide-react, custom i18n (en/ru)

## Structure
```
backend/
├── cmd/server/main.go          # Entry point, DI
└── internal/
    ├── application/            # Business logic (auth, geo, imaging, thumbnail)
    ├── domain/                 # Models (auth.go, media.go)
    ├── infrastructure/         # External (config, database, geocoder, llm, ocr)
    └── interfaces/             # API layer (dto, handler, i18n, middleware)

frontend/src/
├── api/                        # HTTP client, endpoints
├── components/                 # Feature folders (auth, gallery, duplicates, settings, ui)
├── hooks/                      # Custom hooks (21 files)
├── i18n/                       # Translations (en/ru)
├── providers/                  # Context (auth, settings)
├── theme/                      # Dark/light theme
└── types/                      # TypeScript types
```

## Architecture
- **Backend**: Clean architecture, manual DI, async goroutines, GORM auto-migration
- **Frontend**: Feature-based components, custom hooks (polling, infinite scroll), context providers, typed API client

## Coding Standards

### Go
- PascalCase exported, camelCase unexported
- Explicit error handling, no panics
- **No identifier redeclaration** in same scope
- i18n: Use `Msg*` constants, convert: `string(i18n.MsgX)`
- Validation: `i18n.CreateValidationError(i18n.ValidationError)`
- JSON tags must match frontend TypeScript names (camelCase)

### TypeScript
- Strict mode, `verbatimModuleSyntax`, no unused vars
- `import type` for type-only imports
- Strict `TranslationKey` type - no arbitrary strings in `t()`
- Path alias: `@/*` → `src/*`
- Field names must match Go JSON tags exactly

### General
- English only (comments, names, docs)
- No `any` type without justification
- Functional React components only (no classes)
- Frontend fields align with backend JSON tags

## Constraints

### DO NOT
- ❌ Use `any` without justification
- ❌ Redeclare Go identifiers in same scope
- ❌ Use arbitrary strings for i18n keys
- ❌ Mix `Msg*`/`Err*` (use `Msg*`)
- ❌ Skip `MessageKey` → `string` conversion
- ❌ Create React class components
- ❌ Bypass TypeScript strict checks

### MUST
- ✅ Match TS properties to Go JSON tags
- ✅ Use `import type` for type-only imports
- ✅ Handle all Go errors explicitly
- ✅ Use context providers for shared state
- ✅ Follow clean architecture
- ✅ Keep i18n en/ru in sync
- ✅ Cover new Go code with unit-tests, keep tests in sync
- ✅ Run backend unit tests after every code change — `go test ./internal/application/... -count=1`
- ✅ Fix failing tests before committing — zero failures required
- ✅ Functional React components only

## Commands

**Backend**:
```bash
cd backend && go mod tidy
go build -o image-toolkit ./cmd/server/
go run ./cmd/server/                         # Dev: http://localhost:5170
go test ./internal/application/... -count=1  # Run all unit tests (ALWAYS after changes)
go test ./internal/application/... -v        # Verbose test output
go test ./... -coverprofile=coverage.out     # Coverage report
```

**Frontend**:
```bash
cd frontend && npm install
npm run dev                    # Dev: http://localhost:5173
npm run build                  # Production
npx tsc -b                     # Type-check
```

## Environment

**Backend (.env)**: `DB_*` (PostgreSQL), `SERVER_HOST/PORT`, `CORS_ORIGINS`, `BOOTSTRAP_LOGIN/PASSWORD`, `OCR_*`
**Frontend (.env)**: `VITE_API_URL` (empty for dev proxy)

## Key Features
1. MD5 + file size duplicate detection
2. Async scanning with progress tracking
3. WebP thumbnails, disk-cached
4. OCR classification (external service)
5. EXIF GPS extraction + geoclustering
6. Session auth + CSRF + rate limiting
7. Custom i18n (en/ru), strict typing
8. Background jobs: sync, cleanup, OCR, tags

## MCP Tools
The following MCP servers are connected and available for agent tasks:

| Server | Purpose |
|---|---|
| **filesystem** | Read, write, edit, move, and search project files |
| **github** | Manage repos, branches, PRs, issues, commits |
| **postgres** | Run read-only SQL queries against the database |
| **sequentialthinking** | Break down complex multi-step problems with revision support |
| **context7** | Provides up-to-date version-specific docs for external libraries and frameworks |

Prefer using these MCP tools over raw CLI commands where applicable: use [`filesystem`](.roo/mcp.json) for file operations (read/write/search), [`github`](.roo/mcp.json) for Git workflows, [`postgres`](.roo/mcp.json) for database inspection, [`sequentialthinking`](.roo/mcp.json) for planning non-trivial logic, [`context7`](.roo/mcp.json) for up-to-date external dependencies docs.

## Validation
- `npx tsc -b` (frontend)
- `go build ./...` (backend)
- `go test ./internal/application/... -count=1` (backend unit tests — must pass with 0 failures)
- `npm run lint` (ESLint)
- Verify i18n en/ru consistency
