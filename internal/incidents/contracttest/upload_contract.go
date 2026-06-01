package contracttest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

// Repository is the incident metadata contract exercised by the upload
// operation race and backend-parity tests.
type Repository interface {
	CreateIncident(ctx context.Context, clientLabel, notes string) (incidents.Incident, error)
	GetIncident(ctx context.Context, id string) (incidents.Incident, error)
	CloseIncident(ctx context.Context, id string) (incidents.Incident, error)
	CreateMediaStream(ctx context.Context, incidentID, mediaType, label string) (incidents.MediaStream, error)
	GetMediaStream(ctx context.Context, incidentID, streamID string) (incidents.MediaStream, error)
	ListCompletedMediaStreams(ctx context.Context, incidentID string) ([]incidents.MediaStream, error)
	CompleteMediaStream(ctx context.Context, incidentID, streamID string, expectedChunkCount int) (incidents.MediaStream, error)
	CreateChunk(ctx context.Context, params incidents.CreateChunkParams) (incidents.Chunk, error)
	ListStreamChunks(ctx context.Context, incidentID, streamID string) ([]incidents.Chunk, error)
	ReserveUploadOperation(ctx context.Context, params incidents.UploadOperationParams) (incidents.UploadOperation, error)
	CompleteUploadOperation(ctx context.Context, params incidents.UploadOperationParams, chunk incidents.Chunk) (incidents.UploadOperation, error)
	CreateIncidentToken(ctx context.Context, incidentID, label string, expiresAt *time.Time) (incidents.IncidentToken, string, error)
	LookupIncidentToken(ctx context.Context, rawToken string) (incidents.IncidentToken, error)
	RevokeIncidentToken(ctx context.Context, tokenID string) error
}

// NewRepository returns a fresh repository for one contract test.
type NewRepository func(t *testing.T, ctx context.Context) Repository

