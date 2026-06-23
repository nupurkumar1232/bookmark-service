# Breakage Experiments Log

For each: what I predicted, what actually happened, what the metrics showed,
what I'd do differently next time.

---

Chunk 1: Kill Postgres while app is running (~45 min)
Hypothesis to test: "If Postgres dies, my Go app should return 500s on bookmark endpoints, but /health should still return 200."
Why this matters: A common production pattern is /health checks database connectivity. If yours doesn't, load balancers won't know to route traffic away from a broken instance. We'll observe this gap and decide whether to fix it.

## Experiment 1: Kill Postgres (2026-06-22)
   
   ### Prediction
   - /health: expect 200 (Go app process still running)
   - /bookmarks: expect 500 (no DB connection)
   - Grafana: error rate panel spikes to ~50%

### What actually happened
- /health returned: [actual]
- /bookmarks returned: [actual]
- App logs showed: [paste 2-3 representative lines]
- Recovery: [did app auto-reconnect, or did I need `docker compose restart app`?]
- Grafana error rate peaked at: [actual %]

### Difference from prediction
[honest assessment]

### What I'd do differently in production
- /health should probably do a `SELECT 1` against Postgres so load balancers route around broken instances
- Error logs should include request ID for tracing
- ...

Chunk 2: Exhaust the connection pool (~1 hour)
Hypothesis: "If I send many concurrent requests, the DB pool will saturate. New requests will queue. Eventually they'll time out or fail."
Why this matters: Real services hit connection limits constantly. Understanding what failure looks like is half the battle.
Setup:-
Install a load testing tool on your laptop. We'll use hey (simpler than k6):
bash# On laptop
# Ubuntu
sudo apt install -y hey

# If apt doesn't find it:
go install github.com/rakyll/hey@latest
# Then make sure ~/go/bin is in PATH
Experiment

In Grafana, watch the requests in flight panel maximized.
Hit the service with a controlled load.

## Experiment 2: Load test (2026-06-22)

### Setup
- Tool: hey
- Endpoint hammered: /bookmarks
- Concurrency: 200
- Total requests: 5000

### Observed
- Grafana in-flight peaked at: [X]
- p95 latency went from [normal] to [under load]
- Errors: [count + types]
- DB connection pool: [from logs, did pgx complain about pool exhaustion?]

### Bottleneck identified
[best guess: DB pool? Postgres CPU? bcrypt CPU? Nginx workers?]

## Experiment 1: Postgres kill — observations
[your prediction vs actual from earlier]

## Experiment 2: Load test (concurrent signups + bookmark reads)

### Setup
- Tool: hey + parallel bash curls
- Tested /signup with concurrent unique emails
- Peak request rate: ~80 req/s for ~30 seconds

### Key finding: bcrypt is the bottleneck
- /signup p95 latency went from ~100ms baseline to ~8 seconds under load
- All other endpoints (/, /health, /bookmarks, /metrics) stayed flat
- This means: my service can serve ~10 concurrent signups before tail latency
  becomes unacceptable to users
- Root cause: bcrypt cost factor 10 = ~100ms per hash. CPU-bound. With 200
  concurrent requests on a 1-CPU droplet, they serialize on CPU and queue.

### Possible fixes (not applying now, just noting)
- Lower bcrypt cost to 8 (~25ms/hash) — 4x throughput, marginally less secure
  but still well above the "fast hash" attack threshold
- Move signups behind a queue with rate limiting
- Bigger droplet with more CPUs (vertical scaling)
- Don't allow concurrent signups at all (signup is rare anyway, low priority
  for low latency)
- Could increase pgx pool size to 20 in `pgxpool.ParseConfig`
- Could lower bcrypt cost from 10 to 8 (faster signups, slightly less secure)
- Could add Nginx rate limiting (`limit_req_zone`)

### Secondary finding: error rate panel measures the wrong thing
- My current PromQL counts 4xx as errors, but 4xx means "client made a bad
  request," not "server failed"
- During tests, the panel climbs to 100% mostly because of legitimate 400/401
  responses + 404s from bot traffic
- For production alerting, should filter to status=~"5..":
  `100 * sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))`

### Requests in flight: less useful than expected
- Gauge stayed near 0-2 throughout, even under load
- Likely because (a) Prometheus 15s scrape misses brief spikes, and
  (b) requests pile up at CPU/bcrypt not at the Go HTTP layer
- This metric is more useful under sustained heavy load (W9 load test)

## Experiment 3: Broken deploy rollback (2026-06-23)

### Process
- Broke /bookmarks endpoint, deployed
- Detected via Grafana error rate spike within: [X seconds]
- Rollback via git checkout + docker compose up: [Y seconds total downtime]

### Pain points
- Manual SSH + git pull + rebuild is slow (~2 minutes)
- No automated detection — I had to be watching Grafana
- Rollback was easy because I had a clean git history; would be harder if I'd
  amended commits

### Motivation for W7
Need CI/CD that:
1. Auto-deploys on push to main (fast iteration)
2. Health-checks after deploy
3. Auto-rolls back on health check failure (this is the hard part — skip for now)
