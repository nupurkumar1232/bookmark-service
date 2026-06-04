# Bookmark Service — Project Journal

A running log of work, decisions, things that broke, things I figured out.
Append-only. Newest entries at the top.

## W6 Day 1 — Thu, Jun 4, 2026 — Live on the public internet

Bookmark service is deployed to DigitalOcean droplet at 167.71.232.37:8080.
Containerized Go app + Postgres running via docker compose. Hardened SSH,
ufw, fail2ban. Verified end-to-end signup from my laptop hitting the
public IP.

### What worked
- Multi-stage Dockerfile, final image ~15 MB
- Internal Docker network for app <-> postgres
- Postgres healthcheck gating app startup
- JWT secret read from .env, never committed
- ufw only allows 22 and 8080 to the internet; postgres reachable
  only inside the Docker network

### What took longer than expected
- DigitalOcean payment friction (Indian debit card declined repeatedly,
  ~6 hrs lost, ultimately PayPal/family card resolved it)
- Spelling typos on path names (bookmark-serivce, etc.) and editing
  files in the wrong directory (nano main.go in $HOME instead of project)
- Accidentally pasted JWT secret in chat, had to rotate it

### What's next
- Nginx reverse proxy in front of the Go app (so port 80 hits 8080 internally)
- Domain from Namecheap (.me, free with Student Pack)
- Let's Encrypt SSL cert, automated renewal
- After all that: /health and /bookmarks reachable at https://<domain>

### W4-W5 status
W4 deliverable was "API live on a public HTTPS domain." Right now I have
HTTP, no HTTPS — about 80% of W4 done one week late. W5 (observability)
is untouched, deferred. Plan slipping by ~1 week net. Acceptable given
the payment-method friction was outside my control.

## Late W3 / Pre-W4 — Sun, May 24, 2026

Auth stack complete. End of W3 bookmark work.

### What I shipped
- Users table + foreign key + index on bookmarks.user_id
- /signup with bcrypt password hashing (cost 10)
- /login with constant-time hash comparison + JWT generation
- authMiddleware: extracts JWT, verifies signature, checks algorithm, puts user_id in context
- Per-user data scoping: Alice and Bob each see only their own bookmarks
- JWT secret in .env (not committed), 32-byte base64-random secret

### Where I got stuck
- Forgot `errors` import — caught by compile error pointing at exact line
- Pasted command placeholders (`<YOUR_JWT>`) literally without substituting twice
- Accidentally ran curl with -d but no -X POST — created a duplicate bookmark
  (curl implicitly switches to POST when -d is present)

### What I learned
- Postgres SERIAL doesn't reset on DELETE — IDs can have gaps, don't expect them contiguous
- JWT timing attacks: bcrypt comparison is slow, DB-miss is fast. Real production would
  do a dummy bcrypt even on user-not-found to equalize timing.
- The "alg confusion" attack on JWT — always check the signing algorithm in the
  verification callback, never trust the alg from the header alone.

### What's next (W4)
- Deploy to a DigitalOcean droplet (need to claim Student Pack credit if not active)
- Nginx reverse proxy + Let's Encrypt HTTPS
- Domain from Namecheap (.me, free with Student Pack)
- ALL of this is new territory — first real deployment
---
## Mid-W2 — Tue, May 12, 2026

API now wired to Postgres. Restart-survives-data test passed.

### What I built
- Postgres-backed `bookmarkStore` using pgx connection pool
- `INSERT ... RETURNING` for the POST handler (single round-trip, gets the auto-generated ID back)
- `SELECT ... ORDER BY created_at DESC` for the GET handler
- Proper error handling: DB failures return 500 to client, log the actual error server-side
- `req.Context()` passed to DB calls so client disconnects cancel in-flight queries
- Parameterized queries ($1, $2) — no SQL injection risk

### Where I got stuck
- Two separate nano paste failures when replacing the in-memory store with the
  Postgres version. First time, multi-line backtick strings got mangled
  ("newline in string" errors). Second time, the wipe didn't fully clear the
  file, so the new code merged with leftover old code and the `Bookmark` struct
  definition went missing.
