CREATE TABLE auth_sessions_email_2fa_migration (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE CHECK (
    length(token_hash) = 64
    AND token_hash GLOB '[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]'
  ),
  second_factor_verified_at TEXT,
  second_factor_factor_id TEXT,
  second_factor_method TEXT CHECK (
    second_factor_method IS NULL
    OR second_factor_method IN ('email_challenge', 'totp', 'webauthn')
  ),
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT
);

INSERT INTO auth_sessions_email_2fa_migration (
  id, account_id, token_hash, second_factor_verified_at, second_factor_factor_id,
  second_factor_method, created_at, expires_at, revoked_at
)
SELECT
  id, account_id, token_hash, second_factor_verified_at, second_factor_factor_id,
  second_factor_method, created_at, expires_at, revoked_at
FROM auth_sessions;

DROP TABLE auth_sessions;
ALTER TABLE auth_sessions_email_2fa_migration RENAME TO auth_sessions;

CREATE INDEX IF NOT EXISTS idx_auth_sessions_account_id ON auth_sessions(account_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_token_hash ON auth_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);
