package pq

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	SchemeID  = "proofline-pq-envelope-v1"
	SuiteID   = "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1"
	ProfileID = SuiteID

	WrappingAlgorithm        = "proofline-pq-mlkem768-hkdfsha384-aes256gcm"
	WrappingAlgorithmVersion = 1
	KEMID                    = "ML-KEM-768"
	KDFID                    = "HKDF-SHA384"
	AEADID                   = "AES-256-GCM"
	DigestID                 = "SHA-384"
	HKDFInfoID               = "proofline-cek-wrap-v1"

	payloadMagic    = "PLPQENC1\n"
	wrappedKeyMagic = "PLPQWK1\n"
	version         = 1

	cekSize                 = 32
	nonceSize               = 12
	hkdfSaltSize            = 48
	wrappedCEKSize          = cekSize + 16
	maxHeaderLength         = 16 * 1024
	MaxRecipientsPerCEK     = 16
	recipientKeyIDPrefix    = "pqk1_"
	maxCanonicalNameLength  = 1<<16 - 1
	maxCanonicalValueLength = 1<<32 - 1
)

var publicMetadataMandatory = []string{
	"profile",
	"scheme",
	"suite_id",
	"recipient_key_id",
	"recipient_key_version",
	"recipient_role",
	"media_key_id",
	"envelope_id",
	"payload_header_digest_b64u",
	"kem_ciphertext_digest_b64u",
	"hkdf_salt_b64u",
	"hkdf_info_id",
	"cek_wrap_aad_digest_b64u",
}

// PayloadContext is the non-secret context authenticated by the prototype
// payload and CEK wrapping AEAD operations.
type PayloadContext struct {
	EnvelopeID  string
	IncidentID  string
	StreamID    string
	MediaType   string
	ChunkIndex  int
	PayloadType string
	MediaKeyID  string
}

// Recipient describes one recipient public key that should receive a wrapped
// CEK in the prototype envelope.
type Recipient struct {
	KeyID            string
	KeyVersion       int
	Role             string
	EncapsulationKey []byte
}

// Envelope is the isolated prototype container returned by Encrypt.
type Envelope struct {
	PayloadFrame []byte
	Recipients   []RecipientWrappingRecord
}

// RecipientWrappingRecord carries the public metadata and wrapped CEK frame for
// one recipient.
type RecipientWrappingRecord struct {
	Metadata        PublicWrappingMetadata
	WrappedKeyFrame []byte
}

// PublicWrappingMetadata is the JSON-shaped public metadata defined for the
// accepted profile. The prototype keeps it as a struct so tests can tamper with
// individual fields without relying on ad hoc JSON stringification.
type PublicWrappingMetadata struct {
	Profile                 string   `json:"profile"`
	Scheme                  string   `json:"scheme"`
	SuiteID                 string   `json:"suite_id"`
	Mandatory               []string `json:"mandatory"`
	RecipientKeyID          string   `json:"recipient_key_id"`
	RecipientKeyVersion     int      `json:"recipient_key_version"`
	RecipientRole           string   `json:"recipient_role"`
	MediaKeyID              string   `json:"media_key_id"`
	EnvelopeID              string   `json:"envelope_id"`
	PayloadHeaderDigestB64U string   `json:"payload_header_digest_b64u"`
	KEMCiphertextDigestB64U string   `json:"kem_ciphertext_digest_b64u"`
	HKDFSaltB64U            string   `json:"hkdf_salt_b64u"`
	HKDFInfoID              string   `json:"hkdf_info_id"`
	CEKWrapAADDigestB64U    string   `json:"cek_wrap_aad_digest_b64u"`
}

type payloadHeader struct {
	Scheme       string
	SuiteID      string
	Digest       string
	EnvelopeID   string
	IncidentID   string
	StreamID     string
	MediaType    string
	ChunkIndex   int
	PayloadType  string
	PayloadAEAD  string
	PayloadNonce []byte
}

type wrappedKeyFrame struct {
	KEMCiphertext []byte
	WrapNonce     []byte
	WrappedCEK    []byte
}

type encryptOptions struct {
	randomReader io.Reader
	encapsulate  func(*mlkem.EncapsulationKey768) ([]byte, []byte, error)
}

