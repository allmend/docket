#!/bin/sh
# Start a Docket image against a throwaway Postgres and check that it
# migrates, reports ready and serves the login page. Usage: smoke.sh IMAGE
set -eu

IMAGE=${1:?usage: smoke.sh IMAGE}
NET=docket-smoke-$$
PG=$NET-pg
APP=$NET-app

cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then
		echo "--- docket logs ---"
		docker logs "$APP" 2>&1 | tail -50 || true
	fi
	docker rm -f "$APP" "$PG" >/dev/null 2>&1 || true
	docker network rm "$NET" >/dev/null 2>&1 || true
	exit "$status"
}
trap cleanup EXIT

docker network create "$NET" >/dev/null
docker run -d --name "$PG" --network "$NET" \
	-e POSTGRES_USER=docket -e POSTGRES_PASSWORD=docket -e POSTGRES_DB=docket \
	postgres:16-alpine >/dev/null

i=0
until docker exec "$PG" pg_isready -U docket -d docket >/dev/null 2>&1; do
	i=$((i + 1))
	[ "$i" -lt 30 ] || { echo "postgres did not start"; exit 1; }
	sleep 1
done

docker run -d --name "$APP" --network "$NET" -p 127.0.0.1::8081 -p 127.0.0.1::9413 \
	-e DATABASE_URL="postgres://docket:docket@$PG:5432/docket?sslmode=disable" \
	-e JWT_SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')" \
	-e SEED_PASSWORD="smoke-$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n')" \
	-e COOKIE_SECURE=false \
	"$IMAGE" >/dev/null

HTTP=$(docker port "$APP" 8081 | head -1)
HEALTH=$(docker port "$APP" 9413 | head -1)

i=0
until curl -fsS "http://$HEALTH/readyz" >/dev/null 2>&1; do
	i=$((i + 1))
	if [ "$i" -ge 60 ] || [ "$(docker inspect -f '{{.State.Running}}' "$APP")" != true ]; then
		echo "docket never became ready"
		exit 1
	fi
	sleep 1
done

curl -fsS "http://$HEALTH/livez" >/dev/null
curl -fsS "http://$HTTP/login" | grep -q '<form' || { echo "login page has no form"; exit 1; }
echo "smoke test passed: $IMAGE is ready and serves /login"
