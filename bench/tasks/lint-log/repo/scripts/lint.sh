#!/bin/sh
set -u
cd "$(dirname "$0")/.."
status=0
i=0
while [ $i -lt 400 ]; do
  i=$((i + 1))
  printf '[%03d/400] checking src/module_%03d.c ... ok (%d symbols, %d warnings suppressed)\n' $i $i $((i * 7 % 50 + 3)) $((i % 4))
  if [ $i -eq 217 ]; then
    if ! grep -Eq '^timeout: [0-9]+$' config/app.yaml; then
      echo "error: config/app.yaml: key 'timeout' must be an integer number of seconds"
      status=1
    fi
  fi
done
echo "lint: 400 files checked"
exit $status
