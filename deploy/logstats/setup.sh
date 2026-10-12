#!/usr/bin/env bash
# setup.sh — one-shot bootstrap for log statistics on the solo-relay host (Ubuntu).
#
# Usage:
#   scp -r deploy/logstats <host>:/tmp/logstats
#   ssh <host>
#   sudo bash /tmp/logstats/setup.sh
#
# What it does (idempotent — safe to re-run):
#   1. apt-get install goaccess, apache2-utils (htpasswd), unzip, logrotate
#   2. Install the duckdb single binary into /usr/local/bin (GitHub latest
#      release; skipped if duckdb is already on PATH)
#   3. mkdir -p /opt/logstats/goaccess and install report.sh there
#   4. Create the nginx basic-auth file /etc/nginx/.htpasswd-logstats
#      (interactive: asks for a username, htpasswd prompts for the password)
#   5. Install cron: run report.sh every 10 minutes
#   6. Install logrotate config for /var/log/solo-relay/access.log
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATS_DIR=/opt/logstats
HTPASSWD=/etc/nginx/.htpasswd-logstats

if [ "$(id -u)" -ne 0 ]; then
    echo "Please run as root: sudo bash $0" >&2
    exit 1
fi

echo "==> Installing packages (goaccess, apache2-utils, unzip, logrotate)"
apt-get update -qq
apt-get install -y goaccess apache2-utils unzip logrotate

echo "==> Installing duckdb"
if command -v duckdb >/dev/null 2>&1; then
    echo "    duckdb already installed: $(duckdb --version), skipping"
else
    arch="$(uname -m)"
    case "$arch" in
        x86_64)  duck_arch=amd64 ;;
        aarch64) duck_arch=aarch64 ;;
        *) echo "    unsupported arch: $arch, skipping duckdb install" >&2; duck_arch="" ;;
    esac
    if [ -n "$duck_arch" ]; then
        tmp="$(mktemp -d)"
        if curl -fsSL "https://github.com/duckdb/duckdb/releases/latest/download/duckdb_cli-linux-${duck_arch}.zip" -o "$tmp/duckdb.zip" \
            && unzip -o -q "$tmp/duckdb.zip" -d "$tmp"; then
            install -m 0755 "$tmp/duckdb" /usr/local/bin/duckdb
            echo "    installed: $(duckdb --version)"
        else
            # duckdb is only used for ad-hoc analysis; a failed download must
            # not abort the rest of the setup.
            echo "    WARNING: duckdb download failed, skipping (install it manually later)" >&2
        fi
        rm -rf "$tmp"
    fi
fi

echo "==> Creating $STATS_DIR/goaccess and installing report.sh"
mkdir -p "$STATS_DIR/goaccess"
install -m 0755 "$SCRIPT_DIR/report.sh" "$STATS_DIR/report.sh"

echo "==> Configuring nginx basic auth ($HTPASSWD)"
if [ -f "$HTPASSWD" ]; then
    echo "    $HTPASSWD exists, skipping (add users with: htpasswd -B $HTPASSWD <user>)"
else
    read -r -p "    username for /stats/ basic auth: " stats_user
    htpasswd -c "$HTPASSWD" "$stats_user"
fi

echo "==> Installing cron job (/etc/cron.d/logstats)"
cat > /etc/cron.d/logstats <<'EOF'
# Regenerate the GoAccess report every 10 minutes.
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
*/10 * * * * root /opt/logstats/report.sh >/dev/null 2>&1
EOF

echo "==> Installing logrotate config (/etc/logrotate.d/solo-relay-access)"
install -m 0644 "$SCRIPT_DIR/logrotate.d/solo-relay-access" /etc/logrotate.d/solo-relay-access

# Pre-create the log dir so logrotate/DuckDB work before the relay service is
# restarted with the updated unit (systemd LogsDirectory= would create it too).
if id solo-relay >/dev/null 2>&1; then
    install -d -o solo-relay -g solo-relay -m 0755 /var/log/solo-relay
fi

echo
echo "Done. Remaining manual steps:"
echo "  1. Deploy the new solo-relay binary and the updated systemd unit"
echo "     (adds LogsDirectory=solo-relay) + nginx config (adds /stats/)."
echo "  2. Add ACCESS_LOG_PATH=/var/log/solo-relay/access.log to"
echo "     /opt/solo-relay/solo-relay.env"
echo "  3. systemctl daemon-reload && systemctl restart solo-relay && nginx -t && systemctl reload nginx"
echo "  4. Run $STATS_DIR/report.sh once, then open https://<host>/stats/"
