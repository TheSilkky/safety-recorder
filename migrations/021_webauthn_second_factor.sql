CREATE TABLE auth_sessions_webauthn_migration (
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
    OR second_factor_method IN ('totp', 'webauthn')
  ),
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT
);

INSERT INTO auth_sessions_webauthn_migration (
  id, account_id, token_hash, second_factor_verified_at, second_factor_factor_id,
  second_factor_method, created_at, expires_at, revoked_at
)
SELECT
  id, account_id, token_hash, second_factor_verified_at, second_factor_factor_id,
  second_factor_method, created_at, expires_at, revoked_at
FROM auth_sessions;

DROP TABLE auth_sessions;
ALTER TABLE auth_sessions_webauthn_migration RENAME TO auth_sessions;

CREATE INDEX IF NOT EXISTS idx_auth_sessions_account_id ON auth_sessions(account_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_token_hash ON auth_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS account_webauthn_users (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  rp_id TEXT NOT NULL,
  user_handle BLOB NOT NULL CHECK (
    length(user_handle) >= 16
    AND length(user_handle) <= 64
  ),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (account_id, rp_id),
  UNIQUE (rp_id, user_handle)
);

CREATE TABLE IF NOT EXISTS account_webauthn_credentials (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  rp_id TEXT NOT NULL,
  credential_id BLOB NOT NULL,
  public_key BLOB NOT NULL,
  attestation_type TEXT NOT NULL,
  attestation_format TEXT NOT NULL,
  transports TEXT NOT NULL DEFAULT '',
  aaguid BLOB,
  sign_count INTEGER NOT NULL DEFAULT 0 CHECK (sign_count >= 0),
  clone_warning INTEGER NOT NULL DEFAULT 0 CHECK (clone_warning IN (0, 1)),
  attachment TEXT NOT NULL DEFAULT '',
  user_present INTEGER NOT NULL DEFAULT 0 CHECK (user_present IN (0, 1)),
  user_verified INTEGER NOT NULL DEFAULT 0 CHECK (user_verified IN (0, 1)),
  backup_eligible INTEGER NOT NULL DEFAULT 0 CHECK (backup_eligible IN (0, 1)),
  backup_state INTEGER NOT NULL DEFAULT 0 CHECK (backup_state IN (0, 1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  verified_at TEXT,
  last_used_at TEXT,
  UNIQUE (rp_id, credential_id)
);

CREATE INDEX IF NOT EXISTS idx_account_webauthn_credentials_account_id
  ON account_webauthn_credentials(account_id);
CREATE INDEX IF NOT EXISTS idx_account_webauthn_credentials_account_rp
  ON account_webauthn_credentials(account_id, rp_id);
CREATE INDEX IF NOT EXISTS idx_account_webauthn_credentials_credential_id
  ON account_webauthn_credentials(rp_id, credential_id);

CREATE TABLE IF NOT EXISTS account_webauthn_challenges (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  auth_session_id TEXT NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
  rp_id TEXT NOT NULL,
  challenge_type TEXT NOT NULL CHECK (
    challenge_type IN ('webauthn_registration', 'webauthn_assertion')
  ),
  session_data_json BLOB NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_account_webauthn_challenges_account_id
  ON account_webauthn_challenges(account_id);
CREATE INDEX IF NOT EXISTS idx_account_webauthn_challenges_session_type
  ON account_webauthn_challenges(auth_session_id, rp_id, challenge_type);
CREATE INDEX IF NOT EXISTS idx_account_webauthn_challenges_expires_at
  ON account_webauthn_challenges(expires_at);
