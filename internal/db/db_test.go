package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestMigrateCreatesSchemaMigrationsAndRecordsSteps(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !hasTable(t, ctx, conn, "schema_migrations") {
		t.Fatal("expected schema_migrations table")
	}

	got := appliedMigrationIDs(t, ctx, conn)
	want := expectedMigrationIDs(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("applied migrations = %v, want %v", got, want)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	first := appliedMigrationIDs(t, ctx, conn)

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	second := appliedMigrationIDs(t, ctx, conn)
	if !reflect.DeepEqual(second, first) {
		t.Fatalf("second migration IDs = %v, want %v", second, first)
	}
}

func TestMigrateAddsIncidentModeColumns(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, columnName := range []string{"incident_mode", "capture_profile", "escalation_policy", "sharing_state"} {
		if !hasColumn(t, ctx, conn, "incidents", columnName) {
			t.Fatalf("expected incidents.%s column", columnName)
		}
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO incidents (
			id, created_at, updated_at, status, incident_mode, capture_profile,
			escalation_policy, sharing_state
		)
		VALUES (
			'inc_mode_valid',
			'2026-05-21T10:00:00Z',
			'2026-05-21T10:00:00Z',
			'open',
			'interaction_record',
			'audio_location',
			'none',
			'private'
		)`); err != nil {
		t.Fatalf("insert incident with mode fields: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status, incident_mode)
		VALUES ('inc_bad_mode', '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z', 'open', 'panic')`); err == nil {
		t.Fatal("expected invalid incident_mode to fail")
	}
}

func TestMigrateAddsIncidentDeletionRetentionSchema(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !hasColumn(t, ctx, conn, "incidents", "deletion_state") {
		t.Fatal("expected incidents.deletion_state column")
	}
	for _, tableName := range []string{"incident_deletion_decisions", "incident_deletion_items"} {
		if !hasTable(t, ctx, conn, tableName) {
			t.Fatalf("expected %s table", tableName)
		}
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status, deletion_state)
		VALUES ('inc_deleting', '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z', 'open', 'deleting')`); err != nil {
		t.Fatalf("insert incident with valid deletion state: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status, deletion_state)
		VALUES ('inc_bad_delete_state', '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z', 'open', 'purged')`); err == nil {
		t.Fatal("expected invalid deletion_state to fail")
	}
}

func TestMigrateAddsContactKeyAndSharingGrantSchema(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, tableName := range []string{"contact_public_keys", "sharing_grants", "wrapped_key_records"} {
		if !hasTable(t, ctx, conn, tableName) {
			t.Fatalf("expected %s table", tableName)
		}
	}
	for _, columnName := range []string{"owner_account_id", "contact_id", "version", "public_key", "public_key_fingerprint", "key_state"} {
		if !hasColumn(t, ctx, conn, "contact_public_keys", columnName) {
			t.Fatalf("expected contact_public_keys.%s column", columnName)
		}
	}
	for _, columnName := range []string{"owner_account_id", "incident_id", "stream_id", "contact_id", "contact_public_key_id", "grant_state"} {
		if !hasColumn(t, ctx, conn, "sharing_grants", columnName) {
			t.Fatalf("expected sharing_grants.%s column", columnName)
		}
	}
	for _, columnName := range []string{"owner_account_id", "incident_id", "stream_id", "grant_id", "media_key_id", "contact_public_key_id", "wrapped_key_ciphertext", "public_wrapping_metadata", "wrapped_key_state"} {
		if !hasColumn(t, ctx, conn, "wrapped_key_records", columnName) {
			t.Fatalf("expected wrapped_key_records.%s column", columnName)
		}
	}
	for _, tableName := range []string{"contact_public_keys", "sharing_grants", "wrapped_key_records"} {
		for _, forbidden := range []string{"private_key", "raw_media_key", "plaintext", "browser_fragment_secret", "server_escrow_key"} {
			if hasColumn(t, ctx, conn, tableName, forbidden) {
				t.Fatalf("%s must not include %s", tableName, forbidden)
			}
		}
	}
}

func TestMigrateRejectsRecordedChecksumMismatch(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `
		UPDATE schema_migrations
		SET checksum = 'bad'
		WHERE id = '001_init.sql'`); err != nil {
		t.Fatalf("update checksum: %v", err)
	}

	err := Migrate(ctx, conn)
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if !strings.Contains(err.Error(), "001_init.sql checksum mismatch") {
		t.Fatalf("expected checksum mismatch for 001_init.sql, got %v", err)
	}
}

func TestOpenCreatesMigrationTableAndEnablesWAL(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "safety.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if !hasTable(t, ctx, conn, "schema_migrations") {
		t.Fatal("expected schema_migrations table")
	}
	assertJournalMode(t, ctx, conn, "wal")
}

