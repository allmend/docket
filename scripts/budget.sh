#!/bin/sh
# Fails when the gzipped static assets every page loads exceed the budget.
# The design target is 47 KB; the budget holds today's size plus a little
# headroom so growth fails CI, and is lowered as assets shrink.
set -eu
BUDGET=${BUDGET:-86016} # 84 KiB
total=0
printf '%10s  %s\n' 'gzip B' 'file'
for f in static/dist/*.js static/dist/*.css; do
	n=$(gzip -9 -c "$f" | wc -c | tr -d ' ')
	total=$((total + n))
	printf '%10d  %s\n' "$n" "$f"
done
printf '%10d  total (budget %d)\n' "$total" "$BUDGET"
if [ "$total" -gt "$BUDGET" ]; then
	echo "asset budget exceeded by $((total - BUDGET)) bytes" >&2
	exit 1
fi
