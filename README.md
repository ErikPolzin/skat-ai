# Skat

A playable Skat web app with a Go backend, a React frontend, and configurable AI opponents. The project now focuses on running full human-vs-human and human-vs-agent game sessions, while still keeping the AI evaluation and training tools in the backend.

## Current State

- Full Skat game flow: joining a table, dealing, bidding, skat pickup/discard, game declaration, trick play, scoring, and next-game session flow.
- Game modes: Grand, Suit, and Null games are represented in the rules and scoring engine.
- Real-time play over WebSockets, with REST endpoints for profiles, games, history, ratings, leaderboard, and agent selection.
- Basic Auth-backed player profiles with persistent profile state in the frontend.
- Optional persistent storage using Turso/libSQL or PostgreSQL, with an in-memory database fallback for local development.
- Configurable AI players backed by database agent configs. Current card-play options include heuristic, MCTS, perfect-information minimax variants, neural inference, and random play.
- Training and analysis commands for imitation data generation, behavioral cloning, policy-improvement data, agent evaluation, and bidding-threshold sweeps.
- Deployment support for the backend on Cloud Run and the frontend on Firebase/Vite hosting.

## Repository Layout

```text
.
├── backend/
│   ├── agent/                 # Agent composition, metrics, strategies, neural encoding/IO
│   ├── cmd/
│   │   ├── server/            # HTTP/WebSocket game server
│   │   ├── eval_agent/        # Agent evaluation CLI
│   │   ├── generate_imitation_data/
│   │   ├── generate_policy_improvement_data/
│   │   ├── sweep_bidding_threshold/
│   │   └── train_imitation/
│   ├── game/                  # Core Skat rules, actions, serialization, scoring
│   ├── server/                # Routes, auth, WebSocket handling, cache, DB layer
│   └── logger/                # Local and Cloud Logging setup
└── frontend/
    ├── src/
    │   ├── api/               # REST API client
    │   ├── components/        # Game table, lobby, bidding, results, etc.
    │   ├── context/           # Game and WebSocket context
    │   ├── screens/           # Login, lobby, game, history
    │   └── stores/            # Zustand profile/snackbar stores
    └── public/res/            # Cards, suit icons, profile icons, PWA assets
```

## Prerequisites

- Go 1.25 or newer
- Node.js and npm
- Optional: Turso/libSQL or PostgreSQL for persistence
- Optional: Google Cloud credentials for Cloud Logging, Cloud Run, GCS avatars, or neural weights stored in GCS

## Local Development

### Backend

From the backend module:

```bash
cd backend
go run ./cmd/server
```

The server listens on `PORT` or defaults to `8080`. If `DATABASE_URL` is not set, it uses the in-memory database, which is useful for quick local testing but does not persist games, profiles, ratings, or the seeded SQL agent profiles.

Useful backend commands:

```bash
cd backend
go test ./...
go build ./...
go run ./cmd/eval_agent -agent-type heuristic -games 500
go run ./cmd/sweep_bidding_threshold -strategy heuristic -games 5000
```

### Frontend

Create `frontend/.env.local` with local API and WebSocket URLs:

```bash
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
```

Then run Vite:

```bash
cd frontend
npm install
npm run dev
```

Open the Vite URL, sign in with a username and password, then create or join a game. You can fill empty seats with AI agents from the lobby/game UI.

## Configuration

### Backend Environment

| Variable | Purpose |
| --- | --- |
| `PORT` | HTTP port, default `8080` |
| `DATABASE_URL` | Optional Turso/libSQL, PostgreSQL URL, or PostgreSQL connection string |
| `CORS_ORIGIN` | Optional comma-separated allowed origins beyond local Vite and production defaults |
| `GCS_BUCKET` | Optional bucket for uploaded avatars |
| `LOG_LEVEL` | Logger verbosity |
| `CLOUD_LOGGING_ENABLED` | Enables Google Cloud Logging when set to `true`; auto-enabled on Cloud Run |
| `GCP_PROJECT_ID` | Google Cloud project for Cloud Logging |

### Frontend Environment

| Variable | Purpose |
| --- | --- |
| `VITE_API_URL` | Backend REST base URL |
| `VITE_WS_URL` | Backend WebSocket base URL |

## AI Agents

Agents are assembled from independent bidding, game-choice, and card-play strategies. The database stores those settings in `agent_configs`, and the server builds each agent from that config when the agent joins a game.

When using the SQL schemas, the seeded agents include heuristic, MCTS, and neural card-play variants:

- Bill and Dave: heuristic card play with different bidding thresholds.
- Lisa and Max: MCTS card play with different simulation counts.
- Emma and Sam: neural card play using combined declarer/defender weights from `gs://skat-ai-weights/cardplay.weights`.

