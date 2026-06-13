ALTER TABLE auth_sessions
  DROP CONSTRAINT IF EXISTS auth_sessions_second_factor_method_check;

ALTER TABLE auth_sessions
  ADD CONSTRAINT auth_sessions_second_factor_method_check CHECK (
    second_factor_method IS NULL
    OR second_factor_method IN ('email_challenge', 'totp', 'webauthn')
  );
