package httpapi

import (
	"time"

	"github.com/open-proofline/server/internal/envelope/pq"
	"github.com/open-proofline/server/internal/incidents"
)

type streamBundleData struct {
	Stream   incidents.MediaStream
	Chunks   []incidents.Chunk
	Manifest streamBundleManifest
}

type streamBundleManifest struct {
	IncidentID  string                `json:"incident_id"`
	StreamID    string                `json:"stream_id"`
	MediaType   string                `json:"media_type"`
	Label       string                `json:"label,omitempty"`
	Status      string                `json:"status"`
	Encryption  bundleEncryptionHint  `json:"encryption"`
	CreatedAt   time.Time             `json:"created_at"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
	ChunkCount  int                   `json:"chunk_count"`
	TotalBytes  int64                 `json:"total_bytes"`
	Chunks      []bundleChunkManifest `json:"chunks"`
}

type bundleEncryptionHint struct {
	Expected       string `json:"expected"`
	Scheme         string `json:"scheme"`
	SuiteID        string `json:"suite_id"`
	Profile        string `json:"profile"`
	ServerDecrypts bool   `json:"server_decrypts"`
}

type bundleChunkManifest struct {
	ChunkIndex       int       `json:"chunk_index"`
	MediaType        string    `json:"media_type"`
	ByteSize         int64     `json:"byte_size"`
	SHA256Hex        string    `json:"sha256_hex"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	OriginalFilename string    `json:"original_filename,omitempty"`
}

type incidentBundleManifest struct {
	Incident      incidentViewerIncidentSummary `json:"incident"`
	LatestCheckin *incidentViewerCheckinSummary `json:"latest_checkin,omitempty"`
	Encryption    bundleEncryptionHint          `json:"encryption"`
	Streams       []streamBundleManifest        `json:"streams"`
	StreamCount   int                           `json:"stream_count"`
	TotalBytes    int64                         `json:"total_bytes"`
	GeneratedAt   time.Time                     `json:"generated_at"`
}

func makeStreamBundleManifest(stream incidents.MediaStream, chunks []incidents.Chunk) streamBundleManifest {
	manifestChunks := make([]bundleChunkManifest, 0, len(chunks))
	var totalBytes int64
	for _, chunk := range chunks {
		totalBytes += chunk.ByteSize
		manifestChunks = append(manifestChunks, bundleChunkManifest{
			ChunkIndex:       chunk.ChunkIndex,
			MediaType:        chunk.MediaType,
			ByteSize:         chunk.ByteSize,
			SHA256Hex:        chunk.SHA256Hex,
			StartedAt:        chunk.StartedAt,
			EndedAt:          chunk.EndedAt,
			OriginalFilename: chunk.OriginalFilename,
		})
	}
	return streamBundleManifest{
		IncidentID:  stream.IncidentID,
		StreamID:    stream.ID,
		MediaType:   stream.MediaType,
		Label:       stream.Label,
		Status:      stream.Status,
		Encryption:  clientSideEncryptionHint(),
		CreatedAt:   stream.CreatedAt,
		CompletedAt: stream.CompletedAt,
		ChunkCount:  len(chunks),
		TotalBytes:  totalBytes,
		Chunks:      manifestChunks,
	}
}

func makeIncidentBundleManifest(detail incidents.IncidentDetail, bundles []streamBundleData) incidentBundleManifest {
	manifests := make([]streamBundleManifest, 0, len(bundles))
	var totalBytes int64
	for _, bundle := range bundles {
		manifests = append(manifests, bundle.Manifest)
		totalBytes += bundle.Manifest.TotalBytes
	}
	return incidentBundleManifest{
		Incident:      summarizeIncident(detail.Incident),
		LatestCheckin: summarizeLatestCheckin(detail.Checkins),
		Encryption:    clientSideEncryptionHint(),
		Streams:       manifests,
		StreamCount:   len(manifests),
		TotalBytes:    totalBytes,
		GeneratedAt:   time.Now().UTC(),
	}
}

func clientSideEncryptionHint() bundleEncryptionHint {
	return bundleEncryptionHint{
		Expected:       "client-side",
		Scheme:         pq.SchemeID,
		SuiteID:        pq.SuiteID,
		Profile:        pq.ProfileID,
		ServerDecrypts: false,
	}
}
