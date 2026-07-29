# Building a real backend service from scratch: lessons from a summer

## Introduction

I'm a third-year CSE student. Over the summer I built a bookmark service in Go and deployed it to production. Not because bookmarks are exciting, but because deploying something small end-to-end teaches you more than reading a hundred blog posts about Kubernetes.

The service is live at [https://nupurkumar.me](https://nupurkumar.me). Code: [github.com/nupurkumar1232/bookmark-service](https://github.com/nupurkumar1232/bookmark-service).

## What I built

A Go HTTP API with:
- Email/password auth (JWT + bcrypt)
- Per-user bookmark scoping backed by Postgres
- Prometheus metrics + Grafana dashboards
- CI/CD via GitHub Actions
- HTTPS via Let's Encrypt with auto-renewal
- Nginx reverse proxy with rate limiting
- Runs in Docker on a $6/month DigitalOcean droplet

Standard stack. The interesting stuff isn't the stack — it's what I broke along the way.

## Five things that surprised me

### 1. Docker bypasses my firewall

I hardened SSH, set up ufw, felt smug. Then I ran `curl http://<droplet-ip>:8080/health` from my laptop, expecting a firewall rejection. It returned `ok`.

The reason: Docker manipulates iptables directly, and its rules sit in front of ufw's. Publishing a port with `ports: "8080:8080"` opens it to the internet regardless of what ufw says.

Fix: bind to a specific interface — `ports: "127.0.0.1:8080:8080"`. Now only Nginx (running on the same host) can reach the Go app. The internet has to go through Nginx.

This is the kind of thing that ships to production if nobody's checking. I only caught it because I was deliberately testing my assumptions.

### 2. bcrypt is the latency bottleneck for signup

At the default cost factor of 10, bcrypt takes ~100ms per hash on my droplet. Fine for one user at a time. Under load — I ran 200 concurrent signups via k6 — p95 latency on /signup went from ~100ms to ~8 seconds.

The math is straightforward: 1 CPU, 100ms per hash, 200 concurrent requests. They serialize. The 200th request waits for the previous 199 to finish.

Fix: cost 8 instead of 10 (~25ms per hash). Not exactly a rocket-science optimization, but the framing matters — I discovered this because I had monitoring showing the p95 climb, not because I read it somewhere.

### 3. Indian debit cards on foreign cloud signups

Half a day lost to DigitalOcean rejecting my card. Enabled international transactions in my bank app, waited, still rejected. Hit their rate limit, had to wait 24 hours.

This isn't unique to DigitalOcean or India. Anyone signing up for foreign cloud services from a country with strict international transaction rules will hit it. The workaround for me was PayPal.

Add ~1 day of buffer to any project involving foreign signups if you're outside the US/EU.

### 4. Diverged Git history from working on multiple machines

I worked on the laptop, then on the droplet, then back on the laptop without pulling. Ended up with commits on both machines that GitHub had never seen from either.

The fix was mechanical: `git fetch`, look at the divergence, `git reset --hard origin/main` on the laptop, cherry-pick just the commit I wanted to preserve, push clean. But I burned an hour debugging why my Prometheus /metrics endpoint returned 404 in production — it was because I never pushed the code that added the endpoint.

Lesson: `git pull` at the start of every session. Especially if you touched the other machine last.

### 5. /health returning 200 while the DB is dead is worse than useless

My original `/health` handler just wrote "ok" and returned 200. It said the service was healthy even when Postgres was unreachable and every real request was failing.

Load balancers depend on this endpoint to route traffic around broken instances. A false-positive /health means a broken instance keeps receiving traffic. Fix: /health now pings the database with a 2-second timeout, returns 503 if that fails.

Trivial change. Would have been catastrophic in a real production system I hadn't fixed it in.

## What I'd do differently

- **Monitoring from day one.** I added Prometheus + Grafana in week 5. If I'd had it in week 1, I would have seen bot scanner traffic immediately and understood earlier how much of the internet is just automated probing.
- **More tests.** I have one. In a real project, I'd have integration tests hitting a real Postgres, plus tests for the auth flow specifically.
- **A CDN in front for static assets.** Doesn't matter for this service, but would matter for anything with a frontend.

## What's still missing

- Frontend (this is API-only)
- Postgres-specific metrics
- Log aggregation
- Alertmanager for actual alerting (I have `alerts.md` documenting what I'd alert on, but nothing fires)

## Conclusion

The value of this project wasn't the code. It was the sequence of small failures — the firewall bypass, the DB pool exhaustion, the git divergence, the payment friction — and getting through each one. That's the muscle memory that makes you an engineer instead of someone who follows tutorials.

Code: [github.com/nupurkumar1232/bookmark-service](https://github.com/nupurkumar1232/bookmark-service)
Live: [https://nupurkumar.me](https://nupurkumar.me)
