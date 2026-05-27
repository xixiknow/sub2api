# Terra Deployment Runbook

This note records the 2026-05-13 deployment of the `feature/terra-dulupay-affiliate` branch and the recommended process for the next deployment.

## Deployment Summary

- Target app container: `sub2api-terra`
- Compose file in use: `/root/sub2api/deploy/docker-compose.local.yml`
- Effective host binding: `127.0.0.1:8090 -> 8080/tcp`
- Data mount: `/root/sub2api/deploy/data -> /app/data`
- Docker network: `deploy_sub2api-network`
- Image tag used by the existing config: `sub2api:feature-terra-dulupay-affiliate-local`
- Updated source revision: `725c43576ca080a749ca321a759fb2b2b1ee5e07`
- New image ID: `sha256:13810eafb80cdf7314dbb27f583c0d67e756ce29d9a8a595cf2711332ed3e1c4`
- Preserved previous container: `sub2api-terra-rollback-20260513163346`
- Preserved previous image tag: `sub2api:feature-terra-dulupay-affiliate-local-rollback-20260513163346`
- Database backup made before switching: `/tmp/sub2api-postgres-before-725c4357-20260513163312.sql.gz`

Important: `deploy/` was treated as read-only during this deployment. Its config files were checksummed before and after the update, and they matched.

## What Was Special This Time

The `.git/` directory was mounted read-only, so `git fetch` / `git pull` could not update local Git metadata. The workaround was:

1. Clone the target branch into `/tmp`.
2. Copy remote source changes into the working tree.
3. Explicitly exclude `deploy/`.
4. Build the Docker image from the updated working tree.

The application container was not recreated through `docker compose up -d` because that would remove/recreate the original `sub2api-terra` container. To preserve rollback capability, the old container was stopped and renamed, then a new `sub2api-terra` was started with the same effective runtime settings.

## Next Deployment Checklist

1. Confirm current state.

```bash
cd /root/sub2api
git status --short --branch
docker ps -a --no-trunc --filter name=sub2api-terra \
  --format '{{.Names}}\t{{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker inspect sub2api-terra --format 'Image={{.Config.Image}}
ImageID={{.Image}}
Restart={{.HostConfig.RestartPolicy.Name}}
Network={{.HostConfig.NetworkMode}}
Ports={{json .HostConfig.PortBindings}}
Mounts={{json .Mounts}}'
```

2. Record checksums for deployment config files. Do not hash database/data directories.

```bash
cd /root/sub2api
sha256sum \
  deploy/.env \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.yml \
  deploy/docker-compose.standalone.yml \
  deploy/.env.example \
  > /tmp/sub2api-deploy-config-before.sha256
```

3. Get the latest remote source.

If `.git/` is writable, use normal Git:

```bash
cd /root/sub2api
git fetch --all --prune
git pull --ff-only
```

If `.git/` is read-only again, use a temporary clone and exclude `deploy/`:

```bash
cd /root/sub2api
BRANCH=feature/terra-dulupay-affiliate
REMOTE_DIR=/tmp/sub2api-remote-update-$(date -u +%Y%m%d%H%M%S)

git clone --branch "$BRANCH" https://github.com/xixiknow/sub2api.git "$REMOTE_DIR"
rsync -a --delete \
  --exclude '.git/' \
  --exclude 'deploy/' \
  "$REMOTE_DIR"/ /root/sub2api/
```

Before using the `rsync --delete` path, check that there are no local-only files outside `deploy/` that must be preserved.

4. Verify `deploy/` config files were not changed.

```bash
cd /root/sub2api
sha256sum -c /tmp/sub2api-deploy-config-before.sha256
```

5. Back up PostgreSQL before starting the new app image.

```bash
BACKUP=/tmp/sub2api-postgres-before-$(date -u +%Y%m%d%H%M%S).sql.gz
docker exec sub2api-postgres pg_dump -U sub2api sub2api | gzip > "$BACKUP"
ls -lh "$BACKUP"
```

6. Preserve the current image, then build the new image.

