package main

import (
	"bytes"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/envelope"
	pqenv "github.com/open-proofline/server/internal/envelope/pq"
)

const (
	envelopeModePQ = "pq"
	envelopeModeV1 = "v1"

	pqSimulatorKeyFileVersion = 1
)

type chunkUpload struct {
	incidentID     string
	streamID       string
	chunkIndex     int
	mediaType      string
	startedAt      time.Time
	endedAt        time.Time
	filename       string
	body           []byte
	sha256Hex      string
	idempotencyKey string
}

type simulatorEncryption struct {
	enabled bool
	mode    string

	v1Key envelope.Key

	pqRecipient        pqenv.Recipient
	pqDecapsulationKey *mlkem.DecapsulationKey768
	pqRecords          map[string][]pqenv.RecipientWrappingRecord
}

type pqSimulatorKeyFile struct {
	Version               int       `json:"version"`
	Scheme                string    `json:"scheme"`
	SuiteID               string    `json:"suite_id"`
	RecipientKeyID        string    `json:"recipient_key_id"`
	RecipientKeyVersion   int       `json:"recipient_key_version"`
	RecipientRole         string    `json:"recipient_role"`
	EncapsulationKeyB64U  string    `json:"encapsulation_key_b64u"`
	DecapsulationSeedB64U string    `json:"decapsulation_seed_b64u"`
	CreatedAt             time.Time `json:"created_at"`
}

func newChunkUpload(incidentID, streamID string, chunkIndex int, mediaType string, size int64, startedAt time.Time) (chunkUpload, error) {
	body, err := randomChunkBytes(size)
	if err != nil {
		return chunkUpload{}, err
	}
	return buildChunkUpload(incidentID, streamID, chunkIndex, mediaType, startedAt, body), nil
}

func newEncryptedChunkUpload(encryption simulatorEncryption, incidentID, streamID string, chunkIndex int, mediaType string, size int64, startedAt time.Time) (chunkUpload, error) {
	plaintext, err := randomChunkBytes(size)
	if err != nil {
		return chunkUpload{}, err
	}
	body, err := encryption.encryptChunk(incidentID, streamID, mediaType, chunkIndex, plaintext)
	if err != nil {
		return chunkUpload{}, fmt.Errorf("encrypt chunk: %w", err)
	}
	return buildChunkUpload(incidentID, streamID, chunkIndex, mediaType, startedAt, body), nil
}

func (e simulatorEncryption) encryptChunk(incidentID, streamID, mediaType string, chunkIndex int, plaintext []byte) ([]byte, error) {
	if !e.enabled {
		return append([]byte(nil), plaintext...), nil
	}
	switch e.mode {
	case envelopeModePQ:
		env, err := pqenv.Encrypt(plaintext, pqenv.PayloadContext{
			EnvelopeID:  simulatorPQEnvelopeID(incidentID, streamID, mediaType, chunkIndex),
			IncidentID:  incidentID,
			StreamID:    streamID,
			MediaType:   mediaType,
			ChunkIndex:  chunkIndex,
			PayloadType: pqenv.PayloadTypeChunk,
			MediaKeyID:  simulatorPQMediaKeyID(incidentID, streamID, mediaType, chunkIndex),
		}, []pqenv.Recipient{e.pqRecipient})
		if err != nil {
			return nil, err
		}
		e.pqRecords[chunkRecordKey(incidentID, streamID, mediaType, chunkIndex)] = env.Recipients
		return env.PayloadFrame, nil
	case envelopeModeV1:
		return envelope.EncryptChunk(e.v1Key, chunkContext(incidentID, streamID, mediaType, chunkIndex), plaintext)
	default:
		return nil, fmt.Errorf("unsupported envelope mode")
	}
}