// GenerateRecipientKey creates a local fixture recipient key for prototype
// round trips. The returned decapsulation key is secret test/client-side
// material and must not be sent to the server.
func GenerateRecipientKey(keyVersion int) (Recipient, *mlkem.DecapsulationKey768, error) {
	if keyVersion <= 0 {
		return Recipient{}, nil, fmt.Errorf("recipient key version must be positive")
	}
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		return Recipient{}, nil, fmt.Errorf("generate ML-KEM recipient key: %w", err)
	}
	ekBytes := dk.EncapsulationKey().Bytes()
	keyID, err := RecipientKeyID(ekBytes, keyVersion)
	if err != nil {
		return Recipient{}, nil, err
	}
	return Recipient{
		KeyID:            keyID,
		KeyVersion:       keyVersion,
		Role:             "trusted_contact",
		EncapsulationKey: cloneBytes(ekBytes),
	}, dk, nil
}

// RecipientKeyID derives the accepted non-secret recipient key identifier from
// a canonical public-key record.
func RecipientKeyID(encapsulationKey []byte, keyVersion int) (string, error) {
	if keyVersion <= 0 {
		return "", fmt.Errorf("recipient key version must be positive")
	}
	if len(encapsulationKey) != mlkem.EncapsulationKeySize768 {
		return "", fmt.Errorf("encapsulation key must be %d bytes", mlkem.EncapsulationKeySize768)
	}
	canonical, err := encodeCanonicalFields([]canonicalField{
		stringField("scheme", SchemeID),
		stringField("kem", KEMID),
		stringField("digest", DigestID),
		bytesField("encoded_encapsulation_key", encapsulationKey),
		intField("key_version", keyVersion),
	})
	if err != nil {
		return "", err
	}
	digest := sha384(canonical)
	return recipientKeyIDPrefix + base64.RawURLEncoding.EncodeToString(digest), nil
}

// Encrypt creates an isolated prototype envelope for local tests and vectors.
func Encrypt(plaintext []byte, ctx PayloadContext, recipients []Recipient) (Envelope, error) {
	return encryptWithOptions(plaintext, ctx, recipients, encryptOptions{})
}

// Decrypt unwraps and decrypts an isolated prototype envelope for one
// recipient. It is intended for tests and conformance checks, not server-side
// runtime decryption.
func Decrypt(env Envelope, ctx PayloadContext, recipientKeyID string, dk *mlkem.DecapsulationKey768) ([]byte, error) {
	if dk == nil {
		return nil, fmt.Errorf("decapsulation key is required")
	}
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateRecipientKeyID(recipientKeyID); err != nil {
		return nil, err
	}
	header, headerBytes, payloadCiphertext, err := parsePayloadFrame(env.PayloadFrame)
	if err != nil {
		return nil, err
	}
	if err := header.matchesContext(ctx); err != nil {
		return nil, err
	}
	payloadHeaderDigest := sha384(headerBytes)
	for _, record := range env.Recipients {
		if record.Metadata.RecipientKeyID != recipientKeyID {
			continue
		}
		frame, err := parseWrappedKeyFrame(record.WrappedKeyFrame)
		if err != nil {
			return nil, err
		}
		salt, wrapAAD, err := validatePublicMetadata(record.Metadata, ctx, header, payloadHeaderDigest, frame)
		if err != nil {
			return nil, err
		}
		sharedSecret, err := dk.Decapsulate(frame.KEMCiphertext)
		if err != nil {
			return nil, fmt.Errorf("decapsulate CEK wrapping key: %w", err)
		}
		info, err := hkdfInfo(header.EnvelopeID, recipientKeyID, sha384(frame.KEMCiphertext), payloadHeaderDigest)
		if err != nil {
			return nil, err
		}
		kek, err := deriveKEK(sharedSecret, salt, info)
		if err != nil {
			return nil, err
		}
		wrapAEAD, err := newAESGCM(kek)
		if err != nil {
			return nil, err
		}
		cek, err := wrapAEAD.Open(nil, frame.WrapNonce, frame.WrappedCEK, wrapAAD)
		if err != nil {
			return nil, fmt.Errorf("unwrap CEK: %w", err)
		}
		if len(cek) != cekSize {
			return nil, fmt.Errorf("unwrapped CEK has unsupported size")
		}
		payloadAEAD, err := newAESGCM(cek)
		if err != nil {
			return nil, err
		}
		plaintext, err := payloadAEAD.Open(nil, header.PayloadNonce, payloadCiphertext, headerBytes)
		if err != nil {
			return nil, fmt.Errorf("decrypt payload: %w", err)
		}
		return plaintext, nil
	}
	return nil, fmt.Errorf("recipient key not found")
}

