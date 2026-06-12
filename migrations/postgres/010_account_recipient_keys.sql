CREATE TABLE IF NOT EXISTS account_recipient_keys (
  id TEXT PRIMARY KEY,
  owner_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  recipient_id TEXT NOT NULL,
  recipient_type TEXT NOT NULL CHECK (recipient_type IN ('account', 'device')),
  key_id TEXT NOT NULL CHECK (length(key_id) > 0 AND length(key_id) <= 255),
  version INTEGER NOT NULL CHECK (version > 0),
  display_label TEXT,
  scheme TEXT NOT NULL CHECK (length(scheme) > 0 AND length(scheme) <= 120),
  suite_id TEXT NOT NULL CHECK (length(suite_id) > 0 AND length(suite_id) <= 160),
  public_key TEXT NOT NULL CHECK (length(public_key) > 0 AND length(public_key) <= 4096),
  public_key_fingerprint TEXT NOT NULL CHECK (length(public_key_fingerprint) > 0 AND length(public_key_fingerprint) <= 256),
  key_state TEXT NOT NULL CHECK (
    key_state IN ('pending_verification', 'active', 'replaced', 'revoked', 'lost')
  ),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  replaced_at TIMESTAMPTZ,
  lost_at TIMESTAMPTZ,
  replaced_by_recipient_key_id TEXT REFERENCES account_recipient_keys(id) ON DELETE SET NULL,
  UNIQUE(owner_account_id, recipient_id, version),
  UNIQUE(owner_account_id, key_id)
);

CREATE INDEX IF NOT EXISTS idx_account_recipient_keys_owner ON account_recipient_keys(owner_account_id);
CREATE INDEX IF NOT EXISTS idx_account_recipient_keys_recipient ON account_recipient_keys(owner_account_id, recipient_id);
CREATE INDEX IF NOT EXISTS idx_account_recipient_keys_state ON account_recipient_keys(owner_account_id, key_state);
