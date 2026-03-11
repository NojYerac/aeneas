# Docker Compose Setup

This document explains how to run Aeneas locally using Docker Compose.

## Quick Start

```bash
# Start all services (PostgreSQL + Aeneas)
make up

# View logs
make logs

# Stop services
make down
```

## Services

The `docker-compose.yml` defines three services:

### 1. PostgreSQL (`postgres`)
- Image: `postgres:15-alpine`
- Port: `5432` (configurable via `POSTGRES_PORT`)
- Database: `aeneas` (configurable via `AENEAS_DB_NAME`)
- User: `aeneas` (configurable via `AENEAS_DB_USER`)
- Password: `aeneas` (configurable via `AENEAS_DB_PASSWORD`)
- Volume: `postgres_data` for persistence
- Health check: Waits until PostgreSQL is ready

### 2. Migrations (`migrate`)
- Image: `migrate/migrate:latest`
- Purpose: Runs database migrations from `./migrations` directory
- Depends on: `postgres` (waits for health check to pass)
- Behavior: Runs once and exits after applying migrations

### 3. Aeneas (`aeneas`)
- Image: Built from local `Dockerfile`
- Port: `8080` (configurable via `AENEAS_PORT`)
- Depends on: `migrate` (waits for migrations to complete)
- Health check: Checks TCP port 8080 every 10s
- API endpoint: `http://localhost:8080`
- Health endpoint: `http://localhost:8080/health`

## Configuration

Copy `.env.example` to `.env` to customize settings:

```bash
cp .env.example .env
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AENEAS_DB_NAME` | `aeneas` | PostgreSQL database name |
| `AENEAS_DB_USER` | `aeneas` | PostgreSQL username |
| `AENEAS_DB_PASSWORD` | `aeneas` | PostgreSQL password |
| `POSTGRES_PORT` | `5432` | PostgreSQL host port |
| `AENEAS_PORT` | `8080` | Aeneas API port |
| `AENEAS_LOG_LEVEL` | `info` | Log verbosity (`debug`, `info`, `warn`, `error`) |
| `AENEAS_EXPORTER_TYPE` | `stdout` | Metrics exporter type |
| `AENEAS_HEALTHCHECK_CHECK_INTERVAL` | `30s` | Health check interval |

## Makefile Targets

| Command | Description |
|---------|-------------|
| `make up` | Start all services in detached mode |
| `make down` | Stop and remove all containers |
| `make logs` | Follow logs from all services |
| `make ps` | Show running containers |
| `make restart` | Stop and start all services |
| `make clean` | Stop containers and remove volumes (deletes data!) |

## Startup Sequence

1. **PostgreSQL starts** and waits for health check to pass (up to 25 seconds)
2. **Migrate runs** and applies all migrations from `./migrations` directory
3. **Aeneas starts** after migrations complete successfully
4. **Health check passes** within 30 seconds (3 retries × 10s interval)

## Health Check

After startup, verify the service is healthy:

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy"
}
```

## Troubleshooting

### Service won't start

Check logs for all services:
```bash
make logs
```

Or check individual service logs:
```bash
docker compose logs postgres
docker compose logs migrate
docker compose logs aeneas
```

### Database connection issues

Verify PostgreSQL is healthy:
```bash
docker compose ps
```

Check PostgreSQL logs:
```bash
docker compose logs postgres
```

### Migrations failed

Check migration logs:
```bash
docker compose logs migrate
```

To re-run migrations:
```bash
make down
make up
```

### Port already in use

If port 8080 or 5432 is already in use, either:
1. Stop the conflicting service, or
2. Change the port in `.env`:
   ```
   AENEAS_PORT=8081
   POSTGRES_PORT=5433
   ```

## Development Workflow

### Rebuilding after code changes

```bash
# Rebuild and restart
make down
make up
```

Or use `docker compose up -d --build` to rebuild without stopping:
```bash
docker compose up -d --build aeneas
```

### Database reset

To start with a fresh database:
```bash
make clean  # Removes volumes
make up     # Starts fresh
```

### Accessing the database

Connect directly to PostgreSQL:
```bash
docker compose exec postgres psql -U aeneas -d aeneas
```

Or from host (requires `psql` installed):
```bash
PGPASSWORD=aeneas psql -h localhost -p 5432 -U aeneas -d aeneas
```

## Production Considerations

**⚠️ This setup is for local development only.**

For production deployment:
- Use Kubernetes with Helm chart (see `plans/01-MVP-PLAN.md`)
- Store secrets in a proper secret manager (not environment variables)
- Use managed PostgreSQL (RDS, Cloud SQL, etc.)
- Enable TLS/SSL for database connections
- Configure proper resource limits and autoscaling
- Set up monitoring and alerting
- Use immutable image tags (not `latest`)

## Architecture

```
┌─────────────────┐
│   PostgreSQL    │
│   port: 5432    │
└────────┬────────┘
         │
         │ (migrations)
         │
    ┌────▼─────┐
    │ Migrate  │
    │ (init)   │
    └────┬─────┘
         │
         │ (depends on)
         │
    ┌────▼─────────┐
    │   Aeneas     │
    │  port: 8080  │
    └──────────────┘
```

All services run on a private bridge network (`aeneas`), with only Aeneas and PostgreSQL exposing ports to the host.

## Next Steps

- See [README.md](README.md) for project overview and architecture
- See [plans/01-MVP-PLAN.md](plans/01-MVP-PLAN.md) for roadmap
- Run tests: `make test`
- Build locally: `make build`