func encryptWithOptions(plaintext []byte, ctx PayloadContext, recipients []Recipient, opts encryptOptions) (Envelope, error) {
	if err := validateContext(ctx); err != nil {
		return Envelope{}, err
	}
	if len(recipients) == 0 {
		return Envelope{}, fmt.Errorf("at least one recipient is required")
	}
	if len(recipients) > MaxRecipientsPerCEK {
		return Envelope{}, fmt.Errorf("recipient count exceeds %d", MaxRecipientsPerCEK)
	}
	reader := opts.randomReader
	if reader == nil {
		reader = rand.Reader
	}
	encapsulate := opts.encapsulate
	if encapsulate == nil {
		encapsulate = func(ek *mlkem.EncapsulationKey768) ([]byte, []byte, error) {
			sharedSecret, ciphertext := ek.Encapsulate()
			return sharedSecret, ciphertext, nil
		}
	}

	cek, err := readRandom(reader, cekSize)
	if err != nil {
		return Envelope{}, fmt.Errorf("generate CEK: %w", err)
	}
	payloadNonce, err := readRandom(reader, nonceSize)
	if err != nil {
		return Envelope{}, fmt.Errorf("generate payload nonce: %w", err)
	}
	header := payloadHeader{
		Scheme:       SchemeID,
		SuiteID:      SuiteID,
		Digest:       DigestID,
		EnvelopeID:   ctx.EnvelopeID,
		IncidentID:   ctx.IncidentID,
		StreamID:     ctx.StreamID,
		MediaType:    ctx.MediaType,
		ChunkIndex:   ctx.ChunkIndex,
		PayloadType:  ctx.PayloadType,
		PayloadAEAD:  AEADID,
		PayloadNonce: payloadNonce,
	}
	headerBytes, err := encodePayloadHeader(header)
	if err != nil {
		return Envelope{}, err
	}
	payloadAEAD, err := newAESGCM(cek)
	if err != nil {
		return Envelope{}, err
	}
	payloadCiphertext := payloadAEAD.Seal(nil, payloadNonce, plaintext, headerBytes)
	payloadFrame, err := encodePayloadFrame(headerBytes, payloadCiphertext)
	if err != nil {
		return Envelope{}, err
	}
	payloadHeaderDigest := sha384(headerBytes)

	records := make([]RecipientWrappingRecord, 0, len(recipients))
	seenRecipientKeyIDs := make(map[string]bool, len(recipients))
	for _, recipient := range recipients {
		normalized, err := normalizeRecipient(recipient)
		if err != nil {
			return Envelope{}, err
		}
		if seenRecipientKeyIDs[normalized.KeyID] {
			return Envelope{}, fmt.Errorf("duplicate recipient key ID")
		}
		seenRecipientKeyIDs[normalized.KeyID] = true
		ek, err := mlkem.NewEncapsulationKey768(normalized.EncapsulationKey)
		if err != nil {
			return Envelope{}, fmt.Errorf("parse recipient encapsulation key: %w", err)
		}
		sharedSecret, kemCiphertext, err := encapsulate(ek)
		if err != nil {
			return Envelope{}, fmt.Errorf("encapsulate recipient key: %w", err)
		}
		if len(sharedSecret) != mlkem.SharedKeySize {
			return Envelope{}, fmt.Errorf("ML-KEM shared secret has unsupported size")
		}
		if len(kemCiphertext) != mlkem.CiphertextSize768 {
			return Envelope{}, fmt.Errorf("ML-KEM ciphertext has unsupported size")
		}
		salt, err := readRandom(reader, hkdfSaltSize)
		if err != nil {
			return Envelope{}, fmt.Errorf("generate HKDF salt: %w", err)
		}
		wrapNonce, err := readRandom(reader, nonceSize)
		if err != nil {
			return Envelope{}, fmt.Errorf("generate CEK-wrap nonce: %w", err)
		}
		kemCiphertextDigest := sha384(kemCiphertext)
		info, err := hkdfInfo(header.EnvelopeID, normalized.KeyID, kemCiphertextDigest, payloadHeaderDigest)
		if err != nil {
			return Envelope{}, err
		}
		kek, err := deriveKEK(sharedSecret, salt, info)
		if err != nil {
			return Envelope{}, err
		}
		wrapAAD, err := cekWrapAAD(ctx, header.EnvelopeID, normalized, payloadHeaderDigest)
		if err != nil {
			return Envelope{}, err
		}
		wrapAADDigest := sha384(wrapAAD)
		wrapAEAD, err := newAESGCM(kek)
		if err != nil {
			return Envelope{}, err
		}
		wrappedCEK := wrapAEAD.Seal(nil, wrapNonce, cek, wrapAAD)
		wrappedFrame, err := encodeWrappedKeyFrame(kemCiphertext, wrapNonce, wrappedCEK)
		if err != nil {
			return Envelope{}, err
		}
		records = append(records, RecipientWrappingRecord{
			Metadata: PublicWrappingMetadata{
				Profile:                 ProfileID,
				Scheme:                  SchemeID,
				SuiteID:                 SuiteID,
				Mandatory:               cloneStrings(publicMetadataMandatory),
				RecipientKeyID:          normalized.KeyID,
				RecipientKeyVersion:     normalized.KeyVersion,
				RecipientRole:           normalized.Role,
				MediaKeyID:              ctx.MediaKeyID,
				EnvelopeID:              ctx.EnvelopeID,
				PayloadHeaderDigestB64U: base64.RawURLEncoding.EncodeToString(payloadHeaderDigest),
				KEMCiphertextDigestB64U: base64.RawURLEncoding.EncodeToString(kemCiphertextDigest),
				HKDFSaltB64U:            base64.RawURLEncoding.EncodeToString(salt),
				HKDFInfoID:              HKDFInfoID,
				CEKWrapAADDigestB64U:    base64.RawURLEncoding.EncodeToString(wrapAADDigest),
			},
			WrappedKeyFrame: wrappedFrame,
		})
	}
	return Envelope{
		PayloadFrame: cloneBytes(payloadFrame),
		Recipients:   records,
	}, nil
}