// RunUploadOperationRaceAndParity exercises the documented production-cluster
// expectations that SQLite must cover by default and PostgreSQL must cover when
// SAFE_POSTGRES_TEST_DSN is configured.
func RunUploadOperationRaceAndParity(t *testing.T, newRepository NewRepository) {
	t.Helper()

	t.Run("documented cluster expectation duplicate same identity upload race", func(t *testing.T) {
		ctx := context.Background()
		repo := newRepository(t, ctx)
		incident := mustCreateIncident(t, ctx, repo, "duplicate-race")
		stream := mustCreateStream(t, ctx, repo, incident.ID)

		params := testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1)
		start := make(chan struct{})
		var wg sync.WaitGroup
		results := make([]error, 2)
		for i := range results {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, results[i] = repo.CreateChunk(ctx, params)
			}()
		}
		close(start)
		wg.Wait()

		var created int
		var duplicated int
		for _, err := range results {
			switch {
			case err == nil:
				created++
			case errors.Is(err, incidents.ErrDuplicate):
				duplicated++
			default:
				t.Fatalf("create duplicate chunk race returned unexpected error: %v", err)
			}
		}
		if created != 1 || duplicated != 1 {
			t.Fatalf("duplicate upload race outcomes created=%d duplicated=%d, want 1/1", created, duplicated)
		}
		chunks, err := repo.ListStreamChunks(ctx, incident.ID, stream.ID)
		if err != nil {
			t.Fatalf("list stream chunks: %v", err)
		}
		if len(chunks) != 1 || chunks[0].ChunkIndex != 1 {
			t.Fatalf("duplicate upload race persisted chunks %+v, want exactly chunk 1", chunks)
		}
	})

	t.Run("documented cluster expectation upload versus incident close", func(t *testing.T) {
		ctx := context.Background()
		repo := newRepository(t, ctx)
		incident := mustCreateIncident(t, ctx, repo, "close-race")
		stream := mustCreateStream(t, ctx, repo, incident.ID)

		start := make(chan struct{})
		var wg sync.WaitGroup
		var closeErr error
		var chunkErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, closeErr = repo.CloseIncident(ctx, incident.ID)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, chunkErr = repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1))
		}()
		close(start)
		wg.Wait()

		if closeErr != nil {
			t.Fatalf("close incident during upload race: %v", closeErr)
		}
		switch {
		case chunkErr == nil:
			assertChunkCount(t, ctx, repo, incident.ID, stream.ID, 1)
		case errors.Is(chunkErr, incidents.ErrIncidentClosed):
			assertChunkCount(t, ctx, repo, incident.ID, stream.ID, 0)
		default:
			t.Fatalf("create chunk during close race error = %v, want success or ErrIncidentClosed", chunkErr)
		}
		got, err := repo.GetIncident(ctx, incident.ID)
		if err != nil {
			t.Fatalf("get incident after close race: %v", err)
		}
		if got.Status != incidents.StatusClosed {
			t.Fatalf("incident status after close race = %q, want closed", got.Status)
		}
	})

	t.Run("documented cluster expectation upload versus stream completion", func(t *testing.T) {
		ctx := context.Background()
		repo := newRepository(t, ctx)
		incident := mustCreateIncident(t, ctx, repo, "complete-race")
		stream := mustCreateStream(t, ctx, repo, incident.ID)
		if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1)); err != nil {
			t.Fatalf("create initial stream chunk: %v", err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		var completeErr error
		var chunkErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, completeErr = repo.CompleteMediaStream(ctx, incident.ID, stream.ID, 1)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, chunkErr = repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 2))
		}()
		close(start)
		wg.Wait()

		switch {
		case completeErr == nil:
			if !errors.Is(chunkErr, incidents.ErrInvalidState) {
				t.Fatalf("completion won race but late chunk error = %v, want ErrInvalidState", chunkErr)
			}
			assertChunkCount(t, ctx, repo, incident.ID, stream.ID, 1)
			assertStreamStatus(t, ctx, repo, incident.ID, stream.ID, incidents.StreamStatusComplete)
		case errors.Is(completeErr, incidents.ErrInvalidState):
			if chunkErr != nil {
				t.Fatalf("late chunk won race but create chunk error = %v", chunkErr)
			}
			assertChunkCount(t, ctx, repo, incident.ID, stream.ID, 2)
			assertStreamStatus(t, ctx, repo, incident.ID, stream.ID, incidents.StreamStatusOpen)
		default:
			t.Fatalf("complete stream race error = %v, want success or ErrInvalidState", completeErr)
		}
	})

	t.Run("documented cluster expectation idempotency replay and conflict parity", func(t *testing.T) {
		ctx := context.Background()
		repo := newRepository(t, ctx)
		incident := mustCreateIncident(t, ctx, repo, "idempotency")
		stream := mustCreateStream(t, ctx, repo, incident.ID)
		params := testUploadOperationParams(incident.ID, stream.ID, 1, strings.Repeat("b", 64), strings.Repeat("a", 64))
		params.StartedAt = params.StartedAt.Add(123456789 * time.Nanosecond)
		params.EndedAt = params.EndedAt.Add(123456789 * time.Nanosecond)

		reserved, err := repo.ReserveUploadOperation(ctx, params)
		if err != nil {
			t.Fatalf("reserve upload operation: %v", err)
		}
		if reserved.State != incidents.UploadOperationStateReserved {
			t.Fatalf("reserved operation state = %q, want reserved", reserved.State)
		}
		same, err := repo.ReserveUploadOperation(ctx, params)
		if err != nil {
			t.Fatalf("reserve same upload operation: %v", err)
		}
		if same.ID != reserved.ID {
			t.Fatalf("same idempotency key created a new operation: first=%q second=%q", reserved.ID, same.ID)
		}
		conflicting := params
		conflicting.OriginalFilename = "different.enc"
		conflicting.FingerprintHash = strings.Repeat("c", 64)
		if _, err := repo.ReserveUploadOperation(ctx, conflicting); !errors.Is(err, incidents.ErrIdempotencyConflict) {
			t.Fatalf("conflicting upload operation error = %v, want ErrIdempotencyConflict", err)
		}

		chunk, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1))
		if err != nil {
			t.Fatalf("create chunk for upload operation: %v", err)
		}
		completed, err := repo.CompleteUploadOperation(ctx, params, chunk)
		if err != nil {
			t.Fatalf("complete upload operation: %v", err)
		}
		if completed.State != incidents.UploadOperationStateMetadataCommitted || completed.ChunkID != chunk.ID {
			t.Fatalf("unexpected completed upload operation: %+v", completed)
		}
		replayed, err := repo.ReserveUploadOperation(ctx, params)
		if err != nil {
			t.Fatalf("reserve completed upload operation: %v", err)
		}
		if replayed.State != incidents.UploadOperationStateMetadataCommitted || replayed.ChunkID != chunk.ID {
			t.Fatalf("expected completed upload operation replay, got %+v", replayed)
		}
	})

	t.Run("documented cluster expectation token revocation parity", func(t *testing.T) {
		ctx := context.Background()
		repo := newRepository(t, ctx)
		incident := mustCreateIncident(t, ctx, repo, "token-revocation")

		token, rawToken, err := repo.CreateIncidentToken(ctx, incident.ID, "trusted contact", nil)
		if err != nil {
			t.Fatalf("create incident token: %v", err)
		}
		if rawToken == "" {
			t.Fatal("raw token was empty")
		}
		lookedUp, err := repo.LookupIncidentToken(ctx, rawToken)
		if err != nil {
			t.Fatalf("lookup incident token: %v", err)
		}
		if lookedUp.ID != token.ID {
			t.Fatalf("lookup token id = %q, want %q", lookedUp.ID, token.ID)
		}
		if err := repo.RevokeIncidentToken(ctx, token.ID); err != nil {
			t.Fatalf("revoke incident token: %v", err)
		}
		if _, err := repo.LookupIncidentToken(ctx, rawToken); !errors.Is(err, incidents.ErrNotFound) {
			t.Fatalf("lookup revoked token error = %v, want ErrNotFound", err)
		}
	})

	t.Run("documented cluster expectation completed stream metadata reconstructs bundle", func(t *testing.T) {
		ctx := context.Background()
		repo := newRepository(t, ctx)
		incident := mustCreateIncident(t, ctx, repo, "bundle-reconstruction")
		stream := mustCreateStream(t, ctx, repo, incident.ID)
		for index := 1; index <= 2; index++ {
			if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, index)); err != nil {
				t.Fatalf("create stream chunk %d: %v", index, err)
			}
		}
		if _, err := repo.CompleteMediaStream(ctx, incident.ID, stream.ID, 2); err != nil {
			t.Fatalf("complete stream: %v", err)
		}

		completed, err := repo.ListCompletedMediaStreams(ctx, incident.ID)
		if err != nil {
			t.Fatalf("list completed media streams: %v", err)
		}
		if len(completed) != 1 || completed[0].ID != stream.ID {
			t.Fatalf("completed streams = %+v, want only %q", completed, stream.ID)
		}
		chunks, err := repo.ListStreamChunks(ctx, incident.ID, stream.ID)
		if err != nil {
			t.Fatalf("list completed stream chunks: %v", err)
		}
		if len(chunks) != 2 {
			t.Fatalf("completed stream chunk count = %d, want 2", len(chunks))
		}
		for index, chunk := range chunks {
			wantIndex := index + 1
			wantPath := fmt.Sprintf("incidents/%s/streams/%s/audio_%06d.enc", incident.ID, stream.ID, wantIndex)
			if chunk.ChunkIndex != wantIndex || chunk.StoredPath != wantPath || chunk.SHA256Hex == "" || chunk.ByteSize <= 0 {
				t.Fatalf("completed stream chunk %d not reconstructable: %+v", wantIndex, chunk)
			}
		}
	})
}