Supported card-play strategy names in code include `heuristic`, `mcts`, `minimax`, `minimax-heuristic`, `neural`, and `random`.

## Current Evaluation Snapshot

These numbers were generated on 2026-05-18 with the current evaluator, using 2,000 games per strategy and the default `5050` bidding mode. In this mode the candidate strategy uses the same heuristic bidding and game-choice logic, then alternates between declarer and defender card-play roles against the all-heuristic baseline. Results are stochastic, so reruns will vary.

Commands:

```bash
cd backend
go run ./cmd/eval_agent -agent-type heuristic -games 2000 -skip-gameplay-examples
go run ./cmd/eval_agent -agent-type mcts -games 2000 -mcts-simulations 500 -skip-gameplay-examples
go run ./cmd/eval_agent -agent-type neural -games 2000 -cardplay-weights .data/models/cardplay.weights -skip-gameplay-examples
```

| Card-play strategy | Declarer win rate | Defender win rate | Candidate points | Baseline points | Avg declarer points | Passed games |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Heuristic | 780/1005, 77.6% | 205/995, 20.6% | 17,114 | 18,733 | 17.0 | 417/2000, 20.8% |
| MCTS, 500 simulations | 674/1003, 67.2% | 240/997, 24.1% | 1,297 | 15,683 | 1.3 | 431/2000, 21.6% |
| Neural, `.data/models/cardplay.weights` | 732/1005, 72.8% | 303/995, 30.5% | 12,769 | 11,082 | 12.7 | 414/2000, 20.7% |

Game-type breakdown for candidate declarer games:

| Card-play strategy | Grand | Suit | Null | Improvement vs baseline declarer rate | Point difference per declarer game |
| --- | ---: | ---: | ---: | ---: | ---: |
| Heuristic | 144/165, 87.3% | 501/696, 72.0% | 135/144, 93.8% | -1.8 pp | -1.8 |
| MCTS, 500 simulations | 114/169, 67.5% | 433/667, 64.9% | 127/167, 76.0% | -8.7 pp | -14.4 |
| Neural, `.data/models/cardplay.weights` | 137/156, 87.8% | 495/692, 71.5% | 100/157, 63.7% | +3.3 pp | +1.6 |

## Training And Evaluation

The backend includes research utilities for experimenting with card-play models and bidding thresholds:

```bash
cd backend

# Evaluate a strategy stack
go run ./cmd/eval_agent -agent-type mcts -games 500 -mcts-simulations 500

# Generate supervised card-play labels from minimax/heuristic teachers
go run ./cmd/generate_imitation_data -examples 100000 -output .data/imitation_dataset.csv

# Train the combined declarer/defender neural card-play model
go run ./cmd/train_imitation -dataset .data/imitation_dataset.csv -output .data/models/imitation_cardplay.weights

# Generate policy-improvement examples from rollout scoring
go run ./cmd/generate_policy_improvement_data -examples 20000 -output .data/policy_improvement_dataset.csv

# Sweep bidding thresholds and print CSV results
go run ./cmd/sweep_bidding_threshold -strategy heuristic -games 5000
```

The neural card-play loader supports local weight files and `gs://bucket/path` URIs.

## API Surface

The backend exposes:

- `GET /health`
- `POST /api/profiles`
- Authenticated game/session endpoints under `/api/games`
- Authenticated player history, active games, ratings, leaderboard, avatar upload, and agent listing endpoints
- `GET /ws` WebSocket endpoint using either Basic Auth or the frontend's `skat-auth` WebSocket subprotocol

Most API calls after profile creation require Basic Auth credentials matching the profile.

## Deployment

The backend Dockerfile and `backend/cloudbuild.yaml` target Cloud Run:

```bash
cd backend
gcloud builds submit \
  --substitutions=_DATABASE_URL="${DATABASE_URL}",_CORS_ORIGIN="https://your-frontend.example",_GCS_BUCKET="${GCS_BUCKET}"
```

The frontend is a Vite app and can be built with:

```bash
cd frontend
npm run build
```

Set `VITE_API_URL` and `VITE_WS_URL` for the deployed backend before building.

## Development Notes

- Run backend commands from `backend/`; the Go module is not at the repository root.
- Use Turso/libSQL or PostgreSQL locally if you want the seeded AI profiles from the SQL schemas.
- Generated datasets and model weights are expected under `.data/`, which is not required for normal gameplay unless you enable neural agents.
- Without persistent storage, local games/profiles disappear when the server restarts.
- Current AI agents are conservative about announcements; schneider/schwarz declarations are represented in the rules but AI announcement policy is intentionally minimal.

## License

MIT
