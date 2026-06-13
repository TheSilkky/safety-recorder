package evidencebundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/open-proofline/server/internal/envelope/pq"
	"github.com/open-proofline/server/internal/incidents"
)

// ErrChunkIntegrityMismatch means a committed chunk no longer matches the
// trusted metadata used to build an encrypted evidence bundle.
var ErrChunkIntegrityMismatch = errors.New("bundle chunk integrity mismatch")

// ChunkOpener opens a committed encrypted chunk by its server-controlled stored
// path. Callers keep storage ownership outside this package.
type ChunkOpener func(ctx context.Context, storedPath string) (io.ReadCloser, error)

type StreamData struct {
	Stream   incidents.MediaStream
	Chunks   []incidents.Chunk
	Manifest StreamManifest
}

type StreamManifest struct {
	IncidentID  string          `json:"incident_id"`
	StreamID    string          `json:"stream_id"`
	MediaType   string          `json:"media_type"`
	Label       string          `json:"label,omitempty"`
	Status      string          `json:"status"`
	Encryption  EncryptionHint  `json:"encryption"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	ChunkCount  int             `json:"chunk_count"`
	TotalBytes  int64           `json:"total_bytes"`
	Chunks      []ChunkManifest `json:"chunks"`
}

type EncryptionHint struct {
	Expected       string `json:"expected"`
	Scheme         string `json:"scheme"`
	SuiteID        string `json:"suite_id"`
	Profile        string `json:"profile"`
	ServerDecrypts bool   `json:"server_decrypts"`
}

type ChunkManifest struct {
	ChunkIndex       int       `json:"chunk_index"`
	MediaType        string    `json:"media_type"`
	ByteSize         int64     `json:"byte_size"`
	SHA256Hex        string    `json:"sha256_hex"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	OriginalFilename string    `json:"original_filename,omitempty"`
}

type IncidentManifest struct {
	Incident      IncidentSummary  `json:"incident"`
	LatestCheckin *CheckinSummary  `json:"latest_checkin,omitempty"`
	Encryption    EncryptionHint   `json:"encryption"`
	Streams       []StreamManifest `json:"streams"`
	StreamCount   int              `json:"stream_count"`
	TotalBytes    int64            `json:"total_bytes"`
	GeneratedAt   time.Time        `json:"generated_at"`
}

type IncidentSummary struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	ClientLabel string    `json:"client_label,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CheckinSummary struct {
	CreatedAt            time.Time `json:"created_at"`
	DeviceBatteryPercent *int      `json:"device_battery_percent,omitempty"`
	DeviceNetwork        *string   `json:"device_network,omitempty"`
	Latitude             *float64  `json:"latitude,omitempty"`
	Longitude            *float64  `json:"longitude,omitempty"`
	AccuracyMeters       *float64  `json:"accuracy_meters,omitempty"`
}

func NewStreamData(stream incidents.MediaStream, chunks []incidents.Chunk) StreamData {
	return StreamData{
		Stream:   stream,
		Chunks:   chunks,
		Manifest: MakeStreamManifest(stream, chunks),
	}
}

func MakeStreamManifest(stream incidents.MediaStream, chunks []incidents.Chunk) StreamManifest {
	manifestChunks := make([]ChunkManifest, 0, len(chunks))
	var totalBytes int64
	for _, chunk := range chunks {
		totalBytes += chunk.ByteSize
		manifestChunks = append(manifestChunks, ChunkManifest{
			ChunkIndex:       chunk.ChunkIndex,
			MediaType:        chunk.MediaType,
			ByteSize:         chunk.ByteSize,
			SHA256Hex:        chunk.SHA256Hex,
			StartedAt:        chunk.StartedAt,
			EndedAt:          chunk.EndedAt,
			OriginalFilename: chunk.OriginalFilename,
		})
	}
	return StreamManifest{
		IncidentID:  stream.IncidentID,
		StreamID:    stream.ID,
		MediaType:   stream.MediaType,
		Label:       stream.Label,
		Status:      stream.Status,
		Encryption:  ClientSideEncryptionHint(),
		CreatedAt:   stream.CreatedAt,
		CompletedAt: stream.CompletedAt,
		ChunkCount:  len(chunks),
		TotalBytes:  totalBytes,
		Chunks:      manifestChunks,
	}
}

func MakeIncidentManifest(detail incidents.IncidentDetail, streams []StreamData) IncidentManifest {
	manifests := make([]StreamManifest, 0, len(streams))
	var totalBytes int64
	for _, stream := range streams {
		manifests = append(manifests, stream.Manifest)
		totalBytes += stream.Manifest.TotalBytes
	}
	return IncidentManifest{
		Incident:      SummarizeIncident(detail.Incident),
		LatestCheckin: SummarizeLatestCheckin(detail.Checkins),
		Encryption:    ClientSideEncryptionHint(),
		Streams:       manifests,
		StreamCount:   len(manifests),
		TotalBytes:    totalBytes,
		GeneratedAt:   time.Now().UTC(),
	}
}

func ClientSideEncryptionHint() EncryptionHint {
	return EncryptionHint{
		Expected:       "client-side",
		Scheme:         pq.SchemeID,
		SuiteID:        pq.SuiteID,
		Profile:        pq.ProfileID,
		ServerDecrypts: false,
	}
}

func SummarizeIncident(incident incidents.Incident) IncidentSummary {
	return IncidentSummary{
		ID:          incident.ID,
		Status:      incident.Status,
		ClientLabel: incident.ClientLabel,
		CreatedAt:   incident.CreatedAt,
		UpdatedAt:   incident.UpdatedAt,
	}
}

func SummarizeLatestCheckin(checkins []incidents.Checkin) *CheckinSummary {
	if len(checkins) == 0 {
		return nil
	}
	checkin := checkins[len(checkins)-1]
	return &CheckinSummary{
		CreatedAt:            checkin.CreatedAt,
		DeviceBatteryPercent: checkin.DeviceBatteryPercent,
		DeviceNetwork:        checkin.DeviceNetwork,
		Latitude:             checkin.Latitude,
		Longitude:            checkin.Longitude,
		AccuracyMeters:       checkin.AccuracyMeters,
	}
}

func ValidStreamChunks(stream incidents.MediaStream, chunks []incidents.Chunk) bool {
	if stream.ExpectedChunkCount != nil && len(chunks) != *stream.ExpectedChunkCount {
		return false
	}
	for i, chunk := range chunks {
		if chunk.ChunkIndex != i+1 || chunk.MediaType != stream.MediaType {
			return false
		}
	}
	return true
}

func VerifyChunkFiles(ctx context.Context, opener ChunkOpener, chunks []incidents.Chunk) error {
	for _, chunk := range chunks {
		if err := VerifyChunkFile(ctx, opener, chunk); err != nil {
			return err
		}
	}
	return nil
}

func VerifyChunkFile(ctx context.Context, opener ChunkOpener, chunk incidents.Chunk) error {
	file, err := opener(ctx, chunk.StoredPath)
	if err != nil {
		return fmt.Errorf("open chunk for bundle verification: %w", err)
	}

	hash := sha256.New()
	size, readErr := io.Copy(hash, file)
	closeErr := file.Close()
	if readErr != nil {
		return fmt.Errorf("read chunk for bundle verification: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close chunk for bundle verification: %w", closeErr)
	}
	if size != chunk.ByteSize || hex.EncodeToString(hash.Sum(nil)) != chunk.SHA256Hex {
		return ErrChunkIntegrityMismatch
	}
	return nil
}

func IsChunkVerificationFailure(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, ErrChunkIntegrityMismatch)
}