func normalizeRecipient(recipient Recipient) (Recipient, error) {
	if recipient.KeyVersion <= 0 {
		return Recipient{}, fmt.Errorf("recipient key version must be positive")
	}
	if recipient.Role == "" {
		return Recipient{}, fmt.Errorf("recipient role is required")
	}
	if err := rejectNewline("recipient role", recipient.Role); err != nil {
		return Recipient{}, err
	}
	if len(recipient.EncapsulationKey) != mlkem.EncapsulationKeySize768 {
		return Recipient{}, fmt.Errorf("recipient encapsulation key must be %d bytes", mlkem.EncapsulationKeySize768)
	}
	expectedKeyID, err := RecipientKeyID(recipient.EncapsulationKey, recipient.KeyVersion)
	if err != nil {
		return Recipient{}, err
	}
	if recipient.KeyID == "" {
		recipient.KeyID = expectedKeyID
	}
	if recipient.KeyID != expectedKeyID {
		return Recipient{}, fmt.Errorf("recipient key ID does not match encapsulation key")
	}
	if err := validateRecipientKeyID(recipient.KeyID); err != nil {
		return Recipient{}, err
	}
	recipient.EncapsulationKey = cloneBytes(recipient.EncapsulationKey)
	return recipient, nil
}

func validateContext(ctx PayloadContext) error {
	required := map[string]string{
		"envelope_id":  ctx.EnvelopeID,
		"incident_id":  ctx.IncidentID,
		"stream_id":    ctx.StreamID,
		"media_type":   ctx.MediaType,
		"payload_type": ctx.PayloadType,
		"media_key_id": ctx.MediaKeyID,
	}
	for name, value := range required {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
		if err := rejectNewline(name, value); err != nil {
			return err
		}
	}
	if ctx.ChunkIndex <= 0 {
		return fmt.Errorf("chunk_index must be positive")
	}
	return nil
}

func (header payloadHeader) matchesContext(ctx PayloadContext) error {
	if header.EnvelopeID != ctx.EnvelopeID ||
		header.IncidentID != ctx.IncidentID ||
		header.StreamID != ctx.StreamID ||
		header.MediaType != ctx.MediaType ||
		header.ChunkIndex != ctx.ChunkIndex ||
		header.PayloadType != ctx.PayloadType {
		return fmt.Errorf("payload context does not match authenticated header")
	}
	return nil
}

