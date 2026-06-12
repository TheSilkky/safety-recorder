package envelope

const (
	SchemeV1           = "proofline-chunk-encryption-v1"
	AlgorithmAES256GCM = "AES-256-GCM"

	keyVersion      = 1
	keySize         = 32
	nonceSize       = 12
	maxHeaderLength = 16 * 1024

	magic = "PLCHNK1\n"
)

// ChunkContext is the metadata bound to a chunk through AES-GCM associated data.
type ChunkContext struct {
	IncidentID string
	StreamID   string
	MediaType  string
	ChunkIndex int
}

// Key is a simulator/development AES-256-GCM key.
type Key struct {
	Version   int
	Scheme    string
	Algorithm string
	KeyID     string
	Key       []byte
}

// Header is the non-secret JSON header stored inside an encrypted chunk envelope.
type Header struct {
	Version   int    `json:"version"`
	Scheme    string `json:"scheme"`
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	NonceB64  string `json:"nonce_b64"`
	AAD       string `json:"aad"`
}