func (e simulatorEncryption) decryptChunk(incidentID, streamID, mediaType string, chunkIndex int, ciphertext []byte) ([]byte, error) {
	if !e.enabled {
		return append([]byte(nil), ciphertext...), nil
	}
	switch e.mode {
	case envelopeModePQ:
		if e.pqDecapsulationKey == nil {
			return nil, fmt.Errorf("PQ decapsulation key is not configured")
		}
		records := e.pqRecords[chunkRecordKey(incidentID, streamID, mediaType, chunkIndex)]
		if len(records) == 0 {
			return nil, fmt.Errorf("PQ wrapping records are unavailable for this bundle")
		}
		return pqenv.Decrypt(pqenv.Envelope{
			PayloadFrame: append([]byte(nil), ciphertext...),
			Recipients:   records,
		}, pqenv.PayloadContext{
			EnvelopeID:  simulatorPQEnvelopeID(incidentID, streamID, mediaType, chunkIndex),
			IncidentID:  incidentID,
			StreamID:    streamID,
			MediaType:   mediaType,
			ChunkIndex:  chunkIndex,
			PayloadType: pqenv.PayloadTypeChunk,
			MediaKeyID:  simulatorPQMediaKeyID(incidentID, streamID, mediaType, chunkIndex),
		}, e.pqRecipient.KeyID, e.pqDecapsulationKey)
	case envelopeModeV1:
		return envelope.DecryptChunk(e.v1Key, chunkContext(incidentID, streamID, mediaType, chunkIndex), ciphertext)
	default:
		return nil, fmt.Errorf("unsupported envelope mode")
	}
}

func randomChunkBytes(size int64) ([]byte, error) {
	if size > int64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("chunk size is too large for this platform")
	}
	body := make([]byte, int(size))
	if _, err := rand.Read(body); err != nil {
		return nil, fmt.Errorf("generate fake chunk bytes: %w", err)
	}
	return body, nil
}

func buildChunkUpload(incidentID, streamID string, chunkIndex int, mediaType string, startedAt time.Time, body []byte) chunkUpload {
	sum := sha256.Sum256(body)
	chunkStartedAt := startedAt.Add(time.Duration(chunkIndex-1) * chunkDuration)
	return chunkUpload{
		incidentID:     incidentID,
		streamID:       streamID,
		chunkIndex:     chunkIndex,
		mediaType:      mediaType,
		startedAt:      chunkStartedAt,
		endedAt:        chunkStartedAt.Add(chunkDuration),
		filename:       fmt.Sprintf("%s_%06d.enc", mediaType, chunkIndex),
		body:           body,
		sha256Hex:      hex.EncodeToString(sum[:]),
		idempotencyKey: simulatorIdempotencyKey(incidentID, streamID, mediaType, chunkIndex),
	}
}

func simulatorIdempotencyKey(incidentID, streamID, mediaType string, chunkIndex int) string {
	if streamID == "" {
		streamID = "legacy"
	}
	return fmt.Sprintf("simclient-%s-%s-%s-%06d", incidentID, streamID, mediaType, chunkIndex)
}

func loadOrCreateSimulatorEncryption(cfg config) (simulatorEncryption, error) {
	if !cfg.encrypt {
		return simulatorEncryption{enabled: false}, nil
	}
	mode := cfg.envelopeMode
	if mode == "" {
		mode = envelopeModePQ
	}
	switch mode {
	case envelopeModePQ:
		recipient, dk, err := loadOrCreatePQSimulatorKey(cfg.keyFile)
		if err != nil {
			return simulatorEncryption{}, err
		}
		return simulatorEncryption{
			enabled:            true,
			mode:               envelopeModePQ,
			pqRecipient:        recipient,
			pqDecapsulationKey: dk,
			pqRecords:          map[string][]pqenv.RecipientWrappingRecord{},
		}, nil
	case envelopeModeV1:
		key, err := loadOrCreateSimulatorKey(cfg.keyFile)
		if err != nil {
			return simulatorEncryption{}, err
		}
		return newV1SimulatorEncryption(key), nil
	default:
		return simulatorEncryption{}, fmt.Errorf("unsupported envelope mode")
	}
}

func loadExistingSimulatorEncryption(cfg config) (simulatorEncryption, error) {
	if !cfg.encrypt {
		return simulatorEncryption{enabled: false}, nil
	}
	mode := cfg.envelopeMode
	if mode == "" {
		mode = envelopeModePQ
	}
	switch mode {
	case envelopeModePQ:
		return simulatorEncryption{}, fmt.Errorf("--verify-bundle currently requires --envelope v1 because PQ bundle manifests do not include wrapped-key ciphertext")
	case envelopeModeV1:
		key, err := loadExistingSimulatorKey(cfg.keyFile)
		if err != nil {
			return simulatorEncryption{}, err
		}
		return newV1SimulatorEncryption(key), nil
	default:
		return simulatorEncryption{}, fmt.Errorf("unsupported envelope mode")
	}
}

