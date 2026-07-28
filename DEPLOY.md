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


## Nginx config (lives on droplet, not in git)

### /etc/nginx/conf.d/rate-limit.conf
limit_req_zone $binary_remote_addr zone=signup_limit:10m rate=5r/m;
limit_req_zone $binary_remote_addr zone=general_limit:10m rate=60r/m;

### /etc/nginx/sites-available/bookmark
Both server blocks (port 80 and 443) need location = /signup and location /
with respective limit_req directives.

## Rate limits (per IP)
- /signup: 5/min (bcrypt-expensive, anti-spam)
- everything else: 60/min

If droplet rebuilt, recreate from this doc.
