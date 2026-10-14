# Operations

How the API is built, deployed and watched.

The live server is a Debian 13 machine set up and deployed from GitHub Actions (see
[Server](#server) and [GitHub Actions](#github-actions)). The Docker images and the Compose
stack described further down are the earlier GitLab-based plan; they are kept for local use
but do not run on the current server.

## Server

One Debian 13 server runs everything, installed from Debian packages rather than Docker:

| Part | Where |
|---|---|
| PostgreSQL (database `birga`, role `birga`) | local only, port 5432 |
| Redis | local only, port 6379 |
| Backend API | systemd service `birga-backend`, binary `/opt/birga/backend/birga_backend`, listens on `127.0.0.1:8080` (metrics `127.0.0.1:9090`) |
| Backend settings | `/etc/birga/backend.env` (root:birga, 640) |
| Dashboard | static files in `/var/www/birga-dashboard` |
| nginx | `api.<domain>` proxies to the API; `admin.<domain>` serves the dashboard and proxies `/v1/` to the API (same origin, no CORS); the bare IP serves the dashboard at `/` and the API under `/birga/` (Swagger: `http://<ip>/birga/swagger/index.html`) and `/v1/` for Swagger's "Try it out" |
| HTTPS | Let's Encrypt via certbot, renewed automatically |
| Firewall | ufw: SSH, HTTP and HTTPS only |
| Logs | Loki (local, 30 days) fed by Alloy from the API journal and nginx logs; Grafana on port 3000 (see [Logs](#logs)) |
| Backups | `pg_dump` nightly at 03:30 into `/var/backups/birga`, kept 14 days |

`deploy/server/bootstrap.sh` builds all of this and is safe to run again. On its first run it
creates `/etc/birga/backend.env` with a random database password and `JWT_SECRET`; later runs
keep that file and only update the S3 keys. The server starts as `ENVIRONMENT=staging` with
`SMS_PROVIDER=log`, so one-time codes appear in `journalctl -u birga-backend`; switch to
`playmobile` and `production` once the SMS contract is in place.

Useful commands on the server:

```sh
systemctl status birga-backend
journalctl -u birga-backend -f
nano /etc/birga/backend.env && systemctl restart birga-backend
```

## Logs

`deploy/server/monitoring.sh` (run by `bootstrap.sh`) installs Grafana, Loki and Alloy from
Grafana's apt repository:

- **Alloy** reads the `birga-backend` journal (labels `service="api"`, `level` from the JSON
  line) and `/var/log/nginx/{access,error}.log` (`service="nginx"`, `log="access|error"`) and
  pushes them to Loki.
- **Loki** listens on `127.0.0.1:3100` only and keeps 30 days.
- **Grafana** listens on port 3000 (`http://<server-ip>:3000`), user `admin`. Sign-up and
  anonymous access are off. The password is generated once on the server and kept in
  `/root/grafana-admin-password`; read it with
  `ssh root@<server> cat /root/grafana-admin-password`. The home dashboard *Birga logs*
  (folder Birga) shows log volume by level, API errors and warnings, all API lines and nginx
  lines, with a search box. In Explore, `{service="api"} | json | request_id="<id>"` follows
  one request.

Config files live in `deploy/server/monitoring/`. Grafana is plain HTTP on the IP until the
domains have HTTPS.

## GitHub Actions

Workflows in `.github/workflows/`:

| Workflow | When | What |
|---|---|---|
| `ci.yml` | every pull request and push to `main` | golangci-lint (non-blocking: v2.12.2 cannot analyse Go 1.27 yet); tests against PostgreSQL 17 after migrations |
| `ci.yml`, job `deploy` | push to `main` after tests pass (or run by hand on `main`) | builds the binary for the server's CPU, uploads it with `migrations/` and `seeds/`, then runs `deploy/server/release.sh`: migrate up, load `seeds/activities.sql` if `activities` is empty, swap the binary, restart, check `/health`, roll back to the previous binary if the check fails |
| `server-setup.yml` | by hand (Actions tab, inputs: API domain, dashboard domain, certificate e-mail) | uploads `deploy/server/` and runs `bootstrap.sh` |

The deploy job skips itself (with a notice) until the secrets exist and the server has been
set up. The dashboard repository deploys itself the same way into `/var/www/birga-dashboard`.

Repository secrets (Settings, Secrets and variables, Actions):

| Secret | Value |
|---|---|
| `SERVER_HOST` | server IP address |
| `SERVER_USER` | SSH user (`root`) |
| `SERVER_SSH_KEY` | private half of the deploy key; its public half is in the server's `authorized_keys` |
| `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | the `birga-backend-media` IAM key, the same values as in a local `.env`; written to the backend env file |

`deploy/ci/ssh-setup.sh` only connects when the server's ED25519 host key matches
`SERVER_HOST_FINGERPRINT` at the top of each workflow; update it there if the server is
reinstalled. This repository is public, so its Actions logs are too: workflows must never
print secrets or the env file.

## Images

The `Dockerfile` has three stages:

| Target | Image | Contents |
|---|---|---|
| `builder` | `golang:1.27.1-alpine` | compiles a static binary |
| `migrations` | `migrate/migrate:v4.20.1` | `migrations/` baked in; entrypoint `migrate -path /migrations` |
| `runtime` (default) | `alpine:3.22` | the binary, CA certs, tzdata, `TZ=Asia/Tashkent`, non-root user `app`; exposes 8080 and 9090 |

CI pushes both on the default branch:

- `registry.gitlab.com/loyihalar/birga/backend:<short-sha>` and `:latest`
- `registry.gitlab.com/loyihalar/birga/backend/migrations:<short-sha>` and `:latest`

Servers only pull images; they never need the source tree.

## Deploying

In the devops folder on the server, set `BACKEND_TAG` in `.env` (a commit SHA from CI, or
`latest`) and run `make deploy`. It pulls the images, runs migrations, then restarts only the
API. Migrations must therefore be backward compatible with the API version still running
during the rollout.

The API refuses to start without `JWT_SECRET` (at least 32 bytes). Set a generated value
(`openssl rand -hex 32`) in the server's `.env` before deploying a version with
`/v1/auth/*`, and keep it stable: changing it invalidates every issued token.

## Compose stack (devops)

`COMPOSE_FILE` in the devops `.env` picks which layers run:

| File | Services |
|---|---|
| `compose.yaml` | `postgres`, `migrate`, `api` |
| `compose.dev.yaml` | builds from `../backend`, publishes ports, Adminer |
| `compose.proxy.yaml` | Caddy with automatic HTTPS |
| `compose.monitoring.yaml` | Prometheus, Alertmanager (Telegram), Grafana, Loki, Alloy, node/cAdvisor/postgres exporters |
| `compose.analytics.yaml` | Metabase |
| `compose.backup.yaml` | scheduled `pg_dump` with daily/weekly/monthly rotation |

## Observability

**Logs.** JSON on stdout. One access line per request; every line carries `request_id`.
On servers, Alloy ships container logs to Loki. In Grafana Explore:
`{service="api"} | json | request_id="<id>"`.

**Metrics.** `METRICS_PORT` (default 9090) serves `/metrics`, kept off the public port:

| Metric | Labels |
|---|---|
| `birga_http_server_request_duration_seconds` | `method`, `route`, `status`, `code` (application error code) |
| `birga_http_server_requests_in_flight` | `method` |
| `birga_http_client_request_duration_seconds` | `service`, `method`, `status` |
| `birga_db_query_duration_seconds` | `operation`, `status` |
| `birga_db_pool_*` | pool gauges and counters |

Go runtime and process metrics are included. `/ping`, `/health` and `/swagger/*` are excluded
from access logs and HTTP metrics.

**Dashboards and alerts.** The provisioned Grafana dashboard *Birga API* covers traffic,
error codes, latency, database, runtime and host. Alert rules (`devops/config/prometheus/rules/birga.yml`)
fire on API down, 5xx above 5%, p95 above 1s, DB pool and connection pressure, disk, memory
and container restarts, and go to Telegram when a bot token and chat id are configured.

**Probes.** `GET /ping` for liveness, `GET /health` for readiness (fails with 503 when the
database or Redis is unreachable).

**Redis.** The API needs Redis (`REDIS_URL`) for one-time codes and rate limits. The backend
`docker-compose.yml` runs one; the devops stack does not include it yet, so add a Redis
service there (no persistence or backups needed) before deploying this version.

**S3.** Profile photos go to the bucket in `S3_BUCKET` (see
[setup.md](setup.md#media-s3-locally) for the IAM permissions, and
[The Birga AWS setup](setup.md#the-birga-aws-setup) for the bucket `birga-staging` in
`ap-southeast-2` and the backend's IAM user). Set the same `S3_*` values in the server's
environment before deploying. Without
it, uploads answer 503 and the rest of the API works. S3 calls show up in
`birga_http_client_request_duration_seconds{service="s3"}`. Turn on bucket versioning or
a lifecycle rule if old photos should be kept or expired; the API never deletes a photo
that a media row points to.

## Backups

The backup service dumps the `birga` and `metabase` databases on a schedule into
`devops/backups/` (`last/`, `daily/`, `weekly/`, `monthly/`). `make backup` runs one now,
`make restore f=...` restores (the API is stopped meanwhile). Copy backups off the server
regularly and test restores.