func validatePublicMetadata(meta PublicWrappingMetadata, ctx PayloadContext, header payloadHeader, payloadHeaderDigest []byte, frame wrappedKeyFrame) ([]byte, []byte, error) {
	if meta.Profile != ProfileID {
		return nil, nil, fmt.Errorf("unsupported profile")
	}
	if meta.Scheme != SchemeID {
		return nil, nil, fmt.Errorf("unsupported scheme")
	}
	if meta.SuiteID != SuiteID {
		return nil, nil, fmt.Errorf("unsupported suite")
	}
	if meta.HKDFInfoID != HKDFInfoID {
		return nil, nil, fmt.Errorf("unsupported HKDF info ID")
	}
	if err := validateMandatory(meta.Mandatory); err != nil {
		return nil, nil, err
	}
	if err := validateRecipientKeyID(meta.RecipientKeyID); err != nil {
		return nil, nil, err
	}
	if meta.RecipientKeyVersion <= 0 {
		return nil, nil, fmt.Errorf("recipient key version must be positive")
	}
	if meta.RecipientRole == "" {
		return nil, nil, fmt.Errorf("recipient role is required")
	}
	if meta.MediaKeyID != ctx.MediaKeyID {
		return nil, nil, fmt.Errorf("media key ID does not match context")
	}
	if meta.EnvelopeID != header.EnvelopeID {
		return nil, nil, fmt.Errorf("envelope ID does not match payload header")
	}
	gotHeaderDigest, err := decodeBase64URL("payload_header_digest_b64u", meta.PayloadHeaderDigestB64U, sha512.Size384)
	if err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(gotHeaderDigest, payloadHeaderDigest) {
		return nil, nil, fmt.Errorf("payload header digest does not match")
	}
	gotKEMDigest, err := decodeBase64URL("kem_ciphertext_digest_b64u", meta.KEMCiphertextDigestB64U, sha512.Size384)
	if err != nil {
		return nil, nil, err
	}
	kemDigest := sha384(frame.KEMCiphertext)
	if !bytes.Equal(gotKEMDigest, kemDigest) {
		return nil, nil, fmt.Errorf("KEM ciphertext digest does not match")
	}
	salt, err := decodeBase64URL("hkdf_salt_b64u", meta.HKDFSaltB64U, hkdfSaltSize)
	if err != nil {
		return nil, nil, err
	}
	recipient := Recipient{
		KeyID:      meta.RecipientKeyID,
		KeyVersion: meta.RecipientKeyVersion,
		Role:       meta.RecipientRole,
	}
	wrapAAD, err := cekWrapAAD(ctx, header.EnvelopeID, recipient, payloadHeaderDigest)
	if err != nil {
		return nil, nil, err
	}
	gotWrapAADDigest, err := decodeBase64URL("cek_wrap_aad_digest_b64u", meta.CEKWrapAADDigestB64U, sha512.Size384)
	if err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(gotWrapAADDigest, sha384(wrapAAD)) {
		return nil, nil, fmt.Errorf("CEK-wrap AAD digest does not match")
	}
	return salt, wrapAAD, nil
}

func validateMandatory(fields []string) error {
	if len(fields) != len(publicMetadataMandatory) {
		return fmt.Errorf("mandatory fields are not understood")
	}
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if seen[field] {
			return fmt.Errorf("duplicate mandatory field %q", field)
		}
		seen[field] = true
	}
	for _, field := range publicMetadataMandatory {
		if !seen[field] {
			return fmt.Errorf("missing mandatory field %q", field)
		}
	}
	for field := range seen {
		if !containsString(publicMetadataMandatory, field) {
			return fmt.Errorf("unknown mandatory field %q", field)
		}
	}
	return nil
}

func validateRecipientKeyID(keyID string) error {
	if !strings.HasPrefix(keyID, recipientKeyIDPrefix) {
		return fmt.Errorf("recipient key ID must use %s prefix", recipientKeyIDPrefix)
	}
	encodedDigest := strings.TrimPrefix(keyID, recipientKeyIDPrefix)
	digest, err := decodeBase64URL("recipient_key_id", encodedDigest, sha512.Size384)
	if err != nil {
		return err
	}
	if len(digest) != sha512.Size384 {
		return fmt.Errorf("recipient key ID digest has unsupported size")
	}
	return nil
}

func encodePayloadHeader(header payloadHeader) ([]byte, error) {
	if header.Scheme != SchemeID || header.SuiteID != SuiteID || header.Digest != DigestID || header.PayloadAEAD != AEADID {
		return nil, fmt.Errorf("unsupported payload header profile")
	}
	if len(header.PayloadNonce) != nonceSize {
		return nil, fmt.Errorf("payload nonce must be %d bytes", nonceSize)
	}
	return encodeCanonicalFields([]canonicalField{
		stringField("scheme", header.Scheme),
		stringField("suite_id", header.SuiteID),
		stringField("digest", header.Digest),
		stringField("envelope_id", header.EnvelopeID),
		stringField("incident_id", header.IncidentID),
		stringField("stream_id", header.StreamID),
		stringField("media_type", header.MediaType),
		intField("chunk_index", header.ChunkIndex),
		stringField("payload_type", header.PayloadType),
		stringField("payload_aead", header.PayloadAEAD),
		bytesField("payload_nonce", header.PayloadNonce),
	})
}