```bash
cd /root/sub2api
STAMP=$(date -u +%Y%m%d%H%M%S)
COMMIT=$(git rev-parse HEAD 2>/dev/null || git -C "$REMOTE_DIR" rev-parse HEAD)

docker tag \
  sub2api:feature-terra-dulupay-affiliate-local \
  sub2api:feature-terra-dulupay-affiliate-local-rollback-$STAMP

mkdir -p /tmp/sub2api-docker-config
env DOCKER_CONFIG=/tmp/sub2api-docker-config docker build \
  -t sub2api:feature-terra-dulupay-affiliate-local \
  -t sub2api:feature-terra-dulupay-affiliate-local-${COMMIT:0:8} \
  --build-arg COMMIT="$COMMIT" \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  -f Dockerfile .
```

`DOCKER_CONFIG=/tmp/sub2api-docker-config` avoids failures when `/root/.docker` is read-only.

7. Switch containers while preserving the previous container.

```bash
cd /root/sub2api
STAMP=$(date -u +%Y%m%d%H%M%S)
OLD_NAME=sub2api-terra
ROLLBACK_NAME=sub2api-terra-rollback-$STAMP
ENV_FILE=/tmp/sub2api-terra-env-$STAMP.env

docker inspect "$OLD_NAME" --format '{{range .Config.Env}}{{println .}}{{end}}' > "$ENV_FILE"

docker stop --timeout 30 "$OLD_NAME"
docker rename "$OLD_NAME" "$ROLLBACK_NAME"

docker run -d \
  --name "$OLD_NAME" \
  --restart unless-stopped \
  --ulimit nofile=100000:100000 \
  --network deploy_sub2api-network \
  -p 127.0.0.1:8090:8080 \
  -v /root/sub2api/deploy/data:/app/data:rw \
  --env-file "$ENV_FILE" \
  --health-cmd 'wget -q -T 5 -O /dev/null http://localhost:8080/health' \
  --health-interval 30s \
  --health-timeout 10s \
  --health-retries 3 \
  --health-start-period 30s \
  sub2api:feature-terra-dulupay-affiliate-local

rm -f "$ENV_FILE"
```

The temporary env file contains secrets. Delete it immediately after the new container starts.

8. Verify health, logs, migrations, and HTTP response.

```bash
curl -fsS -m 5 http://127.0.0.1:8090/health
curl -fsS -m 5 -I http://127.0.0.1:8090/ | sed -n '1,5p'

docker inspect sub2api-terra --format 'Status={{.State.Status}} Health={{if .State.Health}}{{.State.Health.Status}}{{end}} ImageID={{.Image}}'
docker logs --since=3m sub2api-terra 2>&1 | rg '\t(ERROR|FATAL|PANIC)\t|"level":"(error|fatal|panic)"|panic:' || true

docker exec sub2api-postgres psql -U sub2api -d sub2api -Atc \
  "SELECT filename FROM schema_migrations ORDER BY filename DESC LIMIT 10;"
```

## Rollback

Use rollback only after deciding whether database migrations from the new version are compatible with the old binary. If the new deployment applied irreversible schema changes, restore the PostgreSQL backup before starting the old container.

Fast container rollback:

```bash
docker stop --timeout 30 sub2api-terra
docker rename sub2api-terra sub2api-terra-failed-$(date -u +%Y%m%d%H%M%S)
docker rename sub2api-terra-rollback-20260513163346 sub2api-terra
docker start sub2api-terra
curl -fsS -m 5 http://127.0.0.1:8090/health
```

Image rollback tag from this deployment:

```bash
docker tag \
  sub2api:feature-terra-dulupay-affiliate-local-rollback-20260513163346 \
  sub2api:feature-terra-dulupay-affiliate-local
```

Database restore pattern:

```bash
gunzip -c /tmp/sub2api-postgres-before-725c4357-20260513163312.sql.gz | \
  docker exec -i sub2api-postgres psql -U sub2api -d sub2api
```

Prefer testing restore commands in a throwaway database first if time allows.

## Operational Notes

- Do not commit or copy secrets from `deploy/.env` into notes, logs, or tickets.
- Do not modify files under `deploy/` unless that is the explicit deployment task.
- Do not remove old containers during deployment. Rename them so rollback is a container rename/start operation.
- Keep at least one image rollback tag and one database dump per deployment.
- The current new `sub2api-terra` was started with `docker run` to preserve the old container. Because of that, `docker compose ps` may show the preserved rollback container as the compose service rather than the current container. Use `docker ps --filter name=sub2api-terra` and `docker inspect sub2api-terra` for the authoritative runtime state.
