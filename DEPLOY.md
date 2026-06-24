# Deployment

Production runs on DigitalOcean droplet 167.71.232.37, reachable at https://nupurkumar.me.

## Automatic
Every push to `main` triggers GitHub Actions which:
1. Runs Go tests against a fresh Postgres container
2. Builds the Docker image
3. SSHes to the droplet and rebuilds the `app` container
4. Verifies /health responds

## Manual (emergency rollback)
```bash
ssh nk@167.71.232.37
cd ~/bookmark-service
git log --oneline -10        # find a good commit
git checkout <commit-sha>
docker compose up -d --build app
```

## Secrets used
- DROPLET_HOST, DROPLET_USER, DROPLET_SSH_KEY in GitHub repo secrets
- JWT_SECRET, GRAFANA_PASSWORD in .env on droplet (not in git)
