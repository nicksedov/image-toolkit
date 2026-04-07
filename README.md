# Image Dedup

Приложение для поиска и управления дубликатами изображений в медиатеке на локальном диске.

Проект разделен на два подпроекта:
- **backend/** -- Go REST API с бизнес-логикой (сканирование, БД, генерация скриптов)
- **frontend/** -- React + TypeScript веб-интерфейс (Vite, Tailwind CSS, shadcn/ui)

## Возможности

- Сканирование одной или нескольких директорий на наличие дубликатов изображений
- Определение дубликатов по совпадению размера файла и контрольной суммы (MD5)
- Веб-интерфейс с миниатюрами изображений (до 192px)
- Прямое удаление или перемещение файлов в корзину
- Генерация bash/PowerShell скриптов для перемещения файлов
- Пакетная дедупликация по шаблонам папок
- Асинхронное сканирование с отображением прогресса
- Кэширование метаданных в PostgreSQL для ускорения повторных сканирований

## Поддерживаемые форматы

JPG, JPEG, PNG, GIF, BMP, TIFF, TIF, WEBP

## Требования

- Go 1.23 или выше
- Node.js 18 или выше (для фронтенда)
- PostgreSQL 12 или выше

## Структура проекта

```
image-dedup/
├── backend/
│   ├── main.go           # Точка входа, CLI, запуск сервера
│   ├── config.go         # Конфигурация (переменные окружения)
│   ├── database.go       # Подключение к PostgreSQL
│   ├── models.go         # Модели данных (ImageFile, DuplicateGroup)
│   ├── scanner.go        # Сканирование и поиск дубликатов
│   ├── thumbnail.go      # Генерация миниатюр
│   ├── handlers.go       # HTTP обработчики (JSON API)
│   ├── dto.go            # DTO для запросов/ответов API
│   ├── middleware.go      # CORS middleware
│   ├── scan_manager.go   # Асинхронное сканирование
│   ├── .env.example      # Пример конфигурации
│   ├── go.mod
│   └── go.sum
│
├── frontend/
│   ├── src/
│   │   ├── App.tsx       # Главный компонент
│   │   ├── api/          # HTTP клиент и функции API
│   │   ├── types/        # TypeScript интерфейсы
│   │   ├── hooks/        # React хуки
│   │   ├── components/   # UI компоненты
│   │   └── lib/          # Утилиты
│   ├── .env.example      # Пример конфигурации
│   ├── package.json
│   └── vite.config.ts
│
├── .gitignore
└── README.md
```

## Конфигурация

### Backend (`backend/.env`)

| Переменная     | Описание                              | По умолчанию             |
|----------------|---------------------------------------|--------------------------|
| `DB_HOST`      | Хост PostgreSQL                       | `localhost`              |
| `DB_PORT`      | Порт PostgreSQL                       | `5432`                   |
| `DB_USER`      | Пользователь PostgreSQL               | `postgres`               |
| `DB_PASSWORD`  | Пароль PostgreSQL                     | `postgres`               |
| `DB_NAME`      | Имя базы данных                       | `image_dedup`            |
| `SERVER_PORT`  | Порт API сервера                      | `8080`                   |
| `CORS_ORIGINS` | Разрешенные источники (через запятую) | `http://localhost:5173`  |

### Frontend (`frontend/.env`)

| Переменная     | Описание                            | По умолчанию |
|----------------|-------------------------------------|--------------|
| `VITE_API_URL` | URL бэкенд API                      | (пусто -- используется Vite прокси) |

В режиме разработки Vite проксирует запросы `/api/*` на бэкенд (`http://localhost:8080`).
Для продакшена укажите полный URL бэкенда в `VITE_API_URL`.

## Сборка и запуск

### 1. Создание базы данных PostgreSQL

```sql
CREATE DATABASE image_dedup;
```

### 2. Настройка окружения

```bash
# Backend
cp backend/.env.example backend/.env
# Отредактируйте backend/.env -- укажите параметры подключения к БД

# Frontend
cp frontend/.env.example frontend/.env
# Для разработки можно оставить VITE_API_URL пустым (используется прокси)
```

### 3. Сборка бэкенда

```bash
cd backend
go mod tidy
go build -o image-dedup.exe .    # Windows
go build -o image-dedup .        # Linux/macOS
```

### 4. Сборка фронтенда

```bash
cd frontend
npm install
npm run build    # Собирает в frontend/dist/
```

### 5. Запуск (режим разработки)

**Терминал 1 -- бэкенд:**

```bash
cd backend
go run . --port 8080 C:\path\to\photos
```

Или с несколькими директориями:

```bash
go run . --port 8080 C:\photos D:\backup E:\downloads
```

**Терминал 2 -- фронтенд:**

```bash
cd frontend
npm run dev
```

Откройте в браузере: `http://localhost:5173`

### 6. Запуск (продакшен)

```bash
# Бэкенд
cd backend
./image-dedup --port 8080 /path/to/photos

# Фронтенд -- статические файлы из frontend/dist/
# Раздайте через nginx, Caddy или любой другой веб-сервер
```

## API

Все маршруты с префиксом `/api/`. Ответы в формате JSON.

| Метод | Маршрут               | Описание                                |
|-------|-----------------------|-----------------------------------------|
| GET   | `/api/duplicates`     | Группы дубликатов с пагинацией          |
| POST  | `/api/scan`           | Запуск асинхронного сканирования        |
| GET   | `/api/status`         | Статус текущего сканирования            |
| GET   | `/api/thumbnail`      | Миниатюра для файла                     |
| POST  | `/api/generate-script`| Генерация скрипта удаления              |
| POST  | `/api/delete-files`   | Прямое удаление файлов                  |
| GET   | `/api/folder-patterns`| Шаблоны папок для пакетной дедупликации |
| POST  | `/api/batch-delete`   | Пакетное удаление по правилам           |

## Лицензия

MIT