func decodePayloadHeader(headerBytes []byte) (payloadHeader, error) {
	fields, err := decodeCanonicalFields(headerBytes)
	if err != nil {
		return payloadHeader{}, err
	}
	expected := []string{
		"scheme",
		"suite_id",
		"digest",
		"envelope_id",
		"incident_id",
		"stream_id",
		"media_type",
		"chunk_index",
		"payload_type",
		"payload_aead",
		"payload_nonce",
	}
	if len(fields) != len(expected) {
		return payloadHeader{}, fmt.Errorf("payload header field count is unsupported")
	}
	values := make(map[string][]byte, len(fields))
	for i, field := range fields {
		if field.name != expected[i] {
			return payloadHeader{}, fmt.Errorf("payload header field %d is %q, want %q", i, field.name, expected[i])
		}
		values[field.name] = field.value
	}
	chunkIndex, err := strconv.Atoi(string(values["chunk_index"]))
	if err != nil || chunkIndex <= 0 {
		return payloadHeader{}, fmt.Errorf("chunk_index is malformed")
	}
	header := payloadHeader{
		Scheme:       string(values["scheme"]),
		SuiteID:      string(values["suite_id"]),
		Digest:       string(values["digest"]),
		EnvelopeID:   string(values["envelope_id"]),
		IncidentID:   string(values["incident_id"]),
		StreamID:     string(values["stream_id"]),
		MediaType:    string(values["media_type"]),
		ChunkIndex:   chunkIndex,
		PayloadType:  string(values["payload_type"]),
		PayloadAEAD:  string(values["payload_aead"]),
		PayloadNonce: cloneBytes(values["payload_nonce"]),
	}
	if header.Scheme != SchemeID {
		return payloadHeader{}, fmt.Errorf("unsupported payload scheme")
	}
	if header.SuiteID != SuiteID {
		return payloadHeader{}, fmt.Errorf("unsupported payload suite")
	}
	if header.Digest != DigestID {
		return payloadHeader{}, fmt.Errorf("unsupported payload digest")
	}
	if header.PayloadAEAD != AEADID {
		return payloadHeader{}, fmt.Errorf("unsupported payload AEAD")
	}
	if len(header.PayloadNonce) != nonceSize {
		return payloadHeader{}, fmt.Errorf("payload nonce must be %d bytes", nonceSize)
	}
	return header, nil
}

func encodePayloadFrame(headerBytes, ciphertext []byte) ([]byte, error) {
	if len(headerBytes) == 0 {
		return nil, fmt.Errorf("payload header is required")
	}
	if len(headerBytes) > maxHeaderLength {
		return nil, fmt.Errorf("payload header exceeds %d bytes", maxHeaderLength)
	}
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("payload ciphertext is required")
	}
	frame := make([]byte, 0, len(payloadMagic)+1+4+len(headerBytes)+len(ciphertext))
	frame = append(frame, payloadMagic...)
	frame = append(frame, version)
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(headerBytes)))
	frame = append(frame, headerBytes...)
	frame = append(frame, ciphertext...)
	return frame, nil
}

func parsePayloadFrame(frame []byte) (payloadHeader, []byte, []byte, error) {
	if len(frame) < len(payloadMagic)+1+4 {
		return payloadHeader{}, nil, nil, fmt.Errorf("payload frame is truncated")
	}
	if !bytes.HasPrefix(frame, []byte(payloadMagic)) {
		return payloadHeader{}, nil, nil, fmt.Errorf("invalid payload magic")
	}
	offset := len(payloadMagic)
	if frame[offset] != version {
		return payloadHeader{}, nil, nil, fmt.Errorf("unsupported payload version")
	}
	offset++
	headerLength := binary.BigEndian.Uint32(frame[offset : offset+4])
	offset += 4
	if headerLength == 0 {
		return payloadHeader{}, nil, nil, fmt.Errorf("payload header is missing")
	}
	if headerLength > maxHeaderLength {
		return payloadHeader{}, nil, nil, fmt.Errorf("payload header exceeds %d bytes", maxHeaderLength)
	}
	headerEnd := offset + int(headerLength)
	if len(frame) < headerEnd {
		return payloadHeader{}, nil, nil, fmt.Errorf("payload header is truncated")
	}
	ciphertext := frame[headerEnd:]
	if len(ciphertext) == 0 {
		return payloadHeader{}, nil, nil, fmt.Errorf("payload ciphertext is missing")
	}
	headerBytes := cloneBytes(frame[offset:headerEnd])
	header, err := decodePayloadHeader(headerBytes)
	if err != nil {
		return payloadHeader{}, nil, nil, err
	}
	return header, headerBytes, cloneBytes(ciphertext), nil
}

