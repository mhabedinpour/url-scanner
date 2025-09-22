# URL Scanner Service

> Queue, scan, and retrieve results for URLs using the public urlscan.io API, with PostgreSQL storage, a REST API, metrics, and a queue UI. Includes cooperative rate limiting and JWT-authenticated endpoints.

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev/dl/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-336791?logo=postgresql)](https://hub.docker.com/_/postgres)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker)](https://docs.docker.com/compose/)
[![License](https://img.shields.io/badge/license-MIT-informational)](#)

---

- Fast start with Docker Compose
- River Queue–based background workers
- OpenAPI/Swagger UI, Prometheus metrics, pprof
- Cooperative rate limiting that adapts to urlscan.io headers

---

## Table of Contents
- [Architecture at a Glance](#architecture-at-a-glance)
- [Project Design](#project-design)
  - [Components](#components)
  - [Data Flow](#data-flow)
- [Packages](#packages)
- [Run the Project](#run-the-project)
  - [Requirements](#requirements)
  - [Start Dependencies (Docker Compose)](#start-dependencies-docker-compose)
  - [Configure the Service](#configure-the-service)
  - [Generate JWT Keys](#generate-jwt-keys)
  - [Get urlscan.io API Key](#get-urlscanio-api-key)
  - [Migrate the Database](#migrate-the-database)
  - [Run the Service](#run-the-service)
  - [Generate a Test JWT](#generate-a-test-jwt)
- [Configuration](#configuration)
  - [Parameters](#parameters)
  - [Sample config.yml](#sample-configyml)
- [Rate Limiting Strategy](#rate-limiting-strategy)
  - [Other Possible Solutions and Trade-offs](#other-possible-solutions-and-trade-offs)
- [Retry and Failure Handling](#retry-and-failure-handling)
- [Make Commands](#make-commands)
- [Notes and Tips](#notes-and-tips)

---

## Architecture at a Glance

```mermaid
flowchart LR
    Client[Client]
    API[API Server]
    DB[(PostgreSQL)]
    Worker[Worker]
    Urlscan[urlscan.io API]

    Client -->|enqueue scan| API
    API -->|store scan + enqueue job| DB
    DB -->|job using river| Worker
    Worker -->|submit & poll| Urlscan
    Urlscan -->|result| Worker
    Worker -->|persist result| DB
    Client -->|fetch results| API
    DB -->|result| API
```

---

## Project Design

### Components
- API server (`internal/api`): HTTP server with v1 REST endpoints, OpenAPI/Swagger UI, Prometheus metrics, pprof, and River Queue UI.
- Scanner service (`internal/scanner`): Core domain logic for enqueuing URL scans, de-duplicating jobs per URL, caching results, and orchestrating scan execution.
- Worker (`internal/worker`): River-based background worker that processes scan jobs, enforces cooperative rate limiting, and handles retries/snoozing.
- urlscan.io client (`pkg/urlscanner/urlscanio`): Client to submit scans and fetch results from urlscan.io, parsing rate-limit headers.
- Storage (`pkg/storage/postgres`): PostgreSQL persistence for scans and River Queue; migrations via goose and rivermigrate.
- CLI (`cmd`): scanner CLI with subcommands to run the service, migrate DB, and generate JWTs.
- Supporting packages: logging, metrics, errors, controller/middleware, domain models.

### Data Flow

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Server
    participant Q as River Queue
    participant W as Worker
    participant U as urlscan.io
    participant P as PostgreSQL

    C->>A: Enqueue scan(URL)
    A->>P: Store Scan(pending)
    A->>Q: Add unique job per URL
    Q->>P: Persist job
    W->>Q: Reserve job
    W->>W: reserveRL() // cooperative RL
    W->>U: Submit URL
    U-->>W: 202 + RL headers
    W->>U: Poll for result (exp backoff)
    U-->>W: Result ready
    W->>P: Update all pending scans (completed+result)
    C->>A: Get result
    A->>P: Read
    A-->>C: Scan result
```

---

## Packages
- cmd/
  - main.go: Root CLI (scanner) and wiring of subcommands.
  - scan.go: `scanner scan` → runs API server and worker(s).
  - migrate.go: `scanner migrate` → applies DB and River Queue migrations.
  - jwt.go: `scanner jwt` → generates RS256 JWTs.
- internal/
  - api/: HTTP server wiring, OpenAPI spec, routes, middleware, metrics, swagger UI, River UI.
  - config/: Loads YAML/env configuration into a strong type.
  - scanner/: Core domain service to enqueue/process scans, caching and orchestration.
  - worker/: River worker implementation with cooperative rate limiting.
- pkg/
  - controller/: HTTP middlewares (logging, CORS, pprof mux, etc.).
  - domain/: Domain types for scans, results, users, statuses.
  - logger/: Zap-based logger setup with context helpers.
  - metrics/: Metrics helpers (OpenTelemetry → Prometheus exporter).
  - serrors/: Sentinel errors and wrappers (e.g., ErrBadRequest, ErrNotFound, ErrRateLimited, ErrConflict).
  - storage/: Storage interfaces and implementations.
    - postgres/: PostgreSQL implementation (connections, queries, jobs, scans).
  - urlscanner/:
    - interface and types used by scanner service.
    - urlscanio/: Implementation talking to urlscan.io API; parses rate-limit headers.
- migrations/: Goose SQL migrations for service tables; River migrations applied programmatically.
- scripts/: Lint and test scripts.

---

## Run the Project

### Requirements
- Go 1.22+
- Docker + Docker Compose (for Postgres)
- urlscan.io API key (free account works) https://urlscan.io/user/signup/
- RSA key pair for JWTs (used by API auth)

### Start Dependencies (Docker Compose)
The repository ships a `docker-compose.yml` for PostgreSQL 17.

```bash
make start-docker-compose
# to stop
make stop-docker-compose
# to remove volumes
make remove-docker-compose
```

### Configure the Service
Copy `config.sample.yml` to `config.yml` and adjust as needed:
- HTTP server address, timeouts
- Database connection (defaults match docker-compose)
- JWT public/private keys (PEM)
- Scanner: urlscan.io API key, attempts and cache TTL
- Worker: timeouts and concurrency

> Tip: A shortened example is shown under [Sample config.yml](#sample-configyml). See full comments in `config.sample.yml`.

### Generate JWT Keys

<details>
<summary>Create an RSA key pair and paste into config.yml (click to expand)</summary>

```bash
# Private key (used by scanner jwt command and server-side signing if applicable)
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out jwt_private.pem

# Public key (server uses this to verify tokens)
openssl rsa -in jwt_private.pem -pubout -out jwt_public.pem
```

Place contents into `config.yml`:

```yaml
jwt:
  publicKey: |-
    <paste contents of jwt_public.pem>
  privateKey: |-
    <paste contents of jwt_private.pem>
```
</details>

### Get urlscan.io API Key
Sign up → Account → API → copy the key. Put it into `config.yml` under `scanner.urlscanioApiKey`.

### Migrate the Database
Ensure Postgres is running (via Docker Compose or your own instance).

```bash
make migrate-up
# Under the hood this runs:
# go run cmd/* -c config.yml migrate
```

This applies service migrations (`migrations/*.sql`) via goose and RiverQueue’s own migrations.

### Run the Service

```bash
# From repo root
go run ./cmd/* -c config.yml scan
```

What starts:
- API server on the configured address (default `:8080`)
- Background worker(s) with configured concurrency and timeouts

Useful endpoints:
- `/v1/docs/` → Swagger UI (OpenAPI spec at `/specs/v1.yaml`)
- `/metrics` → Prometheus metrics (path configurable)
- `/riverui/` → River Queue admin UI
- `/debug/pprof/` → pprof endpoints

### Generate a Test JWT
To call authenticated APIs you’ll need a JWT signed with your private key and having a subject (user ID).

```bash
go run ./cmd/* -c config.yml jwt --subject <USER_ID> --ttl 24h
# Use as: Authorization: Bearer <token>
```

---

## Configuration

### Parameters

| Section  | Keys (env var) | Description |
|----------|-----------------|-------------|
| environment | `ENVIRONMENT` | `development` or `production` |
| http | `HTTP_ADDR`, `HTTP_*_TIMEOUT`, `HTTP_MAX_HEADER_BYTES`, `HTTP_METRICS_PATH` | Addr, timeouts, metricsPath, maxHeaderBytes |
| database | `DATABASE_USERNAME`, `DATABASE_PASSWORD`, `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_SSL_MODE`, `DATABASE_NAME`, pool settings | Postgres connection and pool |
| jwt | `JWT_PUBLIC_KEY`, `JWT_PRIVATE_KEY` | PEM strings |
| scanner | `SCANNER_MAX_ATTEMPTS`, `SCANNER_RESULT_CACHE_TTL`, `SCANNER_URLSCAN_IO_API_KEY` | Scan job options + urlscan.io key |
| worker | `WORKER_JOB_TIMEOUT`, `WORKER_JOB_CONCURRENCY` | Worker runtime |
| gracefulShutdownTimeout | `GRACEFUL_SHUTDOWN_TIMEOUT` | Shutdown deadline |

See definitions in `internal/config/config.go`.

### Sample config.yml

```yaml
environment: development
http:
  addr: ":8080"
  readTimeout: 1m
  readHeaderTimeout: 10s
  writeTimeout: 2m
  idleTimeout: 2m
  requestTimeout: 10s
  maxHeaderBytes: 0
  metricsPath: /metrics
database:
  username: myuser
  password: mypassword
  host: localhost
  port: 5432
  sslMode: disable
  name: scanner
  maxOpenConnections: 10
  maxIdleConnections: 8
  connMaxLifetime: 3m
  connMaxIdleTime: 3m
jwt:
  publicKey: |-
    -----BEGIN PUBLIC KEY-----
    ...
    -----END PUBLIC KEY-----
  privateKey: |-
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
scanner:
  maxAttempts: 5
  resultCacheTtl: 1h
  urlscanioApiKey: "YOUR_URLSCAN_API_KEY"
worker:
  jobTimeout: 1m
  jobConcurrency: 10
gracefulShutdownTimeout: 10s
```

---

## Rate Limiting Strategy

> Cooperative worker-side rate limiting ensures we never exceed urlscan.io’s limits while maximizing throughput.

- Upstream headers: The client parses `X-Rate-Limit-Limit`, `X-Rate-Limit-Remaining`, and `X-Rate-Limit-Reset` on responses. If the API returns `429`, the client converts it to a domain error (`ErrRateLimited`) and passes along the parsed rate-limit status.
- Worker cooperative rate limiting (`internal/worker/URLScannerWorker`):
  - Tracks the last known rate-limit status (limit, remaining, resetAt) and the number of in-flight requests.
  - Before starting a job, `reserveRL` checks if `remaining - inFlightRequests > 0`. If not, it waits until either `resetAt` passes or any in-flight request finishes.
  - On first boot with no known state, it allows a single probe request (synthetic `remaining=1`) to learn real headers.
  - After each request, `requestFinished` updates the last known status conservatively (adopts newer `resetAt`, or the lower `remaining`) and wakes waiters.
  - If a scan fails due to rate limiting, the job is snoozed until `resetAt` (`dur = max(0, resetAt - now)`).
- Scanner behavior: The scanner returns the last seen `RateLimitStatus` from submit/poll operations, and the worker decides whether to snooze based on `ErrRateLimited`; other errors mark pending scans as failed.

### Other Possible Solutions and Trade-offs

Below are alternative approaches you might consider, along with when they fit and what they cost:

- Centralized token bucket in Redis
  - How it works: keep a per-API-key bucket (limit, remaining, resetAt) in Redis. Workers perform atomic reserve/consume with TTL set to the reset window.
  - Pros: global coordination across many processes/hosts; precise control; dynamic throttling via ops changes in Redis.
  - Cons: adds infra dependency and latency; needs careful Lua/transaction logic to avoid drift; Redis outage can stall throughput.
  - When to use: multiple services or many worker replicas must share a single upstream budget.

- Local limiter using golang.org/x/time/rate
  - How it works: create a rate.Limiter per worker or per API key; call limiter.Wait(ctx) before each upstream call.
  - Pros: tiny dependency, simple, well-tested; smooths bursts; easy to reason about.
  - Cons: purely local unless combined with a shared coordinator; upstream limits are windowed with explicit ResetAt, which token-bucket may not mirror; risk of desync across replicas.
  - When to use: single-node deployments, or combine with a small central coordinator to distribute permits.

- “Send until 429, then back off”
  - How it works: ignore budgeting and rely on upstream 429 responses; upon 429, sleep until ResetAt and retry.
  - Pros: trivial to implement; adapts automatically as upstream changes limits.
  - Cons: bursts may violate provider ToS; noisy errors and log spam; wasted attempts count against job backoff; can cause thundering herd if many workers hit 429 simultaneously.
  - When to use: low-traffic prototypes or internal tooling where occasional 429s are acceptable.

---

## Retry and Failure Handling

> How the worker retries jobs on failures and when it gives up.

- Configuration and enqueueing
  - Max attempts: `scanner.maxAttempts` controls River’s `MaxAttempts` for the job (set via `internal/scanner/job.go`). This is the total number of tries River will run for a job, including the first attempt.
  - Uniqueness: Jobs are unique per URL across states (available, running, retryable, scheduled, completed, pending) within `resultCacheTtl`, so only one retried job exists for a given URL at a time.
- What happens inside one attempt
  - The worker calls `scanner.Scan(ctx, URL)`.
  - The scanner submits the URL and then polls urlscan.io for a bounded time using exponential backoff between polls. If polling times out or the provider returns an error, the attempt fails.
  - On non-rate-limit errors, the scanner marks all pending scans for that URL as `failed` with the last error to provide immediate feedback to users, while the job itself may still retry (see below).
- Error mapping → job retry behavior (in `internal/worker/urlscanner.go`)
  - Success: job completes; pending scans for the URL are updated to `completed` with the result.
  - Conflict (`ErrConflict`): returned when there are no pending scans left for the URL (e.g., users deleted requests). The job is canceled (no retries), since there’s nothing to do.
  - Rate limited (`ErrRateLimited`): the worker snoozes the job until the upstream reset time (`resetAt`). River will re-run the job after the snooze period. This does not count as a failed attempt.
  - Other errors: the worker returns an error; River marks the job retryable and reschedules it according to its backoff strategy, incrementing the attempt count.
- When retries stop
  - River stops retrying after `MaxAttempts` is exhausted. At that point the job’s final state is `failed`. Because the scanner already marked pending scans as `failed` on the last non-rate-limit error, user-visible state is consistent with the job outcome.
- Notes
  - Backoff: River handles job-level retry scheduling (exponential backoff by default). This is separate from the scanner’s polling backoff within a single attempt.
  - Observability: Use `/riverui/` to inspect attempts, next run time, snoozes, and failures. Prometheus metrics and logs include attempt and error information.

---

## Make Commands

| Group | Command | Description |
|------:|---------|-------------|
| Dependency | `make dependency` | Install golangci-lint, mockgen, goimports, ogen |
|  | `make install-golangci-lint` | Install linter |
|  | `make install-mockgen` | Install mockgen |
|  | `make install-goimports` | Install goimports |
|  | `make install-ogen` | Install ogen |
| CI | `make fix-imports` | Run goimports on cmd, internal, pkg |
|  | `make lint` | Run lints (`./scripts/lint.sh`) |
|  | `make test` | Run tests (`./scripts/test.sh`) |
|  | `make generate` | `go generate ./...` |
|  | `make install-hooks` / `make uninstall-hooks` | Manage pre-commit hooks |
| Dev | `make start-docker-compose` | `docker compose up -d` (Postgres) |
|  | `make stop-docker-compose` | Stop containers |
|  | `make remove-docker-compose` | `docker compose down -v` |
|  | `make clean` | Alias to remove-docker-compose |
| Migrations | `make create-migration name=<NAME>` | `goose -s create <NAME> sql` |
|  | `make migrate-up` | `go run cmd/* -c config.yml migrate` |

---

## Notes and Tips

> OpenAPI and docs: Visit `/v1/docs/` when the server is running to explore endpoints. The raw spec lives at `/specs/v1.yaml`.

> Metrics: Scrape `/metrics` with Prometheus; OpenTelemetry exporter is wired to the Prometheus registry.

> River Queue UI: Visit `/riverui/` to monitor jobs.

> pprof: `/debug/pprof/` for profiling.

> Database defaults: The defaults in `config.sample.yml` match `docker-compose.yml` (localhost:5432, user=myuser, password=mypassword, db=scanner).

> Caching behavior: When a recent completed scan exists within `resultCacheTtl`, new requests for the same URL immediately reuse that result (deduped job).

> Concurrency control: Worker concurrency is configurable; real concurrency may be lower when cooperative RL blocks until budget is available.
