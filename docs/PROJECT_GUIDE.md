# Project Guide

## 1. What this project does

Pulse is a live polling application.

1. A user signs up or logs in.
2. The user creates a poll with a question and 2-6 options.
3. The application returns a shareable poll URL.
4. Participants open the URL and submit one vote.
5. MongoDB stores durable users, polls, and votes.
6. Redis updates live vote counters and publishes the new result.
7. The Go backend sends Redis updates to open browsers using Server-Sent Events.
8. React updates the result bars immediately without refreshing.

## 2. Run the project

### Requirements

- Docker Desktop
- Windows PowerShell

### Start

From the project root:

```powershell
cd C:\aboutme\mydudy\personal\Guviinternship2
docker compose up --build
```

Open:

```text
Frontend: http://localhost:5173
Backend:  http://localhost:8080
```

Check backend health:

```text
http://localhost:8080/health
```

### Stop

```powershell
docker compose down
```

This stops containers without deleting the named MongoDB volume.

Delete application data and containers:

```powershell
docker compose down -v
```

Use the destructive `-v` command only when a clean database reset is intended.

### Run locally without Docker

MongoDB must be available on `mongodb://localhost:27017`, and Redis on `localhost:6379`.

Start the backend:

```powershell
cd C:\aboutme\mydudy\personal\Guviinternship2\backend
go run .
```

Start the frontend in another terminal:

```powershell
cd C:\aboutme\mydudy\personal\Guviinternship2\frontend
npm install
npm run dev
```

## 3. Application flow

### Authentication

1. React sends email and password to `/api/auth/signup` or `/api/auth/login`.
2. Gin validates the request.
3. The password is converted to a bcrypt hash during signup.
4. MongoDB stores the user record.
5. Login verifies the hash and returns a signed Bearer token.
6. React stores the token in browser `localStorage`.

### Poll creation

1. An authenticated React request calls `POST /api/polls`.
2. Gin checks the question and option count and trims the values.
3. MongoDB stores the poll.
4. Redis creates a counter hash initialized to zero.
5. React changes the URL to `/?poll=<poll-id>` and displays the poll.

### Voting and live updates

1. The audience browser creates or reads a voter ID from `localStorage`.
2. React sends `POST /api/polls/:id/votes`.
3. Gin validates the option and voter ID.
4. MongoDB stores the vote using a unique `(pollId, voter)` index.
5. Redis atomically increments the option count in a hash.
6. Redis publishes the complete count as a message.
7. The Go SSE endpoint receives Redis pub/sub data and sends `data: ...` events.
8. React's `EventSource` updates the result bars.
9. Reloading reads MongoDB poll data and Redis counters.

## 4. Folder structure

```text
Guviinternship2/
|-- backend/
|   |-- Dockerfile       Builds the Go server image
|   |-- go.mod           Go modules and dependency versions
|   `-- main.go          API, validation, auth, MongoDB, Redis, and SSE
|-- frontend/
|   |-- src/
|   |   |-- main.jsx     React screens and API/EventSource calls
|   |   `-- style.css    Interface styles
|   |-- index.html       Browser entry document
|   |-- package.json     React and Vite dependencies/scripts
|   `-- vite.config.js   Vite and React plugin configuration
|-- docs/
|   `-- PROJECT_GUIDE.md This documentation
|-- docker-compose.yml   Runs MongoDB, Redis, backend, and frontend
|-- .gitignore           Excludes local/generated files
`-- README.md            Short project summary
```

### Important files

| File | Purpose |
|---|---|
| `backend/main.go` | Starts Gin, connects MongoDB and Redis, defines all API routes, validates input, and streams live updates |
| `backend/Dockerfile` | Builds the Go executable and runs it in a small Alpine image |
| `frontend/src/main.jsx` | Handles signup/login, poll creation, voting, sharing, and real-time result updates |
| `frontend/src/style.css` | Defines the responsive visual design |
| `frontend/package.json` | Defines npm dependencies and the `dev`/`build` commands |
| `docker-compose.yml` | Defines containers, ports, environment variables, dependencies, and MongoDB storage |
| `docs/PROJECT_GUIDE.md` | Setup and architecture documentation |

## 5. API endpoints

| Method | Endpoint | Authentication | Purpose |
|---|---|---|---|
| GET | `/health` | No | Verify that the backend is running |
| POST | `/api/auth/signup` | No | Create an account |
| POST | `/api/auth/login` | No | Sign in and receive a token |
| POST | `/api/polls` | Bearer token | Create a poll |
| GET | `/api/polls/:id` | No | Load one poll and current counts |
| POST | `/api/polls/:id/votes` | No | Submit one vote |
| GET | `/api/polls/:id/events` | No | Receive live SSE result updates |

## 6. Why each technology is used

| Technology | Role | Why it is useful here |
|---|---|---|
| React | Frontend | Component-based UI and browser state for login, poll creation, and live result bars |
| Vite | Frontend tooling | Fast local development and an optimized static production build |
| Go | Backend | Fast, compiled server with explicit concurrency and low memory usage |
| Gin | Go web framework | Routing, JSON handling, middleware, CORS, and HTTP responses |
| MongoDB | Persistent database | Stores structured but evolving documents for users, polls, and unique vote records |
| Redis | Real-time and counters | Provides atomic counter increments and pub/sub live notifications |
| SSE | Browser live transport | Keeps an HTTP connection open and pushes Redis result updates to React |
| Docker | Runtime packaging | Starts consistent MongoDB, Redis, backend, and frontend services |
| Docker Compose | Service orchestration | Starts and connects all project services with one command |

“MangoDB” is not used in this project. The required database is **MongoDB**.

## 7. View stored MongoDB data

Open the MongoDB shell inside the Docker container:

```powershell
docker compose exec mongo mongosh livepoll
```

Useful commands:

```javascript
show collections
db.users.find().pretty()
db.polls.find().pretty()
db.votes.find().pretty()