func encodeWrappedKeyFrame(kemCiphertext, wrapNonce, wrappedCEK []byte) ([]byte, error) {
	if len(kemCiphertext) != mlkem.CiphertextSize768 {
		return nil, fmt.Errorf("KEM ciphertext must be %d bytes", mlkem.CiphertextSize768)
	}
	if len(wrapNonce) != nonceSize {
		return nil, fmt.Errorf("CEK-wrap nonce must be %d bytes", nonceSize)
	}
	if len(wrappedCEK) != wrappedCEKSize {
		return nil, fmt.Errorf("wrapped CEK must be %d bytes", wrappedCEKSize)
	}
	frame := make([]byte, 0, len(wrappedKeyMagic)+1+2+len(kemCiphertext)+1+len(wrapNonce)+2+len(wrappedCEK))
	frame = append(frame, wrappedKeyMagic...)
	frame = append(frame, version)
	frame = binary.BigEndian.AppendUint16(frame, uint16(len(kemCiphertext)))
	frame = append(frame, kemCiphertext...)
	frame = append(frame, byte(len(wrapNonce)))
	frame = append(frame, wrapNonce...)
	frame = binary.BigEndian.AppendUint16(frame, uint16(len(wrappedCEK)))
	frame = append(frame, wrappedCEK...)
	return frame, nil
}

func parseWrappedKeyFrame(frame []byte) (wrappedKeyFrame, error) {
	if len(frame) < len(wrappedKeyMagic)+1+2 {
		return wrappedKeyFrame{}, fmt.Errorf("wrapped-key frame is truncated")
	}
	if !bytes.HasPrefix(frame, []byte(wrappedKeyMagic)) {
		return wrappedKeyFrame{}, fmt.Errorf("invalid wrapped-key magic")
	}
	offset := len(wrappedKeyMagic)
	if frame[offset] != version {
		return wrappedKeyFrame{}, fmt.Errorf("unsupported wrapped-key version")
	}
	offset++
	kemLength := int(binary.BigEndian.Uint16(frame[offset : offset+2]))
	offset += 2
	if kemLength != mlkem.CiphertextSize768 {
		return wrappedKeyFrame{}, fmt.Errorf("KEM ciphertext has unsupported size")
	}
	if len(frame) < offset+kemLength+1 {
		return wrappedKeyFrame{}, fmt.Errorf("KEM ciphertext is truncated")
	}
	kemCiphertext := cloneBytes(frame[offset : offset+kemLength])
	offset += kemLength
	nonceLength := int(frame[offset])
	offset++
	if nonceLength != nonceSize {
		return wrappedKeyFrame{}, fmt.Errorf("CEK-wrap nonce has unsupported size")
	}
	if len(frame) < offset+nonceLength+2 {
		return wrappedKeyFrame{}, fmt.Errorf("CEK-wrap nonce is truncated")
	}
	wrapNonce := cloneBytes(frame[offset : offset+nonceLength])
	offset += nonceLength
	wrappedLength := int(binary.BigEndian.Uint16(frame[offset : offset+2]))
	offset += 2
	if wrappedLength != wrappedCEKSize {
		return wrappedKeyFrame{}, fmt.Errorf("wrapped CEK has unsupported size")
	}
	if len(frame) != offset+wrappedLength {
		return wrappedKeyFrame{}, fmt.Errorf("wrapped CEK frame has trailing or truncated bytes")
	}
	wrappedCEK := cloneBytes(frame[offset : offset+wrappedLength])
	return wrappedKeyFrame{
		KEMCiphertext: kemCiphertext,
		WrapNonce:     wrapNonce,
		WrappedCEK:    wrappedCEK,
	}, nil
}

type canonicalField struct {
	name  string
	value []byte
}

func stringField(name, value string) canonicalField {
	return canonicalField{name: name, value: []byte(value)}
}

func intField(name string, value int) canonicalField {
	return canonicalField{name: name, value: []byte(strconv.Itoa(value))}
}

func bytesField(name string, value []byte) canonicalField {
	return canonicalField{name: name, value: cloneBytes(value)}
}

func encodeCanonicalFields(fields []canonicalField) ([]byte, error) {
	var out []byte
	for _, field := range fields {
		if field.name == "" {
			return nil, fmt.Errorf("canonical field name is required")
		}
		if !utf8.ValidString(field.name) {
			return nil, fmt.Errorf("canonical field name is not UTF-8")
		}
		if len(field.name) > maxCanonicalNameLength {
			return nil, fmt.Errorf("canonical field name is too long")
		}
		if len(field.value) > maxCanonicalValueLength {
			return nil, fmt.Errorf("canonical field value is too long")
		}
		out = binary.BigEndian.AppendUint16(out, uint16(len(field.name)))
		out = append(out, field.name...)
		out = binary.BigEndian.AppendUint32(out, uint32(len(field.value)))
		out = append(out, field.value...)
	}
	return out, nil
}

