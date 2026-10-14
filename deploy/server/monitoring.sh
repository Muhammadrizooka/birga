#!/usr/bin/env bash
# Logs stack for the Birga server: Loki stores logs (30 days), Alloy ships the API journal and
# nginx logs into it, Grafana shows them on port 3000 behind a login.
# Run as root from deploy/server (bootstrap.sh calls it). Safe to run again.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)/monitoring"
GRAFANA_ENV=/etc/birga/grafana.env
GRAFANA_PASSWORD_FILE=/root/grafana-admin-password

echo "Grafana apt repository"
install -d -m 755 /etc/apt/keyrings
if [[ ! -f /etc/apt/keyrings/grafana.gpg ]]; then
	curl -fsSL https://apt.grafana.com/gpg.key | gpg --dearmor -o /etc/apt/keyrings/grafana.gpg
fi
echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" \
	>/etc/apt/sources.list.d/grafana.list
apt-get -o DPkg::Lock::Timeout=600 update -q
apt-get -o DPkg::Lock::Timeout=600 install -y -q grafana loki alloy

echo "Loki"
install -m 644 "$HERE/loki.yaml" /etc/loki/config.yml
install -d -o loki -m 750 /var/lib/loki
systemctl enable loki >/dev/null
systemctl restart loki

echo "Alloy"
usermod -aG adm,systemd-journal alloy
install -m 644 "$HERE/config.alloy" /etc/alloy/config.alloy
systemctl enable alloy >/dev/null
systemctl restart alloy

echo "Grafana"
if [[ ! -f "$GRAFANA_PASSWORD_FILE" ]]; then
	umask 077
	openssl rand -base64 18 | tr -d '/+=' >"$GRAFANA_PASSWORD_FILE"
	umask 022
	echo "new Grafana admin password written to $GRAFANA_PASSWORD_FILE"
fi
install -m 600 /dev/null "$GRAFANA_ENV"
cat >"$GRAFANA_ENV" <<ENV
GF_SERVER_HTTP_PORT=3000
GF_SECURITY_ADMIN_USER=admin
GF_SECURITY_ADMIN_PASSWORD=$(cat "$GRAFANA_PASSWORD_FILE")
GF_USERS_ALLOW_SIGN_UP=false
GF_AUTH_ANONYMOUS_ENABLED=false
GF_ANALYTICS_REPORTING_ENABLED=false
GF_ANALYTICS_CHECK_FOR_UPDATES=false
GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/var/lib/grafana/dashboards/birga/birga-logs.json
ENV
install -d -m 755 /etc/systemd/system/grafana-server.service.d
cat >/etc/systemd/system/grafana-server.service.d/birga.conf <<UNIT
[Service]
EnvironmentFile=$GRAFANA_ENV
UNIT
install -m 644 "$HERE/grafana-datasource.yaml" /etc/grafana/provisioning/datasources/birga-loki.yaml
install -m 644 "$HERE/grafana-dashboards.yaml" /etc/grafana/provisioning/dashboards/birga.yaml
install -d -o grafana -g grafana -m 755 /var/lib/grafana/dashboards/birga
install -m 644 -o grafana -g grafana "$HERE/birga-logs.json" /var/lib/grafana/dashboards/birga/birga-logs.json
systemctl daemon-reload
systemctl enable grafana-server >/dev/null
systemctl restart grafana-server

# The admin password env var only applies when Grafana creates its database; keep it in step.
for _ in $(seq 1 30); do
	curl -fsS http://127.0.0.1:3000/api/health >/dev/null 2>&1 && break
	sleep 1
done
grafana cli --homepath /usr/share/grafana --config /etc/grafana/grafana.ini \
	admin reset-admin-password "$(cat "$GRAFANA_PASSWORD_FILE")" >/dev/null 2>&1 || true
chown -R grafana:grafana /var/lib/grafana

ufw allow 3000/tcp >/dev/null

echo "Checks"
for _ in $(seq 1 30); do
	curl -fsS http://127.0.0.1:3100/ready >/dev/null 2>&1 && break
	sleep 2
done
curl -fsS http://127.0.0.1:3100/ready && echo
curl -fsS http://127.0.0.1:3000/api/health | head -c 200 && echo
systemctl is-active loki alloy grafana-server
