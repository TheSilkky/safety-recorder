CREATE TABLE IF NOT EXISTS account_recovery_events (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  admin_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  action TEXT NOT NULL CHECK (
    action IN ('second_factor_recovery_reset')
  ),
  reason TEXT NOT NULL CHECK (
    reason IN (
      'lost_email_access',
      'lost_totp_device',
      'lost_webauthn_credential',
      'lost_all_factors',
      'admin_created_setup',
      'operator_review'
    )
  ),
  previous_second_factor_setup_state TEXT NOT NULL CHECK (
    previous_second_factor_setup_state IN ('not_required', 'setup_required', 'complete')
  ),
  new_second_factor_setup_state TEXT NOT NULL CHECK (
    new_second_factor_setup_state IN ('setup_required')
  ),
  sessions_revoked BIGINT NOT NULL DEFAULT 0 CHECK (sessions_revoked >= 0),
  email_factors_removed BIGINT NOT NULL DEFAULT 0 CHECK (email_factors_removed >= 0),
  totp_factors_removed BIGINT NOT NULL DEFAULT 0 CHECK (totp_factors_removed >= 0),
  webauthn_credentials_removed BIGINT NOT NULL DEFAULT 0 CHECK (webauthn_credentials_removed >= 0),
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_account_recovery_events_account_id
  ON account_recovery_events(account_id, created_at);
CREATE INDEX IF NOT EXISTS idx_account_recovery_events_admin_account_id
  ON account_recovery_events(admin_account_id, created_at);
