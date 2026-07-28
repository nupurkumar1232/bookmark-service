# Bookmark Service

A read-later bookmarking service built in Go, deployed to production with CI/CD.

🔗 Live at: [https://nupurkumar.me](https://nupurkumar.me)

## What this is

A learning project that goes beyond tutorial-level backend work. I built it to understand production deployment end-to-end: not just "make an API," but "how does a real service stay alive."

## Tech stack

- **Backend:** Go 1.26, chi router, pgx (Postgres driver)
- **Database:** Postgres 16
- **Container:** Multi-stage Docker (~15 MB final image)
- **Reverse proxy:** Nginx with rate limiting
- **TLS:** Let's Encrypt cert with auto-renewal
- **Monitoring:** Prometheus + Grafana on the same host
- **CI/CD:** GitHub Actions — push to main triggers test → build → SSH deploy
- **Server:** DigitalOcean droplet ($6/month), Ubuntu 24.04

## Features

- Email/password signup with bcrypt-hashed passwords
- JWT-based authentication with 24-hour expiry
- Per-user bookmark scoping
- Prometheus metrics at `/metrics`
- Graceful shutdown with 15-second drain window
- `/health` verifies database connectivity (returns 503 if DB unreachable)
- Rate limiting: 5/min per IP on `/signup`, 60/min elsewhere

## API
POST /signup # email + password → 201 created
POST /login # email + password → 200 with JWT
GET /bookmarks # auth required → list bookmarks
POST /bookmarks # auth required → create bookmark
GET /health # checks DB connectivity
GET /metrics # Prometheus format

## Local development

```bash
docker compose up -d
go run main.go
```

Tests:
```bash
go test ./...
```

## Project structure
.
├── main.go # server code
├── main_test.go # tests
├── Dockerfile # multi-stage Go build
├── docker-compose.yml # production stack
├── docker-compose.override.yml # local dev overrides (gitignored)
├── prometheus.yml # scrape config
├── .github/workflows/ci.yml # CI/CD pipeline
├── alerts.md # what I'd alert on in production
├── breakage_log.md # things I broke on purpose to learn
├── journal.md # development journal
└── DEPLOY.md # deploy + rollback runbook

## Things I learned the hard way

1. **Docker bypasses ufw.** Publishing a port with `ports: "8080:8080"` opens it to the internet regardless of firewall rules. Fix: bind to specific interface — `127.0.0.1:8080:8080`.

2. **Go compile on 1 GB droplets needs swap.** Without it, `go build` swap-thrashes and takes ~100 minutes instead of ~3. Fix: 2 GB swapfile.

3. **Diverged Git history from working on multiple machines.** Resolved with reset + cherry-pick. Lesson: always `git pull` at the start of work.

4. **bcrypt cost 10 is the latency bottleneck under load.** /signup p95 went from ~100ms to ~8s under 200 concurrent signups.

5. **`/health` that returns 200 even when the DB is down is worse than useless.** Load balancers can't route around broken instances.

## License

MIT
