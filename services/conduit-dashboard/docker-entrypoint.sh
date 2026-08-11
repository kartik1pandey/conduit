#!/bin/sh
# Runs the migration once, then hands off to the Next.js standalone
# server — the same "migrate on startup, then serve" shape every Go
# service's main.go follows, just expressed as a shell entrypoint since
# the Next.js standalone output's own server.js is generated, not a place
# to hand-inject migration logic.
set -e
node scripts/migrate.mjs
exec node server.js