func newV1SimulatorEncryption(key envelope.Key) simulatorEncryption {
	return simulatorEncryption{
		enabled: true,
		mode:    envelopeModeV1,
		v1Key:   key,
	}
}

func loadOrCreateSimulatorKey(path string) (envelope.Key, error) {
	if path == "" {
		key, err := envelope.GenerateKey()
		if err != nil {
			return envelope.Key{}, fmt.Errorf("generate ephemeral encryption key: %w", err)
		}
		return key, nil
	}

	key, err := envelope.LoadKeyFile(path)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return envelope.Key{}, safePathError("load key file", err)
	}
	key, err = envelope.GenerateKey()
	if err != nil {
		return envelope.Key{}, fmt.Errorf("generate encryption key: %w", err)
	}
	if err := envelope.SaveKeyFile(path, key); err != nil {
		return envelope.Key{}, safePathError("save key file", err)
	}
	return key, nil
}

func loadOrCreatePQSimulatorKey(path string) (pqenv.Recipient, *mlkem.DecapsulationKey768, error) {
	if path == "" {
		recipient, dk, err := pqenv.GenerateRecipientKey(1)
		if err != nil {
			return pqenv.Recipient{}, nil, fmt.Errorf("generate ephemeral PQ recipient key: %w", err)
		}
		return recipient, dk, nil
	}

	recipient, dk, err := loadPQSimulatorKeyFile(path)
	if err == nil {
		return recipient, dk, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return pqenv.Recipient{}, nil, safePathError("load PQ simulator key file", err)
	}
	recipient, dk, err = pqenv.GenerateRecipientKey(1)
	if err != nil {
		return pqenv.Recipient{}, nil, fmt.Errorf("generate PQ simulator key: %w", err)
	}
	if err := savePQSimulatorKeyFile(path, recipient, dk); err != nil {
		return pqenv.Recipient{}, nil, err
	}
	return recipient, dk, nil
}

func loadPQSimulatorKeyFile(path string) (pqenv.Recipient, *mlkem.DecapsulationKey768, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return pqenv.Recipient{}, nil, err
	}
	var file pqSimulatorKeyFile
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return pqenv.Recipient{}, nil, fmt.Errorf("decode PQ simulator key file: %w", err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return pqenv.Recipient{}, nil, fmt.Errorf("decode PQ simulator key file: %w", err)
	}
	return validatePQSimulatorKeyFile(file)
}

func savePQSimulatorKeyFile(path string, recipient pqenv.Recipient, dk *mlkem.DecapsulationKey768) error {
	if dk == nil {
		return fmt.Errorf("PQ decapsulation key is required")
	}
	encapsulationKey := dk.EncapsulationKey().Bytes()
	if !bytes.Equal(encapsulationKey, recipient.EncapsulationKey) {
		return fmt.Errorf("PQ recipient does not match decapsulation key")
	}
	file := pqSimulatorKeyFile{
		Version:               pqSimulatorKeyFileVersion,
		Scheme:                pqenv.SchemeID,
		SuiteID:               pqenv.SuiteID,
		RecipientKeyID:        recipient.KeyID,
		RecipientKeyVersion:   recipient.KeyVersion,
		RecipientRole:         recipient.Role,
		EncapsulationKeyB64U:  base64.RawURLEncoding.EncodeToString(encapsulationKey),
		DecapsulationSeedB64U: base64.RawURLEncoding.EncodeToString(dk.Bytes()),
		CreatedAt:             time.Now().UTC(),
	}
	if _, _, err := validatePQSimulatorKeyFile(file); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return safePathError("create PQ simulator key file directory", err)
		}
	}
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode PQ simulator key file: %w", err)
	}
	body = append(body, '\n')
	if err := writeFileAtomicNoReplace(path, body, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("write PQ simulator key file: output file already exists")
		}
		return safePathError("write PQ simulator key file", err)
	}
	return nil
}

