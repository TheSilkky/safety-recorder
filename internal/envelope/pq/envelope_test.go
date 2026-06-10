package pq

import (
	"bytes"
	"crypto/mlkem"
	"crypto/mlkem/mlkemtest"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTripWithGeneratedFixtureKeys(t *testing.T) {
	recipient, dk, err := GenerateRecipientKey(1)
	if err != nil {
		t.Fatalf("GenerateRecipientKey returned error: %v", err)
	}
	ctx := testPayloadContext()
	plaintext := []byte("test-only post-quantum envelope plaintext")

	env, err := Encrypt(plaintext, ctx, []Recipient{recipient})
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if bytes.Contains(env.PayloadFrame, plaintext) {
		t.Fatal("payload frame contains plaintext")
	}

	got, err := Decrypt(env, ctx, recipient.KeyID, dk)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
	}
}

func TestDeterministicLocalSingleRecipientVector(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "single", 1)
	ctx := testPayloadContext()
	plaintext := []byte("local deterministic vector plaintext")

	env1 := deterministicEncrypt(t, plaintext, ctx, []Recipient{recipient}, "single-vector")
	env2 := deterministicEncrypt(t, plaintext, ctx, []Recipient{recipient}, "single-vector")
	if !bytes.Equal(env1.PayloadFrame, env2.PayloadFrame) {
		t.Fatal("deterministic payload frames differ")
	}
	if !bytes.Equal(env1.Recipients[0].WrappedKeyFrame, env2.Recipients[0].WrappedKeyFrame) {
		t.Fatal("deterministic wrapped-key frames differ")
	}
	if env1.Recipients[0].Metadata.RecipientKeyID != recipient.KeyID {
		t.Fatal("recipient key ID not preserved in vector")
	}
	if !strings.HasPrefix(recipient.KeyID, recipientKeyIDPrefix) {
		t.Fatalf("recipient key ID = %q, want %s prefix", recipient.KeyID, recipientKeyIDPrefix)
	}
	assertVectorDoesNotContainSecrets(t, env1, plaintext, dk.Bytes())

	got, err := Decrypt(env1, ctx, recipient.KeyID, dk)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
	}
}

func TestDeterministicLocalMultiRecipientVector(t *testing.T) {
	alex, alexDK := deterministicRecipient(t, "alex", 1)
	blair, blairDK := deterministicRecipient(t, "blair", 1)
	ctx := testPayloadContext()
	plaintext := []byte("multi-recipient local vector plaintext")

	env := deterministicEncrypt(t, plaintext, ctx, []Recipient{alex, blair}, "multi-vector")
	if len(env.Recipients) != 2 {
		t.Fatalf("recipient count = %d, want 2", len(env.Recipients))
	}
	for _, tt := range []struct {
		name      string
		recipient Recipient
		dk        *mlkem.DecapsulationKey768
	}{
		{name: "alex", recipient: alex, dk: alexDK},
		{name: "blair", recipient: blair, dk: blairDK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(env, ctx, tt.recipient.KeyID, tt.dk)
			if err != nil {
				t.Fatalf("Decrypt returned error: %v", err)
			}
			if !bytes.Equal(got, plaintext) {
				t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
			}
		})
	}
	if _, err := Decrypt(env, ctx, alex.KeyID, blairDK); err == nil {
		t.Fatal("Decrypt succeeded when using another recipient's private key")
	}
}

func TestDecryptRejectsAADMismatch(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "aad", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "aad-vector")
	ctx.ChunkIndex++

	if _, err := Decrypt(env, ctx, recipient.KeyID, dk); err == nil {
		t.Fatal("Decrypt succeeded with changed payload context")
	}
}

