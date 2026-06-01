CREATE TABLE IF NOT EXISTS wrapped_key_records (
  id TEXT PRIMARY KEY,
  owner_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  stream_id TEXT REFERENCES media_streams(id) ON DELETE CASCADE,
  grant_id TEXT NOT NULL REFERENCES sharing_grants(id) ON DELETE CASCADE,
  recipient_type TEXT NOT NULL CHECK (recipient_type IN ('trusted_contact')),
  contact_id TEXT NOT NULL,
  contact_public_key_id TEXT NOT NULL REFERENCES contact_public_keys(id) ON DELETE CASCADE,
  contact_public_key_version INTEGER NOT NULL CHECK (contact_public_key_version > 0),
  media_key_id TEXT NOT NULL CHECK (length(media_key_id) > 0 AND length(media_key_id) <= 255),
  wrapping_algorithm TEXT NOT NULL CHECK (length(wrapping_algorithm) > 0 AND length(wrapping_algorithm) <= 80),
  wrapping_algorithm_version TEXT NOT NULL CHECK (length(wrapping_algorithm_version) > 0 AND length(wrapping_algorithm_version) <= 80),
  wrapped_key_ciphertext TEXT NOT NULL CHECK (length(wrapped_key_ciphertext) > 0 AND length(wrapped_key_ciphertext) <= 16384),
  public_wrapping_metadata TEXT NOT NULL CHECK (length(public_wrapping_metadata) > 0 AND length(public_wrapping_metadata) <= 4096),
  wrapped_key_state TEXT NOT NULL CHECK (wrapped_key_state IN ('active', 'revoked', 'rotated')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  revoked_at TEXT,
  revoked_by_account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,
  rotated_at TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wrapped_key_records_unique_scope
ON wrapped_key_records(owner_account_id, incident_id, COALESCE(stream_id, ''), grant_id, media_key_id, contact_public_key_id);

CREATE INDEX IF NOT EXISTS idx_wrapped_key_records_owner ON wrapped_key_records(owner_account_id);
CREATE INDEX IF NOT EXISTS idx_wrapped_key_records_incident ON wrapped_key_records(owner_account_id, incident_id);
CREATE INDEX IF NOT EXISTS idx_wrapped_key_records_grant ON wrapped_key_records(owner_account_id, grant_id);
CREATE INDEX IF NOT EXISTS idx_wrapped_key_records_state ON wrapped_key_records(owner_account_id, wrapped_key_state);