func validatePQSimulatorKeyFile(file pqSimulatorKeyFile) (pqenv.Recipient, *mlkem.DecapsulationKey768, error) {
	if file.Version != pqSimulatorKeyFileVersion {
		return pqenv.Recipient{}, nil, fmt.Errorf("unsupported PQ simulator key file version")
	}
	if file.Scheme != pqenv.SchemeID || file.SuiteID != pqenv.SuiteID {
		return pqenv.Recipient{}, nil, fmt.Errorf("unsupported PQ simulator key profile")
	}
	if file.RecipientKeyVersion <= 0 || strings.TrimSpace(file.RecipientRole) == "" {
		return pqenv.Recipient{}, nil, fmt.Errorf("PQ simulator key file is missing recipient metadata")
	}
	seed, err := decodeRawBase64URL("decapsulation_seed_b64u", file.DecapsulationSeedB64U, mlkem.SeedSize)
	if err != nil {
		return pqenv.Recipient{}, nil, err
	}
	encapsulationKey, err := decodeRawBase64URL("encapsulation_key_b64u", file.EncapsulationKeyB64U, mlkem.EncapsulationKeySize768)
	if err != nil {
		return pqenv.Recipient{}, nil, err
	}
	dk, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		return pqenv.Recipient{}, nil, fmt.Errorf("decode PQ decapsulation key: %w", err)
	}
	if !bytes.Equal(dk.EncapsulationKey().Bytes(), encapsulationKey) {
		return pqenv.Recipient{}, nil, fmt.Errorf("PQ simulator key public material does not match secret seed")
	}
	keyID, err := pqenv.RecipientKeyID(encapsulationKey, file.RecipientKeyVersion)
	if err != nil {
		return pqenv.Recipient{}, nil, err
	}
	if file.RecipientKeyID != keyID {
		return pqenv.Recipient{}, nil, fmt.Errorf("PQ simulator key ID does not match public material")
	}
	return pqenv.Recipient{
		KeyID:            file.RecipientKeyID,
		KeyVersion:       file.RecipientKeyVersion,
		Role:             file.RecipientRole,
		EncapsulationKey: encapsulationKey,
	}, dk, nil
}

func decodeRawBase64URL(field, value string, expectedSize int) ([]byte, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("%s is required", field)
	}
	if strings.Contains(value, "=") {
		return nil, fmt.Errorf("%s must use unpadded base64url", field)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", field, err)
	}
	if base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, fmt.Errorf("%s is not canonical base64url", field)
	}
	if len(decoded) != expectedSize {
		return nil, fmt.Errorf("%s must decode to %d bytes", field, expectedSize)
	}
	return decoded, nil
}

func chunkContext(incidentID, streamID, mediaType string, chunkIndex int) envelope.ChunkContext {
	return envelope.ChunkContext{
		IncidentID: incidentID,
		StreamID:   streamID,
		MediaType:  mediaType,
		ChunkIndex: chunkIndex,
	}
}

func simulatorPQEnvelopeID(incidentID, streamID, mediaType string, chunkIndex int) string {
	sum := sha256.Sum256([]byte(chunkRecordKey(incidentID, streamID, mediaType, chunkIndex)))
	return "env_pq_" + hex.EncodeToString(sum[:16])
}

func simulatorPQMediaKeyID(incidentID, streamID, mediaType string, chunkIndex int) string {
	sum := sha256.Sum256([]byte(chunkRecordKey(incidentID, streamID, mediaType, chunkIndex)))
	return "mk_pq_" + hex.EncodeToString(sum[:16])
}

func chunkRecordKey(incidentID, streamID, mediaType string, chunkIndex int) string {
	return strings.Join([]string{
		incidentID,
		streamID,
		mediaType,
		strconv.Itoa(chunkIndex),
	}, "\x00")
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') {
			continue
		}
		return false
	}
	return true
}

func shouldSimulateFailure(chunkIndex, every int) bool {
	return every > 0 && chunkIndex%every == 0
}

func shouldSendCheckin(chunkIndex int) bool {
	return chunkIndex == 1 || chunkIndex%defaultCheckinEvery == 0
}

func encryptionLogPrefix(enabled bool) string {
	if enabled {
		return "encrypted "
	}
	return ""
}

func badHashFor(hash string) string {
	if len(hash) != 64 {
		return strings.Repeat("0", 64)
	}
	if hash[0] == '0' {
		return "1" + hash[1:]
	}
	return "0" + hash[1:]
}

func validMediaType(mediaType string) bool {
	switch mediaType {
	case "audio", "video", "location", "metadata":
		return true
	default:
		return false
	}
}
