# Getting Started

This guide starts the Proofline backend locally and runs the simulator against it.

## Requirements

- Go 1.26.3
- SQLite through the bundled Go SQLite driver dependency
- PostgreSQL only when explicitly using `SAFE_METADATA_BACKEND=postgresql`
- Local disk storage for encrypted uploaded blobs

## Run The Backend

From the repository root:

```bash
SAFE_AUTH_BOOTSTRAP_SECRET='replace-with-local-bootstrap-secret' \
go run ./cmd/api
```

Default listeners:

| Listener | Default address |
|---|---|
| Main API and incident viewer | `127.0.0.1:8080` |
| Private admin dashboard | `127.0.0.1:8081` |

The private admin web surface is available at
`http://127.0.0.1:8081/admin`. When `SAFE_AUTH_BOOTSTRAP_SECRET` is set and no
admin exists, that page shows the first-admin bootstrap screen; after an admin
exists, it shows the admin login screen and local account password workflows.

The backend writes local data under `./data` by default:

```text
data/
  safety.db
  tmp/
  incidents/{incident_id}/streams/{stream_id}/{media_type}_{zero_padded_chunk_index}.enc
  incidents/{incident_id}/{media_type}_{zero_padded_chunk_index}.enc
```

The database file name still uses `safety.db` until a separate artifact/data-layout migration is performed.

Uploads are staged in `tmp/`, hashed while streaming, and then hard-linked into the final incident path without overwriting existing chunk files. Streamed uploads use the stream-scoped path; the incident-level path remains for legacy unstreamed chunks.

For a new local database, create the first admin account before running the
simulator:

```bash
curl -sS -X POST http://127.0.0.1:8081/admin/bootstrap \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'bootstrap_secret=replace-with-local-bootstrap-secret' \
  --data-urlencode 'username=admin' \
  --data-urlencode 'password=replace-with-a-long-local-password'
```

Then restart the server without `SAFE_AUTH_BOOTSTRAP_SECRET`.

## Run The Simulator

In another terminal from the repository root:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

The simulator:

- logs in with a local account session
- creates an incident
- creates an incident viewer token using the current incident-token route
- creates an audio media stream
- encrypts fake chunk plaintext and uploads the ciphertext envelope with `stream_id`
- sends periodic checkins
- marks the stream complete
- downloads the completed encrypted stream bundle through the incident viewer when requested
- verifies local decryption of the downloaded bundle when encryption is enabled

The simulator exercises the current generic incident API. It does not set the
optional incident-mode metadata fields for emergency incidents, interaction
records, safety checks, or evidence notes.

## Useful Next Reads

- [Configuration](configuration.md)
- [Incident capture modes](incident-modes.md)
- [Encryption](encryption.md)
- [Simulator](simulator.md)
- [API reference](api.md)
- [Deployment](deployment.md)
