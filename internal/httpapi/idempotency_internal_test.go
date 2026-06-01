package httpapi

import (
	"testing"
	"time"

	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
)

func TestUploadMatchesChunkAllowsDatabaseTimePrecisionLoss(t *testing.T) {
	startedAt := time.Date(2026, 6, 1, 10, 0, 0, 123456789, time.UTC)
	endedAt := startedAt.Add(time.Second)
	upload := chunkUpload{
		temp:             &storage.TempUpload{ByteSize: 12},
		streamID:         "str_test",
		chunkIndex:       1,
		mediaType:        incidents.MediaTypeAudio,
		startedAt:        startedAt,
		endedAt:          endedAt,
		sha256Hex:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		originalFilename: "chunk.enc",
	}
	chunk := incidents.Chunk{
		IncidentID:       "inc_test",
		StreamID:         upload.streamID,
		ChunkIndex:       upload.chunkIndex,
		MediaType:        upload.mediaType,
		StartedAt:        startedAt.Add(500 * time.Nanosecond),
		EndedAt:          endedAt.Add(-500 * time.Nanosecond),
		OriginalFilename: upload.originalFilename,
		ByteSize:         upload.temp.ByteSize,
		SHA256Hex:        upload.sha256Hex,
	}

	if !uploadMatchesChunk("inc_test", upload, chunk) {
		t.Fatal("expected upload to match chunk within database time precision")
	}

	chunk.EndedAt = endedAt.Add(time.Microsecond)
	if uploadMatchesChunk("inc_test", upload, chunk) {
		t.Fatal("expected upload time outside precision tolerance to mismatch")
	}
}
