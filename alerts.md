# Bookmark Service — Alert Runbook

What I'd alert on if this were a real production service.
Each alert has: what it detects, why it matters, what to look at first.

---

## Alert 1: High error rate

**Condition:** 5xx error rate > 1% for 5+ minutes
**PromQL:** `100 * sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) > 1`
**Severity:** Critical (pages on-call)
**Why:** A 5xx means the server is actively failing requests — either a code bug,
a crashed dependency (Postgres), or an OOM kill. Anything above 1% is user-visible.
**First things to check:**
- `docker compose logs app` — is the Go binary panicking?
- `docker compose ps` — is Postgres still healthy?
- `curl https://nupurkumar.me/health` — does the service even respond?
- Recent deployments? A bad deploy is the #1 cause of sudden 5xx spikes.

---

## Alert 2: High p95 latency

**Condition:** p95 latency for any endpoint > 2 seconds for 5+ minutes
**PromQL:** `histogram_quantile(0.95, sum by (le, path) (rate(http_request_duration_seconds_bucket[5m]))) > 2`
**Severity:** Warning (Slack notification, not a page)
**Why:** /signup takes ~100ms normally (bcrypt at cost 10). If it's hitting 2s,
either the server is overloaded, Postgres is slow, or something is blocking the
event loop. Users will notice.
**First things to check:**
- Is CPU/memory spiking? Check DigitalOcean's droplet metrics.
- Postgres connection pool exhausted? (Add DB pool metrics in a future iteration.)
- Is it one endpoint or all? Latency on just /signup = DB issue. All endpoints = server issue.

---

## Alert 3: Service completely down

**Condition:** No successful requests (status=200) for 2+ minutes
**PromQL:** `sum(rate(http_requests_total{status="200"}[2m])) == 0`
**Severity:** Critical (pages on-call immediately)
**Why:** If the service stopped returning 200s entirely, it's either down or
all requests are failing. Different from Alert 1 (which catches partial failures).
**Note:** This alert has a flaw — if traffic is genuinely zero (no users at night),
it'll false-positive. Fix: add a Prometheus synthetic monitor that pings /health
every 30 seconds so there's always some baseline 200 traffic.
**First things to check:**
- `docker compose ps` — are containers running?
- `curl https://nupurkumar.me/health` from outside the droplet
- Was there a reboot or Docker restart?

---

## Alert 4: Requests in flight too high

**Condition:** `http_requests_in_flight > 50` for 1+ minute
**Severity:** Warning
**Why:** My service uses one goroutine per request (standard Go HTTP). 50+
concurrent requests on a 1GB droplet suggests either a traffic spike or a
goroutine leak (handlers hanging without returning). The connection pool has
a finite size; if all goroutines are waiting on DB connections, new requests queue.
**First things to check:**
- Is this legitimate traffic (a genuine spike)? Check request rate panel.
- Is it a goroutine leak? Look for handler code that can block indefinitely.
- If legitimate: consider rate limiting at Nginx level.

---

## What I didn't implement (and why)

**Postgres-specific metrics:** I'd want pg_stat_activity (active connections),
cache hit rate, slow query count. Requires the postgres_exporter sidecar container.
Out of scope for now but a natural W6 addition.

**Alertmanager:** Prometheus has a companion called Alertmanager that routes
alerts to PagerDuty, Slack, email, etc. I haven't set it up because it requires
external accounts and credentials. In a real setup, this is where the PromQL
conditions above would actually fire.

**Synthetic monitoring:** A separate process that hits /health every 30s from
outside the droplet (not from within it) to catch network/DNS issues. Services
like UptimeRobot do this for free. Would prevent Alert 3's false-positive issue.

**Log-based alerting:** My current setup has no log aggregation (Loki, ELK, etc.).
The server logs are in Docker's stdout buffer and vanish if the container restarts.
For a real service, I'd add Loki to the Grafana stack and alert on error log patterns.
