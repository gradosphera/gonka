# Gonka Chain Inference Tracker

Professional full-stack monitoring application for Gonka Chain inference statistics tracking.

## Features

### Backend
- Real-time inference statistics for current epoch (auto-refreshes every 30s)
- Historical epoch statistics with height-wise caching
- SQLite database for immutable historical data
- Multi-URL Gonka Chain client with automatic failover
- Background polling task for continuous updates
- Correct weight extraction from epoch participants
- Models field support per participant
- Computed invalidation rate and missed rate metrics

### Frontend
- Clean, professional dashboard with Gonka.ai inspired design
- Auto-refresh every 30 seconds with countdown timer
- Epoch selector for viewing last 10 epochs
- Comprehensive participant table with:
  - Full participant indexes (monospace font)
  - Correct weights from epoch data
  - Supported models display (gray badges)
  - Total inferenced (inferences + missed)
  - Validated/invalidated counts
  - Missed rate and invalidation rate percentages
- Red highlighting for participants with >10% missed or invalidation rate
- Responsive design with horizontal scroll for mobile

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