- Fix: `rm main.go && touch main.go` to forcibly recreate, then verify with
  `wc -l` and `grep` BEFORE running. Saved another compile-error cycle.

### Lessons
- Verify before running. `grep` for key types/imports after pasting saves
  failed compile cycles.
- Read errors carefully — "newline in string" was a precise clue about
  what got corrupted; I should have spotted it faster.

### What's left for W2
- Pro Git ch. 1-3 deep practice (rebase, conflicts, reflog) — still half-done
- Missing Semester lectures 4-6 — partially done
- 2-page Raft design doc — not started
- ~3 days left in W2


## W1 Wrap-up — Thu, May 8, 2026

End of Week 1. Foundations week — no real product work yet, just setup.

### What I shipped this week
- DigitalOcean account created, GitHub Student Pack approved (waiting on 72-hour benefits cooldown)
- Go 1.26.2 installed on Ubuntu 24.04
- Bookmark service project initialized: `go mod init`, `main.go` hello-world, README, .gitignore
- Git repo set up locally, two clean commits on branch `master`
- Docker Engine installed (not Docker Desktop — native Linux Engine)
- `docker` group access set up so I don't need sudo for every command
- Postgres 16 running in a Docker container via docker-compose
- Verified end-to-end: `psql` connects to the containerized Postgres, runs queries
- First read of the Raft paper. Critical to do before W2 starts since W2 expects me to write a 2-page Raft design doc.

### Where I got stuck (and how I unstuck)
- **Docker group permissions.** After `usermod -aG docker $USER`, `docker run hello-world`
  still failed with "permission denied" on the docker.sock. Group membership only applies
  to new shell sessions — needed a full reboot to pick it up. Diagnostic: `getent group docker`
  shows whether your username is actually in the group at the system level vs. whether your
  current session knows about it.
- **psql connection typo.** Ran `psql -h localhost -U -d bookmarks` — forgot the username
  after `-U`. psql interpreted `-d` as the username and threw "password authentication
  failed for user '-d'". Fix: `psql -h localhost -U bookmark -d bookmarks`. Lesson: read
  the error message first — it literally tells you what username it tried.
- **psql pager confusion.** `SELECT version();` opened the result in a pager full of
  `~` lines and `(END)` at the bottom. I thought something was broken — turns out it's
  just a vim-style scrolling viewer for long output. Press `q` to exit. Or run
  `\pset pager off` to disable.

### Decisions I made (and why)
- **Go for the backend.** SRE-adjacent, easier to deploy than Java/Spring, growing in India.
  Stretches me in a way I won't get from Node or Python.
- **Postgres in Docker, not installed natively.** Same setup will work on the droplet later.
  Easy to nuke and reset. Doesn't pollute my laptop.
- **Branch named `master` not `main`.** Default from my Git install. Will rename when I push
  to GitHub since GitHub uses `main`. Not a big deal.
- **DigitalOcean over AWS / Hetzner.** Plan called for it, docs are best for beginners,
  Student Pack covers the first year.

### What I haven't done yet (carried into W2)
- Pro Git ch. 1-3 — only skimmed. Need to actually practice rebase, cherry-pick, reflog
  before the conflict-resolution scenarios in W2.
- Missing Semester lectures 1-2 — same, partially done.
- Droplet hardening (non-root user, ufw, fail2ban) — deferred to W4 by design, since I
  won't deploy until then.

### Hours spent
~5 hours focused work on Day 7 (Tue), spread across earlier days for reading.

### One honest reflection
I needed a lot of help today. Most of it was sequencing — knowing which command comes
next, which error means what. The understanding was mine; the workflow knowledge wasn't
yet. That's the gap I want to close over the next 10 weeks: not "knowing more facts"
but "being able to debug from cold by reading errors and running my own diagnostics."

### Goal for next entry (end of W2)
- API endpoints `POST /bookmarks` and `GET /bookmarks` working with curl, persisting to Postgres
- 2-page Raft design doc written
- Pro Git practice actually done (not just read about)
- Honest answer: did I need less help in W2 than W1?
