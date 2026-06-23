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
