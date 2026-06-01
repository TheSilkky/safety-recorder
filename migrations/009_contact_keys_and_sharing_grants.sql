CREATE TABLE IF NOT EXISTS contact_public_keys (
  id TEXT PRIMARY KEY,
  owner_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  contact_id TEXT NOT NULL,
  version INTEGER NOT NULL CHECK (version > 0),
  display_label TEXT,
  wrapping_algorithm TEXT NOT NULL CHECK (length(wrapping_algorithm) > 0 AND length(wrapping_algorithm) <= 80),
  public_key TEXT NOT NULL CHECK (length(public_key) > 0 AND length(public_key) <= 4096),
  public_key_fingerprint TEXT NOT NULL CHECK (length(public_key_fingerprint) > 0 AND length(public_key_fingerprint) <= 256),
  key_state TEXT NOT NULL CHECK (
    key_state IN ('pending_verification', 'active', 'replaced', 'revoked', 'lost')
  ),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  revoked_at TEXT,
  UNIQUE(owner_account_id, contact_id, version)
);

CREATE INDEX IF NOT EXISTS idx_contact_public_keys_owner ON contact_public_keys(owner_account_id);
CREATE INDEX IF NOT EXISTS idx_contact_public_keys_contact ON contact_public_keys(owner_account_id, contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_public_keys_state ON contact_public_keys(owner_account_id, key_state);

CREATE TABLE IF NOT EXISTS sharing_grants (
  id TEXT PRIMARY KEY,
  owner_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  stream_id TEXT REFERENCES media_streams(id) ON DELETE CASCADE,
  recipient_type TEXT NOT NULL CHECK (recipient_type IN ('trusted_contact')),
  contact_id TEXT NOT NULL,
  contact_public_key_id TEXT NOT NULL REFERENCES contact_public_keys(id) ON DELETE CASCADE,
  contact_public_key_version INTEGER NOT NULL CHECK (contact_public_key_version > 0),
  data_class TEXT NOT NULL CHECK (
    data_class IN ('metadata', 'ciphertext', 'metadata_ciphertext')
  ),
  grant_state TEXT NOT NULL CHECK (grant_state IN ('active', 'revoked')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  expires_at TEXT,
  revoked_at TEXT,
  revoked_by_account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_sharing_grants_owner ON sharing_grants(owner_account_id);
CREATE INDEX IF NOT EXISTS idx_sharing_grants_incident ON sharing_grants(owner_account_id, incident_id);
CREATE INDEX IF NOT EXISTS idx_sharing_grants_contact ON sharing_grants(owner_account_id, contact_id);
CREATE INDEX IF NOT EXISTS idx_sharing_grants_state ON sharing_grants(owner_account_id, grant_state);