func TestOpenFailsWhenWALModeIsNotEnabled(t *testing.T) {
	ctx := context.Background()

	conn, err := Open(ctx, ":memory:")
	if err == nil {
		conn.Close()
		t.Fatal("expected :memory: database to fail WAL verification")
	}
	if !strings.Contains(err.Error(), `enable wal mode: sqlite returned journal mode "memory"`) {
		t.Fatalf("expected explicit WAL journal mode error, got %v", err)
	}
}

func TestIsWALJournalMode(t *testing.T) {
	tests := []struct {
		name        string
		journalMode string
		want        bool
	}{
		{
			name:        "wal",
			journalMode: "wal",
			want:        true,
		},
		{
			name:        "mixed case",
			journalMode: "WAL",
			want:        true,
		},
		{
			name:        "trimmed",
			journalMode: " wal ",
			want:        true,
		},
		{
			name:        "delete",
			journalMode: "delete",
			want:        false,
		},
		{
			name:        "memory",
			journalMode: "memory",
			want:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isWALJournalMode(test.journalMode); got != test.want {
				t.Fatalf("isWALJournalMode(%q) = %v, want %v", test.journalMode, got, test.want)
			}
		})
	}
}

func TestMigrateDropsIncidentTokenLastUsedAt(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	_, err := conn.ExecContext(ctx, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			status TEXT NOT NULL,
			client_label TEXT,
			notes TEXT
		);
		CREATE TABLE incident_tokens (
			id TEXT PRIMARY KEY,
			incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			label TEXT,
			created_at TEXT NOT NULL,
			expires_at TEXT,
			revoked_at TEXT,
			last_used_at TEXT
		);
		INSERT INTO incidents (id, created_at, updated_at, status)
		VALUES ('inc_existing', '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z', 'open');
		INSERT INTO incident_tokens (
			id, incident_id, token_hash, label, created_at, expires_at, revoked_at, last_used_at
		)
		VALUES (
			'etk_existing',
			'inc_existing',
			'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
			'trusted contact',
			'2026-05-21T10:00:00Z',
			NULL,
			NULL,
			'2026-05-21T10:05:00Z'
		);`)
	if err != nil {
		t.Fatalf("create old schema: %v", err)
	}

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if hasColumn(t, ctx, conn, "incident_tokens", "last_used_at") {
		t.Fatal("expected incident_tokens.last_used_at to be dropped")
	}

	var count int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM incident_tokens
		WHERE id = 'etk_existing'
			AND incident_id = 'inc_existing'
			AND token_hash = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
			AND label = 'trusted contact'
			AND created_at = '2026-05-21T10:00:00Z'
			AND expires_at IS NULL
			AND revoked_at IS NULL`).Scan(&count); err != nil {
		t.Fatalf("count preserved token: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected existing token row to be preserved, got count %d", count)
	}
	if !hasTable(t, ctx, conn, "schema_migrations") {
		t.Fatal("expected existing database migration to create schema_migrations")
	}
}

func TestMigrateRenamesEmergencyTokensToIncidentTokens(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	_, err := conn.ExecContext(ctx, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			status TEXT NOT NULL,
			client_label TEXT,
			notes TEXT
		);
		CREATE TABLE emergency_tokens (
			id TEXT PRIMARY KEY,
			incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			label TEXT,
			created_at TEXT NOT NULL,
			expires_at TEXT,
			revoked_at TEXT
		);
		INSERT INTO incidents (id, created_at, updated_at, status)
		VALUES ('inc_existing', '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z', 'open');
		INSERT INTO emergency_tokens (
			id, incident_id, token_hash, label, created_at, expires_at, revoked_at
		)
		VALUES (
			'etk_existing',
			'inc_existing',
			'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
			'trusted contact',
			'2026-05-21T10:00:00Z',
			NULL,
			NULL
		);`)
	if err != nil {
		t.Fatalf("create legacy token schema: %v", err)
	}

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if hasTable(t, ctx, conn, "emergency_tokens") {
		t.Fatal("expected legacy emergency_tokens table to be removed")
	}

	var count int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM incident_tokens
		WHERE id = 'etk_existing'
			AND incident_id = 'inc_existing'
			AND token_hash = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
			AND label = 'trusted contact'
			AND created_at = '2026-05-21T10:00:00Z'
			AND expires_at IS NULL
			AND revoked_at IS NULL`).Scan(&count); err != nil {
		t.Fatalf("count migrated token: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected existing token row to be migrated, got count %d", count)
	}
}

func TestMigrateAddsMissingChunkStreamIDColumn(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	createLegacyChunksSchema(t, ctx, conn, false)

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !hasColumn(t, ctx, conn, "chunks", "stream_id") {
		t.Fatal("expected chunks.stream_id column")
	}
}

