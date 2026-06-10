CREATE TABLE IF NOT EXISTS sharing_audit_events (
  id TEXT PRIMARY KEY,
  owner_account_id TEXT NOT NULL,
  actor_account_id TEXT NOT NULL,
  action TEXT NOT NULL CHECK (
    action IN (
      'contact_key_registered',
      'contact_key_replaced',
      'contact_key_revoked',
      'contact_key_lost',
      'sharing_grant_created',
      'sharing_grant_revoked',
      'wrapped_key_created',
      'wrapped_key_revoked',
      'incident_sharing_metadata_pruned'
    )
  ),
  outcome_category TEXT NOT NULL CHECK (
    outcome_category IN ('created', 'replaced', 'revoked', 'lost', 'deleted')
  ),
  incident_id TEXT,
  stream_id TEXT,
  grant_id TEXT,
  contact_id TEXT,
  contact_public_key_id TEXT,
  wrapped_key_id TEXT,
  deletion_decision_id TEXT,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sharing_audit_events_owner_created ON sharing_audit_events(owner_account_id, created_at);
CREATE INDEX IF NOT EXISTS idx_sharing_audit_events_incident ON sharing_audit_events(incident_id);
CREATE INDEX IF NOT EXISTS idx_sharing_audit_events_grant ON sharing_audit_events(grant_id);
CREATE INDEX IF NOT EXISTS idx_sharing_audit_events_contact_key ON sharing_audit_events(contact_public_key_id);
CREATE INDEX IF NOT EXISTS idx_sharing_audit_events_wrapped_key ON sharing_audit_events(wrapped_key_id);
CREATE INDEX IF NOT EXISTS idx_sharing_audit_events_deletion_decision ON sharing_audit_events(deletion_decision_id);