func TestDecryptRejectsMalformedSuiteIDs(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "suite", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "suite-vector")

	t.Run("public metadata", func(t *testing.T) {
		tampered := cloneEnvelope(env)
		tampered.Recipients[0].Metadata.SuiteID = "proofline-pq-mlkem512-hkdfsha384-aes256gcm-v1"
		if _, err := Decrypt(tampered, ctx, recipient.KeyID, dk); err == nil {
			t.Fatal("Decrypt succeeded with malformed metadata suite")
		}
	})

	t.Run("payload header", func(t *testing.T) {
		tampered := cloneEnvelope(env)
		tampered.PayloadFrame = rewritePayloadHeaderField(t, tampered.PayloadFrame, "suite_id", []byte("proofline-pq-mlkem512-hkdfsha384-aes256gcm-v1"))
		if _, err := Decrypt(tampered, ctx, recipient.KeyID, dk); err == nil {
			t.Fatal("Decrypt succeeded with malformed payload suite")
		}
	})
}

func TestDecryptRejectsMalformedRecipientKeyID(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "key-id", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "key-id-vector")
	env.Recipients[0].Metadata.RecipientKeyID = "not-a-pq-key-id"

	if _, err := Decrypt(env, ctx, "not-a-pq-key-id", dk); err == nil {
		t.Fatal("Decrypt succeeded with malformed recipient key ID")
	}
}

func TestDecryptRejectsMalformedKEMCiphertext(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "kem", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "kem-vector")
	offset := len(wrappedKeyMagic) + 1
	binary.BigEndian.PutUint16(env.Recipients[0].WrappedKeyFrame[offset:offset+2], 1)

	if _, err := Decrypt(env, ctx, recipient.KeyID, dk); err == nil {
		t.Fatal("Decrypt succeeded with malformed KEM ciphertext length")
	}
}

func TestDecryptRejectsTamperedKEMCiphertextDigest(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "kem-digest", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "kem-digest-vector")
	offset := len(wrappedKeyMagic) + 1 + 2
	env.Recipients[0].WrappedKeyFrame[offset] ^= 0x01

	if _, err := Decrypt(env, ctx, recipient.KeyID, dk); err == nil {
		t.Fatal("Decrypt succeeded with tampered KEM ciphertext")
	}
}

func TestDecryptRejectsTamperedWrappedCEK(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "wrapped-cek", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "wrapped-cek-vector")
	env.Recipients[0].WrappedKeyFrame[len(env.Recipients[0].WrappedKeyFrame)-1] ^= 0x01

	if _, err := Decrypt(env, ctx, recipient.KeyID, dk); err == nil {
		t.Fatal("Decrypt succeeded with tampered wrapped CEK")
	}
}

func TestDecryptRejectsWrongRecipientKey(t *testing.T) {
	recipient, _ := deterministicRecipient(t, "right-recipient", 1)
	_, wrongDK := deterministicRecipient(t, "wrong-recipient", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "wrong-recipient-vector")

	if _, err := Decrypt(env, ctx, recipient.KeyID, wrongDK); err == nil {
		t.Fatal("Decrypt succeeded with wrong recipient key")
	}
}

func TestDecryptRejectsUnknownMandatoryFields(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "mandatory", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "mandatory-vector")
	env.Recipients[0].Metadata.Mandatory = append(env.Recipients[0].Metadata.Mandatory, "unknown_future_field")

	if _, err := Decrypt(env, ctx, recipient.KeyID, dk); err == nil {
		t.Fatal("Decrypt succeeded with unknown mandatory field")
	}
}