func TestMigrateAllowsExistingChunkStreamIDColumn(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	createLegacyChunksSchema(t, ctx, conn, true)

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !hasColumn(t, ctx, conn, "chunks", "stream_id") {
		t.Fatal("expected chunks.stream_id column")
	}
}

func TestMigrateUsesStreamScopedChunkUniquenessForNewDatabase(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	insertMigrationTestIncident(t, ctx, conn, "inc_stream_scope")
	insertMigrationTestStream(t, ctx, conn, "str_audio_one", "inc_stream_scope", "audio")
	insertMigrationTestStream(t, ctx, conn, "str_audio_two", "inc_stream_scope", "audio")

	insertMigrationTestChunk(t, ctx, conn, "chk_stream_one", "inc_stream_scope", sql.NullString{String: "str_audio_one", Valid: true}, 1, "audio")
	insertMigrationTestChunk(t, ctx, conn, "chk_stream_two", "inc_stream_scope", sql.NullString{String: "str_audio_two", Valid: true}, 1, "audio")
	insertMigrationTestChunk(t, ctx, conn, "chk_legacy_audio", "inc_stream_scope", sql.NullString{}, 1, "audio")
	insertMigrationTestChunk(t, ctx, conn, "chk_legacy_video", "inc_stream_scope", sql.NullString{}, 1, "video")

	if err := insertMigrationTestChunkErr(ctx, conn, "chk_stream_duplicate", "inc_stream_scope", sql.NullString{String: "str_audio_one", Valid: true}, 1, "audio"); err == nil {
		t.Fatal("expected duplicate streamed chunk identity to fail")
	}
	if err := insertMigrationTestChunkErr(ctx, conn, "chk_legacy_duplicate", "inc_stream_scope", sql.NullString{}, 1, "audio"); err == nil {
		t.Fatal("expected duplicate legacy chunk identity to fail")
	}
}

func TestMigratePreservesLegacyChunksAndAddsStreamScopedUniqueness(t *testing.T) {
	ctx := context.Background()
	conn := openMemoryDB(t)
	defer conn.Close()

	createLegacyChunksSchema(t, ctx, conn, false)
	insertLegacyChunkBeforeStreamIDMigration(t, ctx, conn)

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !hasColumn(t, ctx, conn, "chunks", "stream_id") {
		t.Fatal("expected chunks.stream_id column")
	}
	assertChunkExists(t, ctx, conn, "chk_existing")

	insertMigrationTestStream(t, ctx, conn, "str_existing_one", "inc_existing", "audio")
	insertMigrationTestStream(t, ctx, conn, "str_existing_two", "inc_existing", "audio")
	insertMigrationTestChunk(t, ctx, conn, "chk_existing_stream_one", "inc_existing", sql.NullString{String: "str_existing_one", Valid: true}, 1, "audio")
	insertMigrationTestChunk(t, ctx, conn, "chk_existing_stream_two", "inc_existing", sql.NullString{String: "str_existing_two", Valid: true}, 1, "audio")

	if err := insertMigrationTestChunkErr(ctx, conn, "chk_existing_legacy_duplicate", "inc_existing", sql.NullString{}, 1, "audio"); err == nil {
		t.Fatal("expected duplicate legacy chunk identity to fail")
	}
}

func openMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return conn
}

func createLegacyChunksSchema(t *testing.T, ctx context.Context, conn *sql.DB, includeStreamID bool) {
	t.Helper()

	streamIDColumn := ""
	if includeStreamID {
		streamIDColumn = "stream_id TEXT,"
	}
	_, err := conn.ExecContext(ctx, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			status TEXT NOT NULL,
			client_label TEXT,
			notes TEXT
		);
		CREATE TABLE chunks (
			id TEXT PRIMARY KEY,
			incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
			`+streamIDColumn+`
			chunk_index INTEGER NOT NULL CHECK (chunk_index >= 0),
			media_type TEXT NOT NULL CHECK (media_type IN ('audio', 'video', 'location', 'metadata')),
			started_at TEXT NOT NULL,
			ended_at TEXT NOT NULL,
			original_filename TEXT,
			stored_path TEXT NOT NULL,
			byte_size INTEGER NOT NULL CHECK (byte_size >= 0),
			sha256_hex TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE (incident_id, media_type, chunk_index)
		);`)
	if err != nil {
		t.Fatalf("create legacy chunks schema: %v", err)
	}
}

func insertMigrationTestIncident(t *testing.T, ctx context.Context, conn *sql.DB, incidentID string) {
	t.Helper()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status)
		VALUES (?, '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z', 'open')`, incidentID); err != nil {
		t.Fatalf("insert incident: %v", err)
	}
}

