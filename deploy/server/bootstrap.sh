#!/usr/bin/env bash
# Prepares a fresh Debian 13 server for Birga: PostgreSQL, Redis, nginx with HTTPS, the
# backend systemd service, the dashboard web root, a firewall, swap and nightly backups.
# Safe to run again: it keeps the existing database password, JWT secret and certificates.
#
# Run as root from this directory. The "Server setup" GitHub Actions workflow does that.
# Inputs (environment variables):
#   API_DOMAIN, ADMIN_DOMAIN  host names served by nginx (required)
#   SITE_DOMAIN               public website domain, e.g. birga-oila.com (optional; www. redirects to it)
#   CERT_EMAIL                Let's Encrypt account e-mail (required for HTTPS)
#   S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY  optional; written to the backend env file
#   PLAYMOBILE_USERNAME, PLAYMOBILE_PASSWORD, PLAYMOBILE_ORIGINATOR  optional; when the login and
#                             password are set they are written to the backend env file and
#                             SMS_PROVIDER becomes playmobile (real SMS)
set -euo pipefail

: "${API_DOMAIN:?API_DOMAIN is required}"
: "${ADMIN_DOMAIN:?ADMIN_DOMAIN is required}"
CERT_EMAIL="${CERT_EMAIL:-}"
SITE_DOMAIN="${SITE_DOMAIN:-}"

HERE="$(cd "$(dirname "$0")" && pwd)"
APP_USER=birga
APP_DIR=/opt/birga/backend
ENV_FILE=/etc/birga/backend.env
WEB_ROOT=/var/www/birga-dashboard
SITE_ROOT=/var/www/birga-website
BACKUP_DIR=/var/backups/birga
MIGRATE_VERSION=v4.18.3

log() { printf '\n==> %s\n' "$*"; }

log "Installing packages"
export DEBIAN_FRONTEND=noninteractive
apt-get -o DPkg::Lock::Timeout=600 update -q
apt-get -o DPkg::Lock::Timeout=600 install -y -q postgresql redis-server nginx certbot python3-certbot-nginx \
	ufw curl rsync openssl gnupg unattended-upgrades

log "Swap"
if ! swapon --show | grep -q .; then
	fallocate -l 2G /swapfile
	chmod 600 /swapfile
	mkswap /swapfile >/dev/null
	swapon /swapfile
	grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
	echo "2G swap file added"
else
	echo "swap already present"
fi

log "Firewall (SSH, HTTP, HTTPS only)"
ufw allow OpenSSH >/dev/null
ufw allow 'Nginx Full' >/dev/null
ufw --force enable >/dev/null
ufw status | head -n 1

log "Redis (local only)"
systemctl enable --now redis-server >/dev/null 2>&1 || systemctl enable --now redis >/dev/null
redis-cli ping

log "Service user and folders"
id "$APP_USER" >/dev/null 2>&1 || useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin "$APP_USER"
install -d -m 755 "$APP_DIR" "$APP_DIR/migrations" "$WEB_ROOT" "$SITE_ROOT"
install -d -m 750 -o root -g "$APP_USER" /etc/birga
install -d -m 700 -o postgres -g postgres "$BACKUP_DIR"

log "golang-migrate $MIGRATE_VERSION"
if ! command -v migrate >/dev/null || ! migrate -version 2>&1 | grep -q "${MIGRATE_VERSION#v}"; then
	case "$(uname -m)" in
	x86_64) arch=amd64 ;;
	aarch64) arch=arm64 ;;
	*) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
	esac
	tmp="$(mktemp -d)"
	curl -fsSL "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-${arch}.tar.gz" | tar -xz -C "$tmp"
	install -m 755 "$tmp/migrate" /usr/local/bin/migrate
	rm -rf "$tmp"
fi
migrate -version

# set_env KEY VALUE: sets KEY in the env file, replacing an existing line.
set_env() {
	local key="$1" value="$2"
	if grep -q "^${key}=" "$ENV_FILE"; then
		local tmp
		tmp="$(mktemp)"
		awk -v k="$key" -v v="$value" 'index($0, k "=") == 1 { print k "=" v; next } { print }' "$ENV_FILE" >"$tmp"
		cat "$tmp" >"$ENV_FILE"
		rm -f "$tmp"
	else
		printf '%s=%s\n' "$key" "$value" >>"$ENV_FILE"
	fi
}

log "PostgreSQL database and backend env file"
systemctl enable --now postgresql >/dev/null
if [[ ! -f "$ENV_FILE" ]]; then
	db_password="$(openssl rand -hex 24)"
	if runuser -u postgres -- psql -tAc "SELECT 1 FROM pg_roles WHERE rolname = 'birga'" | grep -q 1; then
		runuser -u postgres -- psql -q -c "ALTER ROLE birga WITH LOGIN PASSWORD '${db_password}'"
	else
		runuser -u postgres -- psql -q -c "CREATE ROLE birga WITH LOGIN PASSWORD '${db_password}'"
	fi
	runuser -u postgres -- psql -tAc "SELECT 1 FROM pg_database WHERE datname = 'birga'" | grep -q 1 ||
		runuser -u postgres -- createdb -O birga birga

	install -m 640 -o root -g "$APP_USER" /dev/null "$ENV_FILE"
	cat >"$ENV_FILE" <<EOF
# Birga backend settings on this server. Written by deploy/server/bootstrap.sh on first run;
# later runs keep it and only update the S3 and Play Mobile keys. Edit by hand, then: systemctl restart birga-backend
ENVIRONMENT=staging
LOG_LEVEL=info
HTTP_PORT=127.0.0.1:8080
METRICS_PORT=127.0.0.1:9090
ADMIN_API_KEY=

