CREATE TABLE IF NOT EXISTS legacy_incident_reassignment_events (
  id TEXT PRIMARY KEY,
  incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  previous_owner_account_id TEXT,
  new_owner_account_id TEXT,
  actor_account_id TEXT NOT NULL,
  action TEXT NOT NULL CHECK (action IN ('assign_owner', 'keep_unowned')),
  reason_code TEXT NOT NULL CHECK (
    reason_code IN (
      'owner_verified',
      'owner_request',
      'operator_review',
      'keep_admin_only',
      'unknown_owner',
      'other_controlled'
    )
  ),
  source TEXT NOT NULL CHECK (source IN ('admin_api', 'operator_cli')),
  created_at TEXT NOT NULL,
  completed_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_legacy_incident_reassignment_events_incident_id
  ON legacy_incident_reassignment_events(incident_id);
CREATE INDEX IF NOT EXISTS idx_legacy_incident_reassignment_events_actor
  ON legacy_incident_reassignment_events(actor_account_id);
