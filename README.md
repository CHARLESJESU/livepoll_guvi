# Pulse live polls

Pulse is a small live polling app: authenticated creators make a poll, share its URL, and an audience votes while every open poll page updates without refresh.

Detailed setup, architecture, folder, database, Redis, and Docker documentation is in [docs/PROJECT_GUIDE.md](docs/PROJECT_GUIDE.md).

## Stack

- `frontend/`: React + Vite single-page app
- `backend/`: Go + Gin API
- MongoDB: users, polls, and one-vote-per-voter records
- Redis: live counter hashes and pub/sub events consumed by Server-Sent Events

## Run locally

Requirements: Docker Desktop.

```powershell
docker compose up --build
```

Open http://localhost:5173. Create an account, create a poll, then open its generated URL in another browser window to vote. The creator view receives Redis pub/sub events through the Go SSE endpoint.

Without Docker, start MongoDB and Redis separately, run `go run .` in `backend`, and run `npm install; npm run dev` in `frontend`.

## API

- `POST /api/auth/signup`, `POST /api/auth/login`
- `POST /api/polls` (Bearer token required)
- `GET /api/polls/:id`
- `POST /api/polls/:id/votes`
- `GET /api/polls/:id/events` (SSE)

Set `MONGO_URL`, `REDIS_URL`, `FRONTEND_URL`, `SESSION_SECRET`, and `PORT` in deployment. Deploy the backend with reachable MongoDB/Redis services and the frontend as a static Vite build with `VITE_API_URL` pointing at the backend. A public deployment URL and video must be supplied by the project owner after choosing hosting accounts; this workspace cannot create those third-party resources.
