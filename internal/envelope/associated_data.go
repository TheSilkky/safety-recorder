package envelope

import (
	"fmt"
	"strings"
)

// BuildAssociatedData returns the exact UTF-8 AEAD associated data for a chunk.
func BuildAssociatedData(ctx ChunkContext) ([]byte, error) {
	if err := rejectNewline("incident_id", ctx.IncidentID); err != nil {
		return nil, err
	}
	if err := rejectNewline("stream_id", ctx.StreamID); err != nil {
		return nil, err
	}
	if err := rejectNewline("media_type", ctx.MediaType); err != nil {
		return nil, err
	}
	if ctx.ChunkIndex <= 0 {
		return nil, fmt.Errorf("chunk_index must be positive")
	}
	return []byte(fmt.Sprintf(
		"ProoflineChunk:v1\nincident_id=%s\nstream_id=%s\nmedia_type=%s\nchunk_index=%d\n",
		ctx.IncidentID,
		ctx.StreamID,
		ctx.MediaType,
		ctx.ChunkIndex,
	)), nil
}

func rejectNewline(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain newlines", field)
	}
	return nil
}
