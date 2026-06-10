ALTER TABLE auth_sessions
  DROP CONSTRAINT IF EXISTS auth_sessions_second_factor_method_check;

ALTER TABLE auth_sessions
  ADD CONSTRAINT auth_sessions_second_factor_method_check CHECK (
    second_factor_method IS NULL
    OR second_factor_method IN ('totp', 'webauthn')
  );

CREATE TABLE IF NOT EXISTS account_webauthn_users (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  rp_id TEXT NOT NULL,
  user_handle BYTEA NOT NULL CHECK (
    octet_length(user_handle) >= 16
    AND octet_length(user_handle) <= 64
  ),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (account_id, rp_id),
  UNIQUE (rp_id, user_handle)
);

CREATE TABLE IF NOT EXISTS account_webauthn_credentials (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  rp_id TEXT NOT NULL,
  credential_id BYTEA NOT NULL,
  public_key BYTEA NOT NULL,
  attestation_type TEXT NOT NULL,
  attestation_format TEXT NOT NULL,
  transports TEXT NOT NULL DEFAULT '',
  aaguid BYTEA,
  sign_count BIGINT NOT NULL DEFAULT 0 CHECK (sign_count >= 0),
  clone_warning BOOLEAN NOT NULL DEFAULT false,
  attachment TEXT NOT NULL DEFAULT '',
  user_present BOOLEAN NOT NULL DEFAULT false,
  user_verified BOOLEAN NOT NULL DEFAULT false,
  backup_eligible BOOLEAN NOT NULL DEFAULT false,
  backup_state BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  verified_at TIMESTAMPTZ,
  last_used_at TIMESTAMPTZ,
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
  session_data_json BYTEA NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_account_webauthn_challenges_account_id
  ON account_webauthn_challenges(account_id);
CREATE INDEX IF NOT EXISTS idx_account_webauthn_challenges_session_type
  ON account_webauthn_challenges(auth_session_id, rp_id, challenge_type);
CREATE INDEX IF NOT EXISTS idx_account_webauthn_challenges_expires_at
  ON account_webauthn_challenges(expires_at);