func TestDecryptRejectsDowngradeAttempts(t *testing.T) {
	recipient, dk := deterministicRecipient(t, "downgrade", 1)
	ctx := testPayloadContext()
	env := deterministicEncrypt(t, []byte("plaintext"), ctx, []Recipient{recipient}, "downgrade-vector")

	t.Run("legacy payload magic", func(t *testing.T) {
		tampered := cloneEnvelope(env)
		copy(tampered.PayloadFrame[:len(payloadMagic)], []byte("SRCENC1\n"))
		if _, err := Decrypt(tampered, ctx, recipient.KeyID, dk); err == nil {
			t.Fatal("Decrypt succeeded with legacy payload magic")
		}
	})

	t.Run("legacy metadata scheme", func(t *testing.T) {
		tampered := cloneEnvelope(env)
		tampered.Recipients[0].Metadata.Scheme = "safety-recorder-chunk-encryption-v1"
		if _, err := Decrypt(tampered, ctx, recipient.KeyID, dk); err == nil {
			t.Fatal("Decrypt succeeded with legacy metadata scheme")
		}
	})
}

func TestEncryptRejectsTooManyRecipients(t *testing.T) {
	var recipients []Recipient
	for i := 0; i < MaxRecipientsPerCEK+1; i++ {
		recipient, _ := deterministicRecipient(t, "too-many-"+strconv.Itoa(i), 1)
		recipients = append(recipients, recipient)
	}
	if _, err := deterministicEncryptWithError([]byte("plaintext"), testPayloadContext(), recipients, "too-many"); err == nil {
		t.Fatal("Encrypt succeeded with too many recipients")
	}
}

func TestEncryptRejectsDuplicateRecipients(t *testing.T) {
	recipient, _ := deterministicRecipient(t, "duplicate", 1)
	recipients := []Recipient{recipient, recipient}

	if _, err := deterministicEncryptWithError([]byte("plaintext"), testPayloadContext(), recipients, "duplicate"); err == nil {
		t.Fatal("Encrypt succeeded with duplicate recipient key ID")
	}
}

func TestRecipientKeyIDDerivationRejectsWrongPublicKeyLength(t *testing.T) {
	if _, err := RecipientKeyID([]byte("short"), 1); err == nil {
		t.Fatal("RecipientKeyID succeeded with short encapsulation key")
	}
}

func testPayloadContext() PayloadContext {
	return PayloadContext{
		EnvelopeID:  "env_test_001",
		IncidentID:  "inc_test_001",
		StreamID:    "str_test_001",
		MediaType:   "audio",
		ChunkIndex:  1,
		PayloadType: "chunk",
		MediaKeyID:  "media-key-test-001",
	}
}

func deterministicRecipient(t *testing.T, label string, keyVersion int) (Recipient, *mlkem.DecapsulationKey768) {
	t.Helper()
	seed := fixtureBytes("test-only ML-KEM decapsulation seed "+label, mlkem.SeedSize)
	dk, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		t.Fatalf("NewDecapsulationKey768 returned error: %v", err)
	}
	ekBytes := dk.EncapsulationKey().Bytes()
	keyID, err := RecipientKeyID(ekBytes, keyVersion)
	if err != nil {
		t.Fatalf("RecipientKeyID returned error: %v", err)
	}
	return Recipient{
		KeyID:            keyID,
		KeyVersion:       keyVersion,
		Role:             "trusted_contact",
		EncapsulationKey: ekBytes,
	}, dk
}

func deterministicEncrypt(t *testing.T, plaintext []byte, ctx PayloadContext, recipients []Recipient, label string) Envelope {
	t.Helper()
	env, err := deterministicEncryptWithError(plaintext, ctx, recipients, label)
	if err != nil {
		t.Fatalf("encryptWithOptions returned error: %v", err)
	}
	return env
}

func deterministicEncryptWithError(plaintext []byte, ctx PayloadContext, recipients []Recipient, label string) (Envelope, error) {
	reader := bytes.NewReader(fixtureBytes("test-only random stream "+label, 8192))
	counter := 0
	opts := encryptOptions{
		randomReader: reader,
		encapsulate: func(ek *mlkem.EncapsulationKey768) ([]byte, []byte, error) {
			randomness := fixtureBytes(label+" mlkem randomness "+strconv.Itoa(counter), 32)
			counter++
			return mlkemtest.Encapsulate768(ek, randomness)
		},
	}
	return encryptWithOptions(plaintext, ctx, recipients, opts)
}

