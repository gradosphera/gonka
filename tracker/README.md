# Gonka Chain Observer

Minimalistic full-stack application for observing Gonka Chain with inference statistics tracking.

## Features

- Real-time inference statistics for current epoch
- Historical epoch statistics with height-wise caching
- SQLite database for immutable historical data
- Multi-URL Gonka Chain client with automatic failover
- Background polling every 30 seconds
- Interactive dashboard with visual highlighting

## Structure

- `backend/` - FastAPI backend (Python 3.11)
- `frontend/` - React + TypeScript frontend (Vite)
- `planning/` - Task planning and specifications
- `config.env` - Environment configuration
- `Makefile` - Setup and run commands
- `docker-compose.yaml` - Traefik reverse proxy + services

## Setup

```bash
make setup-env
```

## Run

```bash
make run-app
```

Application available at `http://localhost`:
- Frontend: `http://localhost/`
- Backend API: `http://localhost/api/v1/hello`
- Inference Stats: `http://localhost/api/v1/inference/current`

## Test

```bash
make test-all
```

- `test-backend` - Backend unit tests
- `test-integration` - Live service tests
- `test-all` - Complete test suite

## Development

Backend:
```bash
cd backend
uv run uvicorn backend.app:app --reload --host 0.0.0.0 --port 8080
```

Frontend:
```bash
cd frontend
npm run dev
```

