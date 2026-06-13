package evidencebundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/envelope/pq"
	"github.com/open-proofline/server/internal/incidents"
)

func TestMakeStreamManifestKeepsCiphertextMetadata(t *testing.T) {
	completedAt := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	stream := incidents.MediaStream{
		ID:          "str_test",
		IncidentID:  "inc_test",
		MediaType:   incidents.MediaTypeAudio,
		Label:       "audio recording",
		Status:      incidents.StreamStatusComplete,
		CreatedAt:   completedAt.Add(-time.Minute),
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
	}
	chunks := []incidents.Chunk{
		{
			ChunkIndex:       1,
			MediaType:        incidents.MediaTypeAudio,
			ByteSize:         12,
			SHA256Hex:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			StartedAt:        completedAt.Add(-time.Minute),
			EndedAt:          completedAt,
			OriginalFilename: "chunk.enc",
		},
	}

	manifest := MakeStreamManifest(stream, chunks)

	if manifest.IncidentID != stream.IncidentID || manifest.StreamID != stream.ID || manifest.ChunkCount != 1 || manifest.TotalBytes != 12 {
		t.Fatalf("unexpected stream manifest: %+v", manifest)
	}
	if manifest.Encryption.Expected != "client-side" ||
		manifest.Encryption.Scheme != pq.SchemeID ||
		manifest.Encryption.SuiteID != pq.SuiteID ||
		manifest.Encryption.Profile != pq.ProfileID ||
		manifest.Encryption.ServerDecrypts {
		t.Fatalf("unexpected encryption hint: %+v", manifest.Encryption)
	}
	if got := manifest.Chunks[0].OriginalFilename; got != "chunk.enc" {
		t.Fatalf("original filename = %q, want chunk.enc", got)
	}
}

func TestValidStreamChunksRequiresContiguousMatchingMedia(t *testing.T) {
	expected := 2
	stream := incidents.MediaStream{
		MediaType:          incidents.MediaTypeAudio,
		ExpectedChunkCount: &expected,
	}
	valid := []incidents.Chunk{
		{ChunkIndex: 1, MediaType: incidents.MediaTypeAudio},
		{ChunkIndex: 2, MediaType: incidents.MediaTypeAudio},
	}
	if !ValidStreamChunks(stream, valid) {
		t.Fatal("expected contiguous matching chunks to be valid")
	}

	nonContiguous := []incidents.Chunk{
		{ChunkIndex: 1, MediaType: incidents.MediaTypeAudio},
		{ChunkIndex: 3, MediaType: incidents.MediaTypeAudio},
	}
	if ValidStreamChunks(stream, nonContiguous) {
		t.Fatal("expected non-contiguous chunks to be invalid")
	}

	wrongMedia := []incidents.Chunk{
		{ChunkIndex: 1, MediaType: incidents.MediaTypeAudio},
		{ChunkIndex: 2, MediaType: incidents.MediaTypeVideo},
	}
	if ValidStreamChunks(stream, wrongMedia) {
		t.Fatal("expected mixed-media chunks to be invalid")
	}
}

func TestVerifyChunkFileChecksCommittedBytesAgainstMetadata(t *testing.T) {
	payload := []byte("encrypted bytes")
	hash := sha256.Sum256(payload)
	chunk := incidents.Chunk{
		StoredPath: "incidents/inc_test/streams/str_test/audio_000001.enc",
		ByteSize:   int64(len(payload)),
		SHA256Hex:  hex.EncodeToString(hash[:]),
	}
	opener := func(context.Context, string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}

	if err := VerifyChunkFile(context.Background(), opener, chunk); err != nil {
		t.Fatalf("verify chunk file: %v", err)
	}

	chunk.SHA256Hex = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := VerifyChunkFile(context.Background(), opener, chunk); !errors.Is(err, ErrChunkIntegrityMismatch) {
		t.Fatalf("verify tampered chunk error = %v, want ErrChunkIntegrityMismatch", err)
	}
}