func fixtureBytes(label string, size int) []byte {
	var out []byte
	counter := 0
	for len(out) < size {
		sum := sha512.Sum512([]byte(label + ":" + strconv.Itoa(counter)))
		out = append(out, sum[:]...)
		counter++
	}
	return out[:size]
}

func rewritePayloadHeaderField(t *testing.T, frame []byte, name string, value []byte) []byte {
	t.Helper()
	_, headerBytes, ciphertext, err := parsePayloadFrame(frame)
	if err != nil {
		t.Fatalf("parsePayloadFrame returned error: %v", err)
	}
	fields, err := decodeCanonicalFields(headerBytes)
	if err != nil {
		t.Fatalf("decodeCanonicalFields returned error: %v", err)
	}
	replaced := false
	for i := range fields {
		if fields[i].name == name {
			fields[i].value = cloneBytes(value)
			replaced = true
		}
	}
	if !replaced {
		t.Fatalf("field %q not found in payload header", name)
	}
	rewrittenHeader, err := encodeCanonicalFields(fields)
	if err != nil {
		t.Fatalf("encodeCanonicalFields returned error: %v", err)
	}
	rewritten, err := encodePayloadFrame(rewrittenHeader, ciphertext)
	if err != nil {
		t.Fatalf("encodePayloadFrame returned error: %v", err)
	}
	return rewritten
}

func cloneEnvelope(env Envelope) Envelope {
	out := Envelope{
		PayloadFrame: cloneBytes(env.PayloadFrame),
		Recipients:   make([]RecipientWrappingRecord, len(env.Recipients)),
	}
	for i, record := range env.Recipients {
		out.Recipients[i] = RecipientWrappingRecord{
			Metadata: PublicWrappingMetadata{
				Profile:                 record.Metadata.Profile,
				Scheme:                  record.Metadata.Scheme,
				SuiteID:                 record.Metadata.SuiteID,
				Mandatory:               cloneStrings(record.Metadata.Mandatory),
				RecipientKeyID:          record.Metadata.RecipientKeyID,
				RecipientKeyVersion:     record.Metadata.RecipientKeyVersion,
				RecipientRole:           record.Metadata.RecipientRole,
				MediaKeyID:              record.Metadata.MediaKeyID,
				EnvelopeID:              record.Metadata.EnvelopeID,
				PayloadHeaderDigestB64U: record.Metadata.PayloadHeaderDigestB64U,
				KEMCiphertextDigestB64U: record.Metadata.KEMCiphertextDigestB64U,
				HKDFSaltB64U:            record.Metadata.HKDFSaltB64U,
				HKDFInfoID:              record.Metadata.HKDFInfoID,
				CEKWrapAADDigestB64U:    record.Metadata.CEKWrapAADDigestB64U,
			},
			WrappedKeyFrame: cloneBytes(record.WrappedKeyFrame),
		}
	}
	return out
}

func assertVectorDoesNotContainSecrets(t *testing.T, env Envelope, plaintext, decapsulationSeed []byte) {
	t.Helper()
	encoded, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope vector: %v", err)
	}
	if bytes.Contains(env.PayloadFrame, plaintext) || bytes.Contains(encoded, plaintext) {
		t.Fatal("local vector contains plaintext")
	}
	if bytes.Contains(encoded, decapsulationSeed) {
		t.Fatal("local vector contains decapsulation seed")
	}
	seedB64 := base64.StdEncoding.EncodeToString(decapsulationSeed)
	if bytes.Contains(encoded, []byte(seedB64)) {
		t.Fatal("local vector contains encoded decapsulation seed")
	}
	plaintextDigest := sha256.Sum256(plaintext)
	if bytes.Contains(encoded, plaintextDigest[:]) {
		t.Fatal("local vector contains plaintext digest bytes")
	}
}
