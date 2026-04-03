# SignPost Upgrade Procedure

## Pre-Upgrade Checklist

1. **Backup your data** -- Download a full backup from the Dashboard (Backup button) or copy the data volume:
   ```bash
   docker compose -f docker-compose.prod.yml exec signpost \
     cp /data/signpost/signpost.db /data/signpost/backups/pre-upgrade-$(date +%Y%m%d).db
   ```

2. **Note your current version** -- Check the About page or:
   ```bash
   curl -u admin:yourpass http://localhost:8080/api/v1/status | jq .version
   ```

3. **Verify current state is healthy**:
   ```bash
   curl -f http://localhost:8080/api/v1/healthz
   echo QUIT | nc -w 2 localhost 25
   ```

4. **Read the release notes** for the target version to check for breaking changes.

## Upgrade Steps

### Docker Compose (recommended)

```bash
# 1. Pull the new image
docker compose -f docker-compose.prod.yml pull

# 2. Stop the current container
docker compose -f docker-compose.prod.yml down

# 3. Start with the new image
docker compose -f docker-compose.prod.yml up -d

# 4. Watch logs for startup issues
docker compose -f docker-compose.prod.yml logs -f --tail=50
```

### Manual Docker

```bash
# 1. Pull the new image
docker pull ghcr.io/drose12/signpost:latest

# 2. Stop and remove the old container
docker stop signpost && docker rm signpost

# 3. Start the new container (adjust flags to match your setup)
docker run -d --name signpost \
  -p 25:25 -p 587:587 -p 127.0.0.1:8080:8080 \
  -v signpost-data:/data/signpost \
  --env-file .env \
  --restart unless-stopped \
  ghcr.io/drose12/signpost:latest
```

## Post-Upgrade Verification

Run these checks after every upgrade:

1. **Health check** (HTTP API + SMTP):
   ```bash
   curl -f http://localhost:8080/api/v1/healthz
   echo QUIT | nc -w 2 localhost 25
   ```

2. **Check version**:
   ```bash
   curl -u admin:yourpass http://localhost:8080/api/v1/status | jq .version
   ```

3. **Verify domains are intact**:
   ```bash
   curl -u admin:yourpass http://localhost:8080/api/v1/domains | jq '.[].name'
   ```

4. **Send a test email** from the Dashboard or:
   ```bash
   curl -u admin:yourpass -X POST http://localhost:8080/api/v1/test/send \
     -H 'Content-Type: application/json' \
     -d '{"from":"test@yourdomain.com","to":"you@example.com","subject":"Upgrade test","body":"Testing after upgrade"}'
   ```

5. **Check DNS records** are still valid:
   ```bash
   curl -u admin:yourpass http://localhost:8080/api/v1/domains/1/dns/check | jq '.records[] | {type, status}'
   ```

## Rollback Procedure

If something goes wrong after upgrading:

### Quick Rollback (previous image)

```bash
# 1. Stop the broken container
docker compose -f docker-compose.prod.yml down

# 2. Pin to the previous version in docker-compose.prod.yml
#    Change: image: ghcr.io/drose12/signpost:latest
#    To:     image: ghcr.io/drose12/signpost:v0.1.0  (your previous version)

# 3. Start with the old image
docker compose -f docker-compose.prod.yml up -d
```

### Full Rollback (with data restore)

If the upgrade modified the database schema in a way that is incompatible with the old version:

```bash
# 1. Stop the container
docker compose -f docker-compose.prod.yml down

# 2. Restore the database backup
docker run --rm -v signpost-data:/data/signpost -v $(pwd):/backup alpine \
  cp /backup/pre-upgrade-backup.db /data/signpost/signpost.db

# 3. Pin to the previous version and start
docker compose -f docker-compose.prod.yml up -d
```

## Database Migrations

- Migrations run **automatically** on startup.
- Migrations are **forward-only** -- there is no automatic rollback of schema changes.
- The schema version is stored in the database and reported via `GET /api/v1/status` (`schema_version` field).
- If a migration fails, the application will not start. Check logs with:
  ```bash
  docker compose -f docker-compose.prod.yml logs signpost
  ```

## Version Compatibility

| SignPost Version | Schema Version | Maddy Version | Notes |
|:---:|:---:|:---:|---|
| 0.1.0 | 1 | 0.9.2 | Initial release -- core SMTP relay + DKIM |

As new versions are released, this table will be updated with compatibility information.

## Troubleshooting

### Container won't start after upgrade

1. Check logs: `docker compose -f docker-compose.prod.yml logs signpost`
2. Common cause: database migration failure. Restore from backup and report the issue.

### SMTP not accepting connections after upgrade

1. Verify Maddy is running: `curl http://localhost:8080/api/v1/status | jq .maddy_status`
2. Check if config generation succeeded: look for errors in logs.
3. Maddy config is regenerated on every startup from the database -- if the template changed in the new version, it should work automatically.

### Web UI shows old version

Clear your browser cache or hard-refresh (Ctrl+Shift+R). The frontend is embedded in the Go binary and served as static files.

### API returns 401 after upgrade

Your admin credentials are stored in environment variables (`.env` file), not in the database. Verify your `SIGNPOST_ADMIN_USER` and `SIGNPOST_ADMIN_PASS` are set correctly.
