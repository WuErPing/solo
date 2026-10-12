#!/usr/bin/env bash
# report.sh — regenerate the GoAccess static report from the nginx access log.
# Installed at /opt/logstats/report.sh by setup.sh and run by cron every 10 min.
set -euo pipefail

OUT_DIR=/opt/logstats/goaccess
mkdir -p "$OUT_DIR"

# Write to a temp file and rename, so nginx never serves a half-written report.
# (GoAccess requires the -o filename to end in .html, hence the odd suffix.)
tmp="$OUT_DIR/.index.tmp.html"
goaccess /var/log/nginx/access.log --log-format=COMBINED -o "$tmp"
mv "$tmp" "$OUT_DIR/index.html"
