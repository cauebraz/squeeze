#!/bin/sh
status=0
for f in "$@"; do
  if grep -q 'TODO' "$f"; then
    grep -n 'TODO' "$f" | sed "s|^|$f:|"
    status=1
  fi
done
exit $status
