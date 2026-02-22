# ReminderIn

<details>
  <summary>Dashboard Screenshot (click to expand)</summary>
  <br />
  <p align="center">
    <a href="https://github.com/user-attachments/assets/e6b7bdc9-0b09-49cf-bf37-cb249a1fdd96">
      <img src="https://github.com/user-attachments/assets/e6b7bdc9-0b09-49cf-bf37-cb249a1fdd96" alt="ReminderIn dashboard screenshot" width="380" />
    </a>
  </p>
</details>

Personal WhatsApp reminder app with a lightweight Go backend and web dashboard.  
Schedule one-time or recurring reminders, then deliver them to yourself, contacts, or groups.

## Features

- Login-protected dashboard with HTTP-only JWT cookie auth.
- Brute-force protection for login attempts (IP-based limiter).
- WhatsApp linking via QR scan or phone pairing code.
- One-time and recurring reminder scheduling (cron format).
- Multi-target delivery (self, direct number, group, or JID).
- Search, sort, pagination, and ETag caching for reminder list.
- SQLite persistence (WAL mode) for reminders and app settings.
- Security middleware: same-origin checks and hardened response headers.
- Low-resource friendly runtime for small-memory servers.

## Tech Stack

| Layer      | Stack |
| ---------- | ----- |
| Language   | Go 1.25 |
| HTTP       | `net/http` + `chi` |
| Database   | SQLite (`mattn/go-sqlite3`) |
| Scheduler  | `robfig/cron/v3` |
| Auth       | JWT (`golang-jwt/jwt/v5`) |
| WhatsApp   | `whatsmeow` |
| Frontend   | HTML, CSS, Vanilla JS |

## Project Structure

```text
Reminderin/
|-- cmd/
|   `-- api/
|       |-- main.go
|       |-- scheduler.go
|       `-- main_test.go
|-- internal/
|   |-- handler/
|   |   |-- api.go
|   |   `-- api_test.go
|   |-- store/
|   |   |-- sqlite_store.go
|   |   `-- sqlite_store_test.go
|   `-- whatsapp/
|       `-- client.go
|-- web/
|   |-- embed.go
|   `-- static/
|       |-- index.html
|       |-- css/
|       |-- js/
|       `-- vendor/
|-- data/
|-- Dockerfile
|-- .dockerignore
|-- .env.example
|-- go.mod
`-- go.sum
```

## Environment Variables

| Variable | Required | Default | Description |
| -------- | -------- | ------- | ----------- |
| `REMINDERIN_USERNAME` | Yes | - | Admin username for login. |
| `REMINDERIN_PASSWORD` | Yes | - | Admin password for login. |
| `JWT_SECRET` | Yes | - | JWT signing secret (use at least 32 random bytes). |
| `PORT` | No | `8080` | HTTP server port. |
| `DB_PATH` | No | `data/reminderin.db` | Main SQLite DB path for app data. |
| `JWT_EXP_HOURS` | No | `168` | JWT expiration in hours. |
| `WA_LOAD_ALL_CLIENTS` | No | `false` | Load all stored WhatsApp sessions on startup. |
| `HTTP_ACCESS_LOG` | No | `false` | Enable request logging middleware. |
| `LOGIN_MAX_ATTEMPTS` | No | `5` | Failed attempts before temporary lock. |
| `LOGIN_LOCK_SECONDS` | No | `60` | Lock duration after max failed attempts. |
| `LOGIN_MAX_TRACKED_IPS` | No | `10000` | Maximum tracked login state entries. |
| `LOGIN_TRACK_TTL_SECONDS` | No | `86400` | TTL for tracked login states. |
| `LOGIN_TRACK_CLEANUP_SECONDS` | No | `300` | Cleanup interval for login state map. |
| `TRUST_PROXY_HEADERS` | No | `false` | Trust `X-Forwarded-*` headers from reverse proxy. |
| `WA_MAX_LINK_SESSIONS` | No | `2` | Max concurrent WA linking sessions (SSE). |
| `WA_SEND_TIMEOUT_SECONDS` | No | `20` | Timeout for WA send operation. |
| `WA_QUERY_TIMEOUT_SECONDS` | No | `20` | Timeout for WA directory queries. |
| `WA_DIRECTORY_CACHE_TTL_SECONDS` | No | `60` | Cache TTL for contacts/groups lookup. |
| `REMINDER_DUE_BATCH_LIMIT` | No | `200` | Max due reminders processed per scheduler cycle. |
| `ALLOW_INSECURE_DEFAULTS` | No | `false` | Development-only fallback for missing secrets/credentials. |

## Getting Started

1. Clone repository:

```bash
git clone <your-repo-url>
cd Reminderin
```

2. Create env file:

```bash
cp .env.example .env
```

3. Fill required values in `.env`:

```env
REMINDERIN_USERNAME=your_admin_username
REMINDERIN_PASSWORD=your_strong_password
JWT_SECRET=your_very_long_random_secret_min_32_bytes
```

4. Install dependencies:

```bash
go mod download
```

5. Run app:

```bash
go run ./cmd/api
```

6. Open dashboard:

```txt
http://localhost:8080
```

## Docker

Build image:

```bash
docker build -t reminderin:latest .
```

Run container:

```bash
docker run -d --name reminderin \
  -p 8080:8080 \
  -v reminderin_data:/app/data \
  -e REMINDERIN_USERNAME=your_admin_username \
  -e REMINDERIN_PASSWORD=your_strong_password \
  -e JWT_SECRET=your_random_secret_min_32_bytes \
  reminderin:latest
```

Open app:

```txt
http://localhost:8080
```

## Deployment Notes

- For small-memory VPS, `256-350MB` budget is generally safe for personal usage.
- Recommended runtime env on low RAM:
  - `WA_LOAD_ALL_CLIENTS=false`
  - `REMINDER_DUE_BATCH_LIMIT=100` or `200`
  - `GOMEMLIMIT=280MiB`
  - `GOGC=80`
