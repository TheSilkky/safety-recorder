# Cluster Backup, Restore, And Failure Runbook

This runbook covers backup, restore, and failure handling for deployments that
explicitly use the optional PostgreSQL metadata backend, optional S3-compatible
encrypted blob backend, and optional Valkey/Redis-compatible coordination
backend.

Proofline is still experimental and not production-ready public
infrastructure. These runbooks do not add public `/v1` exposure, public account
workflows, retention enforcement, observability, abuse controls, backup
automation, cloud deployment scripts, backend decryption, key escrow, or key
custody behavior.
They also do not make a multi-node deployment safe without the future
cluster-safe upload operation work documented in
[cluster-safe upload operation semantics](cluster-safe-upload-semantics.md).

Do not put raw viewer tokens, incident tokens, request bodies, uploaded bytes,
object keys, database connection strings, credentials, private hostnames,
network topology, plaintext, raw keys, exploit details, or real user safety data
into public issues, public runbooks, support tickets, logs, dashboards, or
screenshots.

## Durable State

Cluster-style deployments must keep durable metadata and encrypted blobs
together. Coordination state is intentionally not evidence storage.

| System | Role | Backup source of truth |
|---|---|---|
| PostgreSQL metadata | Incidents, streams, chunk metadata, checkins, viewer-token hashes, migrations, and durable upload-operation state for complete chunk upload idempotency. | Yes, when `SAFE_METADATA_BACKEND=postgresql`. |
| S3-compatible encrypted blob storage | Committed encrypted chunk bytes addressed by server-controlled final object keys. | Yes, when `SAFE_BLOB_BACKEND=s3`. |
| Deployment configuration | Backend selectors, bind addresses, data paths, upload limits, token TTL defaults, S3 settings, PostgreSQL settings, Valkey settings, reverse-proxy routing, and secret references. | Yes, but keep secret values in a private secret-management backup, not in public docs or tickets. |
| Local `SAFE_DATA_DIR/tmp` | Temporary upload staging before final commit. Current S3 support uses local temp files and does not create S3 staging objects. | No, except for forensic review during a private incident response. |
| Valkey/Redis-compatible coordination | Startup-checked short-lived coordination only. Future use may include leases, in-progress hints, or retry coordination. | No. It must not be treated as durable evidence storage. |
| Generated ZIP bundles | On-demand HTTP responses containing encrypted chunks and manifests. | No. The server does not persist generated bundle files. |

SQLite metadata and local filesystem blobs remain supported for local-first
deployments. The broader retention, backup, and deletion policy is documented in
[retention, backup, and deletion](retention-backup-deletion.md).

## Backup Runbook

Use this runbook before relying on optional PostgreSQL and S3-compatible
backends for real incident evidence. Keep the actual command lines,
credentials, endpoints, bucket names, prefixes, hostnames, and private network
details in private operator documentation.

1. Confirm the deployment shape.
   - Record whether the deployment uses `SAFE_METADATA_BACKEND=postgresql`,
     `SAFE_BLOB_BACKEND=s3`, and `SAFE_COORDINATION_BACKEND=valkey` or `redis`.
   - Confirm SQLite and local filesystem fallbacks are not being confused with
     the active cluster-style backend selectors.
   - Confirm the main `/v1` listener is still behind the deployment's reviewed
     boundary, such as localhost, LAN, WireGuard, firewall rules, or a strict
     private reverse proxy.

2. Choose a consistency window.
   - Prefer a quiesced window where authenticated main write routes are stopped or blocked
     before the metadata and blob backups are taken.
   - If a fully quiesced window is not possible, use database and object-store
     backup mechanisms that can be tied to the same recovery point and document
     the remaining risk in private operator notes.
   - Do not use Valkey coordination state as proof that all uploads are
     complete; durable metadata and committed blobs are the evidence record.

3. Back up PostgreSQL metadata.
   - Use a PostgreSQL backup method appropriate for the deployment, such as a
     logical dump, physical backup, or managed database snapshot.
   - Include schema migrations and all Proofline metadata tables.
   - Treat database backup output, backup logs, and failure output as sensitive
     if they can include IDs, labels, timestamps, private paths, or connection
     details.

