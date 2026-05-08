# Bookmark Service — Project Journal

A running log of work, decisions, things that broke, things I figured out.
Append-only. Newest entries at the top.

---

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
- First read of the Raft paper. Critical to do before W2 starts since W2 expects me to
  write a 2-page Raft design doc.
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
