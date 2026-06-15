#!/bin/sh
# postinstall script for nfpm packages
set -e

case "$1" in
configure)
  # Enable and start on systemd systems
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
    systemctl enable pxego || true
    systemctl start pxego || true
  fi
  ;;
esac