4. Back up S3-compatible committed encrypted blobs.
   - Back up the configured bucket and safe prefix that contain committed
     encrypted chunk objects.
   - Preserve object bytes, object metadata needed by the backend or operator,
     and enough inventory information to compare restored blobs with restored
     metadata in a private environment.
   - Do not expose object-store URLs, object keys, bucket names, configured
     prefixes, credentials, or private endpoints in public places.

5. Back up deployment configuration and secret references.
   - Preserve backend selectors, bind-address configuration, token TTL defaults,
     upload limits, timeout settings, reverse-proxy route separation, and
     secret-reference names.
   - Preserve actual secret values only in the deployment's private secret
     backup process.
   - Do not copy raw credentials into this repository, issue bodies, PR
     descriptions, public runbooks, or shared screenshots.

6. Exclude short-lived coordination from evidence backups.
   - Do not back up Valkey as if it contains incident metadata, chunk metadata,
     committed encrypted bytes, viewer-token hashes, retention decisions, or
     deletion decisions.
   - If Valkey is backed up for unrelated operational reasons, label it as
     short-lived coordination only.

7. Verify the backup set privately.
   - Check that the metadata backup and blob backup cover the same intended
     recovery window.
   - Compare restored or sampled metadata rows with private object inventory in
     a non-public environment.
   - Confirm backup monitoring avoids raw tokens, request bodies, uploaded
     bytes, Authorization headers, plaintext, raw keys, and private deployment
     details.

## Restore Runbook

Run restore drills before relying on the system for real incidents. Restore
drills must preserve the main/private-admin listener split and must not expose
main `/v1` routes publicly without a reviewed deployment boundary.

1. Restore into an isolated environment.
   - Use private or loopback bind addresses only, such as
     `SAFE_MAIN_BIND_ADDRS=127.0.0.1:8080` and
     `SAFE_ADMIN_BIND_ADDRS=127.0.0.1:8081`.
   - Do not route a restore drill through a public reverse-proxy entry point.
   - Do not use real viewer links or raw tokens in shared notes while testing.

2. Restore metadata first.
   - Restore PostgreSQL into an isolated database.
   - Apply or verify the expected PostgreSQL schema migrations.
   - Confirm the application can open the restored metadata backend without
     printing connection strings or credentials.

3. Restore encrypted blobs for the same recovery window.
   - Restore the S3-compatible bucket and prefix, or restore into an isolated
     bucket or prefix dedicated to the drill.
   - Preserve server-controlled final object keys exactly inside the private
     restored storage.
   - Do not expose object-store URLs or configured prefixes through public
     viewer responses, logs, screenshots, or tickets.

4. Restore deployment configuration.
   - Use the same backend selectors as the backup set.
   - Keep `/v1` on local or otherwise reviewed private-boundary addresses.
   - Use private secret injection for credentials instead of writing secret
     values into the runbook or shell history.

5. Validate metadata and blob consistency.
   - Start the API in the isolated environment.
   - Load known incident metadata through authenticated main `/v1` routes only.
   - Generate completed stream or incident encrypted ZIP bundles.
   - Confirm generated manifests match expected stream and chunk metadata.
   - Confirm completed bundle generation fails closed when a required blob is
     missing or when metadata and blobs do not match.

6. Validate coordination loss behavior.
   - If Valkey/Redis coordination is configured and unavailable at startup, the
     server should fail closed.
   - For restore drills that do not need coordination, use
     `SAFE_COORDINATION_BACKEND=none` only when that accurately represents the
     deployment shape being tested.
   - Future operation-level coordination loss should be handled as retryable
     operational failure, not as evidence loss, because durable state belongs in
     PostgreSQL and committed blob storage.

7. Keep restored environments private.
   - Do not expose `/v1` publicly during validation.
   - Do not use restore drills to claim production readiness.
   - Tear down or lock down restored copies according to the deployment's
     private retention and backup policy.

## Failure Runbook

Failure handling should be conservative. Do not overwrite committed evidence,
delete ambiguous blobs, or infer that coordination state can repair missing
durable state.