func decodeCanonicalFields(data []byte) ([]canonicalField, error) {
	var fields []canonicalField
	offset := 0
	for offset < len(data) {
		if len(data)-offset < 2 {
			return nil, fmt.Errorf("canonical field name length is truncated")
		}
		nameLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if nameLength == 0 {
			return nil, fmt.Errorf("canonical field name is missing")
		}
		if len(data)-offset < nameLength+4 {
			return nil, fmt.Errorf("canonical field name is truncated")
		}
		nameBytes := data[offset : offset+nameLength]
		offset += nameLength
		if !utf8.Valid(nameBytes) {
			return nil, fmt.Errorf("canonical field name is not UTF-8")
		}
		valueLength := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if len(data)-offset < valueLength {
			return nil, fmt.Errorf("canonical field value is truncated")
		}
		value := cloneBytes(data[offset : offset+valueLength])
		offset += valueLength
		fields = append(fields, canonicalField{name: string(nameBytes), value: value})
	}
	return fields, nil
}

func cekWrapAAD(ctx PayloadContext, envelopeID string, recipient Recipient, payloadHeaderDigest []byte) ([]byte, error) {
	return encodeCanonicalFields([]canonicalField{
		stringField("scheme", SchemeID),
		stringField("suite_id", SuiteID),
		stringField("digest", DigestID),
		stringField("wrapping_algorithm", WrappingAlgorithm),
		intField("wrapping_algorithm_version", WrappingAlgorithmVersion),
		stringField("envelope_id", envelopeID),
		stringField("recipient_key_id", recipient.KeyID),
		intField("recipient_key_version", recipient.KeyVersion),
		stringField("recipient_role", recipient.Role),
		stringField("media_key_id", ctx.MediaKeyID),
		stringField("kem", KEMID),
		stringField("kdf", KDFID),
		stringField("wrapping_purpose", HKDFInfoID),
		bytesField("payload_header_digest", payloadHeaderDigest),
	})
}

func hkdfInfo(envelopeID, recipientKeyID string, kemCiphertextDigest, payloadHeaderDigest []byte) ([]byte, error) {
	return encodeCanonicalFields([]canonicalField{
		stringField("wrapping_purpose", HKDFInfoID),
		stringField("suite_id", SuiteID),
		stringField("envelope_id", envelopeID),
		stringField("recipient_key_id", recipientKeyID),
		stringField("digest", DigestID),
		bytesField("kem_ciphertext_digest", kemCiphertextDigest),
		bytesField("payload_header_digest", payloadHeaderDigest),
	})
}

func deriveKEK(sharedSecret, salt, info []byte) ([]byte, error) {
	if len(sharedSecret) != mlkem.SharedKeySize {
		return nil, fmt.Errorf("ML-KEM shared secret has unsupported size")
	}
	if len(salt) != hkdfSaltSize {
		return nil, fmt.Errorf("HKDF salt has unsupported size")
	}
	prk, err := hkdf.Extract(sha512.New384, sharedSecret, salt)
	if err != nil {
		return nil, fmt.Errorf("HKDF extract: %w", err)
	}
	kek, err := hkdf.Expand(sha512.New384, prk, string(info), cekSize)
	if err != nil {
		return nil, fmt.Errorf("HKDF expand: %w", err)
	}
	return kek, nil
}

func newAESGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != cekSize {
		return nil, fmt.Errorf("AES-256-GCM key has unsupported size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES block cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return aead, nil
}

func readRandom(reader io.Reader, size int) ([]byte, error) {
	out := make([]byte, size)
	if _, err := io.ReadFull(reader, out); err != nil {
		return nil, err
	}
	return out, nil
}

func sha384(data []byte) []byte {
	sum := sha512.Sum384(data)
	return sum[:]
}

func rejectNewline(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain newlines", field)
	}
	return nil
}

func decodeBase64URL(field, value string, expectedLength int) ([]byte, error) {
	if value == "" {
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
	if expectedLength > 0 && len(decoded) != expectedLength {
		return nil, fmt.Errorf("%s must decode to %d bytes", field, expectedLength)
	}
	return decoded, nil
}

func cloneBytes(in []byte) []byte {
	if in == nil {
		return nil
	}
	return append([]byte(nil), in...)
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string(nil), in...)
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
