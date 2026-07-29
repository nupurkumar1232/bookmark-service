# Breakage Experiments Log

For each: what I predicted, what actually happened, what the metrics showed,
what I'd do differently next time.

---

## W9 Chunk 2: Bottleneck fix + re-test (2026-07-29)

### Changes applied
- bcrypt cost 10 → 8 (~4x faster per hash: ~100ms → ~25ms)
- pgxpool MaxConns 4 → 20, MinConns 5

### Results comparison (all with rate limits raised for testing)

| Metric | Baseline | After fix | Change |
|---|---|---|---|
| p95 latency | 587ms | 506ms | -14% |
| Success rate | 92% | 93% | +1% |
| Signup success | 95% | 97% | +2% |
| Login success | 88% | 89% | +1% |
| Throughput | 15 req/s | 14.6 req/s | ~flat |
| Median latency | 120ms | 157ms | +30% (slightly worse) |

### Honest interpretation
The improvement was modest, not the 4x I predicted. bcrypt is A bottleneck
but not THE dominant one. Median actually got slightly worse — probably
because MinConns=5 means more warm connections held open, which isn't
free.

### Real bottleneck hypothesis
The service ceiling on this droplet appears to be ~15 req/s regardless
of bcrypt cost. Reasons:
- Single vCPU on $6/mo droplet
- Every signup + login requires bcrypt (still ~25ms each after fix)
- 30 concurrent VUs on 1 CPU means requests queue on CPU
- Nginx defaults to 1 worker process
- max latency of 1 second in this run shows a real tail

Fixing this properly would mean vertical scaling (bigger droplet) or
horizontal (multiple app instances behind a load balancer). Both are
outside the scope of a $6/month personal project.

### What I'd do next in a real production setting
1. Move signup/login to an async job queue (users wait longer but don't
   queue on CPU-bound tasks in the request path)
2. Rate limit at Nginx (which I already do — 5/min signup) so peak load
   never reaches the app
3. Vertical scale for a hard traffic peak
4. Add Postgres read replicas if reads dominate (they don't for us)

### Rate limits reverted to production values
signup: 5/min, general: 60/min per IP. This is the actual production
behavior. The load test used raised limits to observe internal
bottlenecks.



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

### Process I followed
- Created broken-deploy-test branch, committed `http.Error(... StatusInternalServerError)`
- Pushed branch, switched droplet to it, rebuilt — took ~108 minutes the first
  time because droplet ran out of RAM during Go compile and started swap-thrashing
- Added 2 GB swap on droplet — subsequent builds ~3 minutes
- Verified /bookmarks returned 500 with "kaboom" body
- Grafana error rate panel spiked to ~[X%] within ~30 seconds

### Rollback
- `git checkout main && docker compose up -d --build app` on droplet — ~3 min total
- Same /bookmarks request returned 200 again

### Lessons
1. The 1 GB droplet swaps under Go compile pressure. Permanent fix: 2 GB swap
   file at /swapfile. Build time went from ~108 min to ~3 min.
2. Manual rollback over SSH is slow. Even with everything working, the rebuild
   step blocks for 3+ minutes — during which production is broken. Real services
   need: blue/green or rolling deploys with health checks gating traffic.
3. The "broken branch" pattern worked — main never saw the broken commit, so
   `git checkout main` was an instant rollback in terms of code, just slow
   in terms of rebuild.
4. Detection was manual — I had to be watching Grafana. Real services need
   alerting (Alertmanager + PagerDuty/Slack) to page someone within seconds
   of error rate spiking.

### Implication for W7
The pain of manual deploy + manual rollback is the motivation for CI/CD.
With GitHub Actions:
- Push to main = auto-deploy
- Push that breaks tests = doesn't deploy
- Bad commit on main = revert commit + push = auto-rollback in ~3 min

## Experiment 4: Latency injection (2026-06-23)

### Result
- Latency change visible in Grafana within: [2 seconds]
- p95 went from [5.2ms] to [115ms]
- p99 (if I had it) would be more sensitive

### Reflection
The current dashboard alerts on >2s. 800ms wouldn't trigger. Should I lower
the threshold? Or add a per-endpoint baseline so 800ms on /health (which
should be <10ms) triggers but 800ms on /signup (where bcrypt takes ~100ms)
doesn't?
