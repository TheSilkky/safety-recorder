CREATE TABLE IF NOT EXISTS trusted_contact_relationships (
  id TEXT PRIMARY KEY,
  owner_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  recipient_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  relationship_role TEXT NOT NULL CHECK (relationship_role IN ('trusted_contact')),
  relationship_state TEXT NOT NULL CHECK (
    relationship_state IN ('pending_invite', 'active', 'declined', 'revoked', 'replaced')
  ),
  display_label TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  invited_at TEXT NOT NULL,
  accepted_at TEXT,
  declined_at TEXT,
  revoked_at TEXT,
  revoked_by_account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,
  replaced_at TEXT,
  replaced_by_relationship_id TEXT REFERENCES trusted_contact_relationships(id) ON DELETE SET NULL,
  CHECK (owner_account_id <> recipient_account_id)
);

CREATE INDEX IF NOT EXISTS idx_trusted_contact_relationships_owner ON trusted_contact_relationships(owner_account_id);
CREATE INDEX IF NOT EXISTS idx_trusted_contact_relationships_recipient ON trusted_contact_relationships(recipient_account_id);
CREATE INDEX IF NOT EXISTS idx_trusted_contact_relationships_state ON trusted_contact_relationships(owner_account_id, relationship_state);
CREATE UNIQUE INDEX IF NOT EXISTS idx_trusted_contact_relationships_open_pair
  ON trusted_contact_relationships(owner_account_id, recipient_account_id, relationship_role)
  WHERE relationship_state IN ('pending_invite', 'active');
