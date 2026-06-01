ALTER TABLE accounts ADD COLUMN email_normalized TEXT;
ALTER TABLE accounts ADD COLUMN email_verified_at TEXT;
ALTER TABLE accounts ADD COLUMN account_state TEXT NOT NULL DEFAULT 'active' CHECK (
  account_state IN (
    'pending_email_verification',
    'active',
    'disabled',
    'suspended',
    'pending_payment'
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_email_normalized
  ON accounts(email_normalized)
  WHERE email_normalized IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_account_state ON accounts(account_state);

CREATE TABLE IF NOT EXISTS account_verification_tokens (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  purpose TEXT NOT NULL CHECK (purpose <> ''),
  token_hash TEXT NOT NULL UNIQUE CHECK (
    length(token_hash) = 64
    AND token_hash NOT GLOB '*[^0-9a-f]*'
  ),
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_account_verification_tokens_account_id
  ON account_verification_tokens(account_id);
CREATE INDEX IF NOT EXISTS idx_account_verification_tokens_purpose_hash
  ON account_verification_tokens(purpose, token_hash);
CREATE INDEX IF NOT EXISTS idx_account_verification_tokens_expires_at
  ON account_verification_tokens(expires_at);