POSTGRES_URL=postgres://birga:${db_password}@127.0.0.1:5432/birga?sslmode=disable
REDIS_URL=redis://127.0.0.1:6379/0

# "log" writes OTP codes to the journal (journalctl -u birga-backend). Switch to playmobile
# with real credentials before going to production.
SMS_PROVIDER=log
OTP_DEFAULT_CODE=

JWT_SECRET=$(openssl rand -hex 32)

S3_BUCKET=birga-staging
S3_REGION=ap-southeast-2
S3_ACCESS_KEY_ID=
S3_SECRET_ACCESS_KEY=
EOF
	echo "created $ENV_FILE with a new database password and JWT secret"
else
	echo "$ENV_FILE exists, keeping it"
fi
if [[ -n "${S3_ACCESS_KEY_ID:-}" && -n "${S3_SECRET_ACCESS_KEY:-}" ]]; then
	set_env S3_ACCESS_KEY_ID "$S3_ACCESS_KEY_ID"
	set_env S3_SECRET_ACCESS_KEY "$S3_SECRET_ACCESS_KEY"
	echo "S3 keys written"
fi
if [[ -n "${PLAYMOBILE_USERNAME:-}" && -n "${PLAYMOBILE_PASSWORD:-}" ]]; then
	set_env PLAYMOBILE_USERNAME "$PLAYMOBILE_USERNAME"
	set_env PLAYMOBILE_PASSWORD "$PLAYMOBILE_PASSWORD"
	[[ -n "${PLAYMOBILE_ORIGINATOR:-}" ]] && set_env PLAYMOBILE_ORIGINATOR "$PLAYMOBILE_ORIGINATOR"
	set_env SMS_PROVIDER playmobile
	echo "Play Mobile keys written, SMS_PROVIDER=playmobile"
fi
chown root:"$APP_USER" "$ENV_FILE"
chmod 640 "$ENV_FILE"

log "Backend service"
install -m 644 "$HERE/birga-backend.service" /etc/systemd/system/birga-backend.service
systemctl daemon-reload
systemctl enable birga-backend >/dev/null
if [[ -x "$APP_DIR/birga_backend" ]]; then
	systemctl restart birga-backend
	echo "backend restarted"
else
	echo "no backend binary yet; the Deploy workflow installs it"
fi

log "Nightly database backup (03:30, keeps 14 days)"
install -m 755 "$HERE/backup.sh" /usr/local/bin/birga-backup
cat >/etc/cron.d/birga-backup <<EOF
30 3 * * * postgres /usr/local/bin/birga-backup $BACKUP_DIR
EOF

log "nginx"
[[ -f "$WEB_ROOT/index.html" ]] ||
	echo '<!doctype html><title>Birga</title><p>The dashboard has not been deployed yet.' >"$WEB_ROOT/index.html"
sed "s/{{API_DOMAIN}}/$API_DOMAIN/g" "$HERE/nginx-api.conf" >/etc/nginx/sites-available/birga-api.conf
sed "s/{{ADMIN_DOMAIN}}/$ADMIN_DOMAIN/g" "$HERE/nginx-admin.conf" >/etc/nginx/sites-available/birga-admin.conf
ln -sf /etc/nginx/sites-available/birga-api.conf /etc/nginx/sites-enabled/birga-api.conf
ln -sf /etc/nginx/sites-available/birga-admin.conf /etc/nginx/sites-enabled/birga-admin.conf
if [[ -n "$SITE_DOMAIN" ]]; then
	[[ -f "$SITE_ROOT/index.html" ]] ||
		echo '<!doctype html><title>Birga</title><p>The website has not been deployed yet.' >"$SITE_ROOT/index.html"
	sed "s/{{SITE_DOMAIN}}/$SITE_DOMAIN/g" "$HERE/nginx-site.conf" >/etc/nginx/sites-available/birga-site.conf
	ln -sf /etc/nginx/sites-available/birga-site.conf /etc/nginx/sites-enabled/birga-site.conf
fi
install -m 644 "$HERE/nginx-ip.conf" /etc/nginx/sites-available/birga-ip.conf
ln -sf /etc/nginx/sites-available/birga-ip.conf /etc/nginx/sites-enabled/birga-ip.conf
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx

log "Logs: Loki, Alloy, Grafana on port 3000"
bash "$HERE/monitoring.sh"

log "HTTPS certificates"
server_ip="$(curl -fsS -4 https://api.ipify.org || true)"
# One certificate per entry; a space-separated entry shares one certificate (site + www).
cert_sets=("$API_DOMAIN" "$ADMIN_DOMAIN")
if [[ -n "$SITE_DOMAIN" ]]; then cert_sets+=("$SITE_DOMAIN www.$SITE_DOMAIN"); fi
for names in "${cert_sets[@]}"; do
	read -ra domains <<<"$names"
	domain="${domains[0]}"
	resolved=""
	for d in "${domains[@]}"; do
		r="$(getent ahostsv4 "$d" | awk 'NR == 1 { print $1 }')"
		[[ "$r" == "$server_ip" ]] || { resolved="${r:-nothing}"; domain="$d"; break; }
		resolved="$r"
	done
	if [[ -z "$CERT_EMAIL" ]]; then
		echo "$domain: CERT_EMAIL not set, skipping HTTPS"
	elif [[ -z "$resolved" || "$resolved" != "$server_ip" ]]; then
		echo "$domain: DNS points to '${resolved:-nothing}', not this server ($server_ip); skipping HTTPS for now"
	else
		certbot --nginx --non-interactive --agree-tos --redirect -m "$CERT_EMAIL" \
			"${domains[@]/#/--domains=}" --keep-until-expiring --expand
	fi
done

log "Done"
systemctl is-active postgresql redis-server nginx || true