### Missing Blob Referenced By Metadata

If PostgreSQL metadata references a committed blob that cannot be opened from
the configured blob backend:

- Treat completed bundle generation failure as correct fail-closed behavior.
- Do not silently omit the missing chunk from an incident bundle.
- Check whether the blob exists in another backup generation for the same
  recovery window.
- Do not recreate encrypted chunk bytes, change metadata hashes, or mark the
  missing blob as present without a separate evidence-preserving recovery
  design.
- Keep any incident-specific details in private operator notes.

### Blob Without Matching Metadata

If a committed final object exists but no restored metadata row references it:

- Treat the object as ambiguous.
- Do not delete it during normal cleanup unless a separate private recovery
  review proves it is safe to remove.
- Do not expose object keys, bucket paths, or object URLs in public issue
  discussion.
- Prefer preserving the object until metadata backup scope, restore ordering,
  or future upload-operation ownership proof can explain the mismatch.

### PostgreSQL Restore Without Matching Blobs

If metadata restores but the matching S3-compatible blob set does not:

- Treat the restore as incomplete.
- Keep `/v1` private and do not expose the public viewer as if evidence is
  available.
- Restore the matching blob generation before using the environment for
  evidence review.
- Verify completed bundle generation after blobs are restored.

### Blob Restore Without Matching PostgreSQL Metadata

If encrypted blob objects restore but the matching PostgreSQL metadata does not:

- Treat the restore as incomplete.
- Do not build ad hoc public object listings or dashboards for evidence review.
- Restore metadata from the same recovery window before serving bundles.
- If metadata cannot be recovered, preserve the blobs for private forensic or
  operator review without claiming normal application-level recoverability.

### Abandoned Local Temp Files

Current S3-compatible uploads stage bytes under `SAFE_DATA_DIR/tmp` before a
final conditional object write. If the process crashes, local temp files may
remain.

- Treat `SAFE_DATA_DIR/tmp` as staging, not committed evidence.
- Clean it only with a conservative private operator policy.
- Never delete committed blob objects or metadata rows as part of temp-file
  cleanup.
- Do not use client-provided stored paths, object keys, or filesystem paths to
  decide cleanup targets.
- Leaving old staging files is preferable to deleting possible evidence.

### Future Object-Storage Staging Objects

The current S3 backend does not create S3 staging objects. If a future resumable
or multipart design adds object-storage staging:

- Keep staging prefixes server-controlled.
- Expire only objects that are proven to be temporary staging.
- Do not delete final committed objects unless metadata and operation ownership
  proof show they are safe to remove.
- Preserve any cleanup design in security, retention, backup, restore, and
  threat-model docs before implementation.

### Coordination Service Unavailable

If Valkey/Redis-compatible coordination is configured but unavailable at
startup:

- Treat startup failure as expected fail-closed behavior.
- Restore or repair coordination service availability before starting a
  deployment that requires it.
- Do not bypass startup failure by changing backend selectors unless the
  operator intentionally chooses a private local-first shape and documents the
  operational impact.

If future operation-level coordination is unavailable after startup:

- Return retryable operational errors for affected operations.
- Rely on PostgreSQL constraints, immutable blob commits, and client retries to
  protect durable state.
- Do not store committed evidence, viewer-token hashes, retention decisions, or
  deletion decisions only in Valkey.

## Production-Cluster Readiness Gate

Do not recommend a production-cluster deployment for real use until at least the
following work exists and has been reviewed:

- public `/v1` access-control design or a deployment that keeps `/v1` strictly
  private
- cluster-safe upload operation semantics implemented and tested
- backup and restore drills for PostgreSQL metadata plus encrypted blobs
- retention and deletion enforcement appropriate for the deployment
- observability that redacts raw tokens, request bodies, uploaded bytes,
  Authorization headers, plaintext, raw keys, object keys, credentials, and
  private deployment details
- abuse controls and rate limiting for exposed public viewer routes
- operator guidance for incident response, restore approval, and evidence
  handling

Until those controls exist, these runbooks are planning and operational
guidance for experimental deployments, not a production-readiness statement.