db.users.countDocuments()
db.polls.countDocuments()
db.votes.countDocuments()

exit
```

MongoDB collection meanings:

- `users`: email and bcrypt password hash
- `polls`: question, options, owner ID, and creation time
- `votes`: poll ID, browser voter ID, selected option, and creation time

The vote collection has a unique index on `(pollId, voter)`, preventing the same browser ID from voting twice in the same poll.

## 8. View Redis data

Open Redis CLI:

```powershell
docker compose exec redis redis-cli
```

Useful commands:

```text
KEYS *
SCAN 0
HGETALL poll:<poll-id>:counts
PUBSUB CHANNELS poll:*
```

For example, `HGETALL poll:64abc123:counts` may return:

```text
0 3
1 5
2 1
```

The Redis key stores option indexes and counts:

- `poll:<poll-id>:counts`: Redis hash containing live option counts
- `poll:<poll-id>`: Redis pub/sub channel delivering live count updates

Redis is the live counter/pub-sub layer. MongoDB is the durable database.

If Redis loses a poll counter hash, the backend detects the missing or
incomplete hash, aggregates that poll's durable votes from MongoDB, and
recreates the Redis hash before returning counts. Redis can therefore be
restarted without losing voting results.

## 9. View Docker containers and storage

List running services:

```powershell
docker compose ps
```

View service logs:

```powershell
docker compose logs -f backend
docker compose logs -f frontend
docker compose logs -f mongo
docker compose logs -f redis
```

Open a shell in a service:

```powershell
docker compose exec backend sh
docker compose exec frontend sh
```

Docker Desktop also provides a Containers view. Click a container to see logs, terminal access, environment variables, and network details.

MongoDB data is stored in the named Docker volume configured as:

```yaml
volumes:
  - mongo-data:/data/db
```

List and inspect Docker volumes:

```powershell
docker volume ls
docker volume inspect guviinternship2_mongo-data
```

The exact volume name can differ based on the parent folder name or Compose project name. Find it with `docker volume ls`.

Redis data is primarily kept in memory. The current Compose setup does not mount a Redis volume, so recreating or removing the Redis container can remove its counter state.

## 10. Environment variables

| Variable | Used by | Default |
|---|---|---|
| `MONGO_URL` | Go backend | `mongodb://localhost:27017` |
| `MONGO_DB` | Go backend | `livepoll` |
| `REDIS_URL` | Go backend | `redis://localhost:6379` |
| `FRONTEND_URL` | Go CORS | `http://localhost:5173` |
| `SESSION_SECRET` | Go auth token signing | required; no default |
| `PORT` | Go backend | `8080` |
| `VITE_API_URL` | React frontend | `http://localhost:8080` |

For deployment, use an authenticated MongoDB Atlas URI in `MONGO_URL` and an
authenticated Redis URI in `REDIS_URL`. Use `rediss://` when the Redis provider
supports TLS. The backend accepts these URI formats directly. Set
`SESSION_SECRET` to a non-default secret; startup fails if it is missing or
uses a development placeholder. Set `FRONTEND_URL` to the exact deployed
frontend origin and build the frontend with `VITE_API_URL` set to the deployed
backend URL.

The vote uniqueness rule is enforced by MongoDB's unique `(pollId, voter)`
index. `voter` is a browser-generated identifier, so it prevents repeat votes
from the same browser identity, not from the same real person.

The `/api/polls/:id/events` endpoint uses Server-Sent Events. The production
hosting platform and reverse proxy must support long-lived HTTP connections,
disable response buffering for this route, and allow connections to remain
open.