func mustCreateIncident(t *testing.T, ctx context.Context, repo Repository, label string) incidents.Incident {
	t.Helper()
	incident, err := repo.CreateIncident(ctx, label, "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	return incident
}

func mustCreateStream(t *testing.T, ctx context.Context, repo Repository, incidentID string) incidents.MediaStream {
	t.Helper()
	stream, err := repo.CreateMediaStream(ctx, incidentID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	return stream
}

func assertChunkCount(t *testing.T, ctx context.Context, repo Repository, incidentID, streamID string, want int) {
	t.Helper()
	chunks, err := repo.ListStreamChunks(ctx, incidentID, streamID)
	if err != nil {
		t.Fatalf("list stream chunks: %v", err)
	}
	if len(chunks) != want {
		t.Fatalf("stream chunk count = %d, want %d: %+v", len(chunks), want, chunks)
	}
}

func assertStreamStatus(t *testing.T, ctx context.Context, repo Repository, incidentID, streamID, want string) {
	t.Helper()
	stream, err := repo.GetMediaStream(ctx, incidentID, streamID)
	if err != nil {
		t.Fatalf("get media stream: %v", err)
	}
	if stream.Status != want {
		t.Fatalf("stream status = %q, want %q", stream.Status, want)
	}
}

func testChunkParams(incidentID, streamID, mediaType string, chunkIndex int) incidents.CreateChunkParams {
	startedAt := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC).Add(time.Duration(chunkIndex) * time.Second)
	storedPath := fmt.Sprintf("incidents/%s/%s_%06d.enc", incidentID, mediaType, chunkIndex)
	if streamID != "" {
		storedPath = fmt.Sprintf("incidents/%s/streams/%s/%s_%06d.enc", incidentID, streamID, mediaType, chunkIndex)
	}
	return incidents.CreateChunkParams{
		IncidentID:       incidentID,
		StreamID:         streamID,
		ChunkIndex:       chunkIndex,
		MediaType:        mediaType,
		StartedAt:        startedAt,
		EndedAt:          startedAt.Add(time.Second),
		OriginalFilename: fmt.Sprintf("chunk-%06d.enc", chunkIndex),
		StoredPath:       storedPath,
		ByteSize:         int64(4 + chunkIndex),
		SHA256Hex:        strings.Repeat("a", 64),
	}
}

func testUploadOperationParams(incidentID, streamID string, chunkIndex int, keyHash, fingerprintHash string) incidents.UploadOperationParams {
	chunk := testChunkParams(incidentID, streamID, incidents.MediaTypeAudio, chunkIndex)
	return incidents.UploadOperationParams{
		Operation:          incidents.UploadOperationUploadChunk,
		IdempotencyKeyHash: keyHash,
		IncidentID:         incidentID,
		StreamID:           streamID,
		ChunkIndex:         chunk.ChunkIndex,
		MediaType:          chunk.MediaType,
		StartedAt:          chunk.StartedAt,
		EndedAt:            chunk.EndedAt,
		OriginalFilename:   chunk.OriginalFilename,
		ByteSize:           chunk.ByteSize,
		SHA256Hex:          chunk.SHA256Hex,
		FingerprintHash:    fingerprintHash,
	}
}
