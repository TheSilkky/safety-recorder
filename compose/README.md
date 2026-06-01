# Compose Smoke Tests

This directory contains local Docker Compose stacks for exercising Proofline
Server backend combinations before release work.

These stacks are for local smoke testing only. They use fixed local test
credentials, publish the main and private-admin listeners on loopback by
default, and do not make Proofline production-ready public infrastructure.

The smoke script starts the server with a local bootstrap secret, creates a
temporary admin account over the private-admin loopback listener, and runs the
simulator against the main API/viewer listener with that account. The default
bootstrap secret and password are placeholders for local throwaway smoke volumes
only. The script waits for token-neutral `/admin/static/styles.css` on the
private-admin loopback port before bootstrapping the test account through
`POST /admin/bootstrap`.

## Variants

| Variant | File | Metadata | Blob storage | Coordination |
|---|---|---|---|---|
| `full` | `compose-full.yml` | PostgreSQL | MinIO S3-compatible bucket | Valkey |
| `sqlite-local` | `compose-sqlite-local.yml` | SQLite | Local filesystem | none |
| `postgresql-local` | `compose-postgresql-local.yml` | PostgreSQL | Local filesystem | none |
| `sqlite-s3` | `compose-sqlite-s3.yml` | SQLite | MinIO S3-compatible bucket | none |

Run the default full-stack smoke test from the repository root:

```bash
compose/smoke-test.sh
```

Run a specific variant:

```bash
compose/smoke-test.sh sqlite-local
compose/smoke-test.sh postgresql-local
compose/smoke-test.sh sqlite-s3
```

Pass additional simulator arguments after `--`:

```bash
compose/smoke-test.sh full -- --chunks 5 --simulate-failure-every 2
```

The script uses `PROOFLINE_MAIN_PORT` and `PROOFLINE_ADMIN_PORT` when set,
defaulting to `18080` and `18081`. `PROOFLINE_PRIVATE_PORT` and
`PROOFLINE_PUBLIC_PORT` remain accepted by the script as legacy aliases for
those local host ports.

```bash
PROOFLINE_MAIN_PORT=28080 PROOFLINE_ADMIN_PORT=28081 compose/smoke-test.sh full
```

The local smoke auth values can also be overridden:

```bash
PROOFLINE_SMOKE_BOOTSTRAP_SECRET='replace-with-local-compose-bootstrap-secret' \
PROOFLINE_SMOKE_USERNAME=admin \
PROOFLINE_SMOKE_PASSWORD='replace-with-a-long-local-password' \
compose/smoke-test.sh sqlite-local
```

Set `KEEP_COMPOSE=1` to leave containers and volumes running after the smoke
test for manual inspection.

## Optional S3 Deletion Smoke

The default Go test suite does not require MinIO or any object-store service.
To verify incident deletion against a real S3-compatible object-store path, run
the opt-in HTTP API smoke test with a disposable local MinIO bucket.

One local setup shape is:

```bash
docker network create proofline-s3-smoke

docker run --rm -d \
  --name proofline-s3-smoke-minio \
  --network proofline-s3-smoke \
  -p 127.0.0.1:19000:9000 \
  -e MINIO_ROOT_USER=proofline \
  -e MINIO_ROOT_PASSWORD=proofline-minio-password \
  quay.io/minio/minio:latest server /data

docker run --rm \
  --entrypoint /bin/sh \
  --network proofline-s3-smoke \
  -e MINIO_ROOT_USER=proofline \
  -e MINIO_ROOT_PASSWORD=proofline-minio-password \
  -e MINIO_BUCKET=proofline-evidence \
  quay.io/minio/mc:latest -c '
    until mc alias set proofline http://proofline-s3-smoke-minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"; do
      sleep 1
    done
    mc mb --ignore-existing "proofline/$MINIO_BUCKET"
  '
```

Then run the opt-in test from the repository root:

```bash
SAFE_S3_DELETION_SMOKE=1 \
SAFE_S3_ENDPOINT=http://127.0.0.1:19000 \
SAFE_S3_REGION=us-east-1 \
SAFE_S3_BUCKET=proofline-evidence \
SAFE_S3_PREFIX=smoke/httpapi-deletion \
SAFE_S3_ACCESS_KEY_ID=proofline \
SAFE_S3_SECRET_ACCESS_KEY=proofline-minio-password \
SAFE_S3_FORCE_PATH_STYLE=true \
go test ./internal/httpapi -run TestS3DeletionSmokeRemovesObjectsAndHidesViewer -count=1
```

Clean up the disposable local service afterwards:

```bash
docker rm -f proofline-s3-smoke-minio
docker network rm proofline-s3-smoke
```

The smoke test uploads encrypted test chunks through the main API handler,
checks the objects through server-controlled stored paths, requests private
incident deletion, runs one deletion-worker pass, confirms the objects are gone
or already absent from the object store, and verifies public viewer routes keep
returning the generic fail-closed token error. Do not use production
credentials, non-disposable buckets, private endpoints, raw tokens, uploaded
bytes, plaintext, raw keys, object keys, stored paths, or private deployment
details in public issue comments, logs, screenshots, or support material from
this smoke run.