func insertMigrationTestStream(t *testing.T, ctx context.Context, conn *sql.DB, streamID, incidentID, mediaType string) {
	t.Helper()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO media_streams (id, incident_id, media_type, status, created_at, updated_at)
		VALUES (?, ?, ?, 'open', '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z')`, streamID, incidentID, mediaType); err != nil {
		t.Fatalf("insert media stream: %v", err)
	}
}

func insertMigrationTestChunk(t *testing.T, ctx context.Context, conn *sql.DB, chunkID, incidentID string, streamID sql.NullString, chunkIndex int, mediaType string) {
	t.Helper()
	if err := insertMigrationTestChunkErr(ctx, conn, chunkID, incidentID, streamID, chunkIndex, mediaType); err != nil {
		t.Fatalf("insert chunk %s: %v", chunkID, err)
	}
}

func insertMigrationTestChunkErr(ctx context.Context, conn *sql.DB, chunkID, incidentID string, streamID sql.NullString, chunkIndex int, mediaType string) error {
	var streamValue any
	if streamID.Valid {
		streamValue = streamID.String
	}
	_, err := conn.ExecContext(ctx, `
		INSERT INTO chunks (
			id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
			stored_path, byte_size, sha256_hex, created_at
		)
		VALUES (
			?, ?, ?, ?, ?, '2026-05-21T10:00:00Z', '2026-05-21T10:00:01Z',
			?, 4, ?, '2026-05-21T10:00:02Z'
		)`,
		chunkID,
		incidentID,
		streamValue,
		chunkIndex,
		mediaType,
		"incidents/"+incidentID+"/"+chunkID+".enc",
		strings.Repeat("a", 64),
	)
	return err
}

func insertLegacyChunkBeforeStreamIDMigration(t *testing.T, ctx context.Context, conn *sql.DB) {
	t.Helper()
	_, err := conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status)
		VALUES ('inc_existing', '2026-05-21T10:00:00Z', '2026-05-21T10:00:00Z', 'open');
		INSERT INTO chunks (
			id, incident_id, chunk_index, media_type, started_at, ended_at,
			stored_path, byte_size, sha256_hex, created_at
		)
		VALUES (
			'chk_existing',
			'inc_existing',
			1,
			'audio',
			'2026-05-21T10:00:00Z',
			'2026-05-21T10:00:01Z',
			'incidents/inc_existing/audio_000001.enc',
			4,
			?,
			'2026-05-21T10:00:02Z'
		);`, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("insert legacy chunk before migration: %v", err)
	}
}

func assertChunkExists(t *testing.T, ctx context.Context, conn *sql.DB, chunkID string) {
	t.Helper()
	var count int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM chunks
		WHERE id = ?`, chunkID).Scan(&count); err != nil {
		t.Fatalf("count chunk %s: %v", chunkID, err)
	}
	if count != 1 {
		t.Fatalf("expected chunk %s to exist, got count %d", chunkID, count)
	}
}

func hasTable(t *testing.T, ctx context.Context, conn *sql.DB, tableName string) bool {
	t.Helper()
	var name string
	err := conn.QueryRowContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = ?`,
		tableName,
	).Scan(&name)
	if err == nil {
		return true
	}
	if err == sql.ErrNoRows {
		return false
	}
	t.Fatalf("query table %s: %v", tableName, err)
	return false
}

func assertJournalMode(t *testing.T, ctx context.Context, conn *sql.DB, want string) {
	t.Helper()
	var journalMode string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal mode: %v", err)
	}
	if !strings.EqualFold(journalMode, want) {
		t.Fatalf("journal mode = %q, want %q", journalMode, want)
	}
}

func appliedMigrationIDs(t *testing.T, ctx context.Context, conn *sql.DB) []string {
	t.Helper()
	rows, err := conn.QueryContext(ctx, `
		SELECT id
		FROM schema_migrations
		ORDER BY id`)
	if err != nil {
		t.Fatalf("query schema migrations: %v", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan migration id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration ids: %v", err)
	}
	return ids
}

func expectedMigrationIDs(t *testing.T) []string {
	t.Helper()
	steps, err := migrationSteps()
	if err != nil {
		t.Fatalf("migrationSteps: %v", err)
	}
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.id)
	}
	sort.Strings(ids)
	return ids
}

func hasColumn(t *testing.T, ctx context.Context, conn *sql.DB, tableName, columnName string) bool {
	t.Helper()
	hasColumn, err := tableHasColumn(ctx, conn, tableName, columnName)
	if err != nil {
		t.Fatalf("inspect %s.%s: %v", tableName, columnName, err)
	}
	return hasColumn
}
