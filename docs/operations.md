# Operations: upgrades, backups and recovery

Docket migrates its own database at startup, so upgrading is "run the new
image". This guide covers what happens underneath, how to upgrade safely, and
what to do when an upgrade goes wrong.

## How startup migration works

Every Docket process applies pending migrations before it serves anything, in
every `MODE`. Migrations are the SQL files in `migrations/`, baked into the image.

- **Safe with many replicas.** Migrations run under a Postgres advisory lock:
  when several pods start at once, one migrates and the others wait, then find
  nothing left to do.
- **Readiness waits for it.** `/readyz` fails until migrations finished, so a
  pod receives no traffic before its schema is in place. The example
  Deployment's startup probe allows 60 seconds.
- **A failed migration stops the process.** It logs `migrations` with the error
  and exits non-zero, so the pod crash-loops instead of serving against a
  half-migrated schema.
- **Migrate without serving** with `--migrate-only`: it applies migrations and
  exits. Use it from an init container or a Job if you want migration as a
  separate, visible step:

  ```yaml
  initContainers:
    - name: migrate
      image: ghcr.io/allmend/docket:0.19.1   # same tag as the app container
      args: ["--migrate-only"]
      envFrom:
        - secretRef:
            name: docket
  ```

  The app container still checks on startup and finds nothing to do.

## Strategy: roll forward

A bad migration is fixed by a **new release with a new migration**, never by
rolling the database back. Every Docket migration follows three rules, which is
what makes the upgrade and rollback advice below hold:

1. **A shipped migration is never edited.** Databases that already ran it would
   never run the new version, and installs would quietly diverge.
2. **A fix works on both kinds of database**: ones that ran the broken
   migration, and fresh installs that run it followed by the fix.
3. **Each migration stays compatible with the previous release's code.** During
   a rolling update old pods keep serving against the new schema. Destructive
   changes (dropping or renaming a column) are split across two releases: add
   the new shape, move the code over, remove the old shape later.

## Upgrading

1. **Read the [CHANGELOG](../CHANGELOG.md)** for every release between yours and
   the target. Required config changes are listed at the top of a release.
2. **Back up the database** (see below). Migrations are tested, but a backup is
   the only undo for a migration that destroys data.
3. **Pin the version.** Use an exact tag (`ghcr.io/allmend/docket:0.19.1`), not
   `latest`. Tags available:

   | Tag | Moves? | Use |
   |---|---|---|
   | `0.19.0` | never | production: you choose when to upgrade |
   | `0.19` | to each `0.19.x` patch | patches only, automatically |
   | `latest` | to every release | trying Docket out |

4. **Apply and watch the rollout:**

   ```bash
   kubectl -n docket set image deploy/docket docket=ghcr.io/allmend/docket:0.19.1
   kubectl -n docket rollout status deploy/docket
   kubectl -n docket logs deploy/docket | grep -E '"msg":"(starting docket|migrations)'
   ```

   `starting docket` carries the version, and `migrations ok` means the schema
   is current. The version is also in the page footer.

You can skip releases: going from 0.15.0 to 0.19.0 in one step applies every
migration in between, in order.

## Rolling back

**Image-only rollback works** when the release you return to is the previous
one (or later than 0.16.0). An older image that finds a newer schema logs
`database schema is newer than this build; starting anyway` and serves, which
is safe because of rule 3 above. Releases up to and including 0.15.0 lack this
check and crash-loop against a newer schema.

```bash
kubectl -n docket rollout undo deploy/docket
```

The schema stays at the newer version. Upgrading again later just finds nothing
to migrate.

**Restore a backup** when an image rollback isn't enough: a migration destroyed
data, or you are going back further than one release across a destructive
change. Everything written after the backup is lost.

## Backups

All of Docket's state is in Postgres; the pods hold nothing. Back up the
database and you have backed up Docket.

```bash
# Dump (custom format: compressed, restorable selectively)
pg_dump --format=custom --no-owner "$DATABASE_URL" > docket-$(date +%F).dump

# Restore into an empty database, with Docket scaled to zero
kubectl -n docket scale deploy/docket --replicas=0
pg_restore --no-owner --dbname "$DATABASE_URL" docket-2026-10-21.dump
kubectl -n docket scale deploy/docket --replicas=2
```

Restore with the image that wrote the backup, or any newer one; a newer image
migrates the restored database forward on startup. If you run Postgres with an
operator such as CloudNativePG, its scheduled backups and point-in-time
recovery cover this, and `pg_dump` before an upgrade is still a cheap extra.

## Recovering a dirty database

golang-migrate marks the database **dirty** when a migration fails partway. A
dirty database is never migrated further, and Docket refuses to start on it:

```
{"level":"ERROR","msg":"migrations","err":"Dirty database version 27. Fix and force version."}
```

Postgres runs each migration file in a transaction, so a failed migration
usually left nothing behind. Recover like this:

1. **Find the failed version.** It is in the error, or:

   ```sql
   SELECT version, dirty FROM schema_migrations;
   ```

2. **Check what the migration did.** Read
   `migrations/<version>_*.up.sql` from the release you were installing
   (`git show v0.19.0:migrations/` lists them) and compare it with the
   database. Fix the cause of the failure, such as a duplicate row that blocked
   a new unique index.

3. **Mark the previous version clean**, so the failed migration runs again. If
   version 27 failed and changed nothing:

   ```sql
   UPDATE schema_migrations SET version = 26, dirty = false;
   ```

   If it did complete but was still marked dirty, set `version = 27` instead.
   This is what `migrate force <version>` does; with the
   [migrate CLI](https://github.com/golang-migrate/migrate) and the release's
   own `migrations/` directory:

   ```bash
   migrate -path migrations -database "$DATABASE_URL" force 26
   ```

4. **Start Docket again.** It retries the migration. If it fails the same way,
   [open an issue](https://github.com/allmend/docket/issues) with the log line;
   the fix will be a new release with a new migration, per the strategy above.
