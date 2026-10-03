#!/bin/sh
# pnpm 12's registry entry is intentionally shebang-less. Go's exec does not
# perform the shell fallback, so provide a real executable launcher.
exec /usr/local/bin/node /opt/pnpm/bin/pnpm.mjs "$@"
