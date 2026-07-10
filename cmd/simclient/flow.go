package main

import (
	"context"
	"fmt"
	"io"
	"time"
)

func prepareEncryption(out io.Writer, cfg config) (simulatorEncryption, error) {
	if !cfg.encrypt {
		fmt.Fprintln(out, "Encryption: disabled. Uploading raw fake chunk bytes for development compatibility only.")
		fmt.Fprintln(out)
		return simulatorEncryption{enabled: false}, nil
	}

	encryption, err := loadOrCreateSimulatorEncryption(cfg)
	if err != nil {
		return simulatorEncryption{}, err
	}
	fmt.Fprintln(out, "Encryption: enabled")
	switch encryption.mode {
	case envelopeModePQ:
		fmt.Fprintln(out, "Envelope: post-quantum")
		fmt.Fprintf(out, "Recipient key ID: %s\n", encryption.pqRecipient.KeyID)
	case envelopeModeV1:
		fmt.Fprintln(out, "Envelope: v1 compatibility")
		fmt.Fprintf(out, "Key ID: %s\n", encryption.v1Key.KeyID)
	}
	if cfg.keyFile != "" {
		fmt.Fprintln(out, "Key file configured; path omitted from output.")
	}
	fmt.Fprintln(out)
	return encryption, nil
}

func authenticateSimulatorSession(ctx context.Context, out io.Writer, sim *client, cfg config) error {
	fmt.Fprintln(out, "Logging in...")
	login, err := sim.login(ctx, cfg.username, cfg.password)
	if err != nil {
		return err
	}
	sim.sessionToken = login.Token

	if cfg.setupTOTPSecondFactor {
		if login.SecondFactorVerificationRequired {
			return fmt.Errorf("--setup-totp-second-factor cannot be used when an active factor already requires verification")
		}
		fmt.Fprintln(out, "Setting up TOTP second factor...")
		if err := sim.setupTOTPSecondFactor(ctx); err != nil {
			return err
		}
		fmt.Fprintln(out, "TOTP second factor verified.")
		return nil
	}

	if !login.SecondFactorVerificationRequired {
		return nil
	}
	if cfg.totpCode == "" {
		return fmt.Errorf("account session requires second-factor verification; set the current TOTP code through PROOFLINE_SIM_TOTP_CODE")
	}
	fmt.Fprintln(out, "Verifying TOTP second factor...")
	if err := sim.verifyTOTPSecondFactor(ctx, cfg.totpCode); err != nil {
		return err
	}
	fmt.Fprintln(out, "TOTP second factor verified.")
	return nil
}

func uploadChunks(ctx context.Context, out io.Writer, sim client, cfg config, incidentID, streamID string, encryption simulatorEncryption, relaySession relaySession) error {
	startedAt := time.Now().UTC()
	for i := 1; i <= cfg.chunks; i++ {
		chunk, err := createUploadChunk(cfg, encryption, incidentID, streamID, i, startedAt)
		if err != nil {
			return err
		}
		if err := uploadChunkWithOptionalHashFailure(ctx, out, sim, cfg, chunk, i, relaySession); err != nil {
			return err
		}
		if shouldSendCheckin(i) {
			fmt.Fprintln(out, "Sending checkin...")
			if err := sim.createCheckin(ctx, incidentID, i); err != nil {
				return err
			}
		}
		if i < cfg.chunks && cfg.interval > 0 {
			time.Sleep(cfg.interval)
		}
	}
	return nil
}

func createUploadChunk(cfg config, encryption simulatorEncryption, incidentID, streamID string, chunkIndex int, startedAt time.Time) (chunkUpload, error) {
	if cfg.encrypt {
		return newEncryptedChunkUpload(encryption, incidentID, streamID, chunkIndex, cfg.mediaType, cfg.chunkSize, startedAt)
	}
	return newChunkUpload(incidentID, streamID, chunkIndex, cfg.mediaType, cfg.chunkSize, startedAt)
}

func uploadChunkWithOptionalHashFailure(ctx context.Context, out io.Writer, sim client, cfg config, chunk chunkUpload, chunkIndex int, relaySession relaySession) error {
	if !shouldSimulateFailure(chunkIndex, cfg.simulateFailureEvery) {
		fmt.Fprintf(out, "Uploading %s%s chunk %d/%d%s...\n", encryptionLogPrefix(cfg.encrypt), cfg.mediaType, chunkIndex, cfg.chunks, uploadModeLogSuffix(cfg.uploadMode))
		if err := uploadChunkForConfiguredMode(ctx, sim, cfg, relaySession, chunk); err != nil {
			return err
		}
		return verifyFirstChunkIdempotentReplay(ctx, out, sim, cfg, chunk, chunkIndex)
	}

	fmt.Fprintf(out, "Uploading %s%s chunk %d/%d%s with intentionally bad hash...\n", encryptionLogPrefix(cfg.encrypt), cfg.mediaType, chunkIndex, cfg.chunks, uploadModeLogSuffix(cfg.uploadMode))
	failed := chunk
	failed.sha256Hex = badHashFor(chunk.sha256Hex)
	if err := expectHashMismatchForConfiguredMode(ctx, sim, cfg, relaySession, failed); err != nil {
		return err
	}
	fmt.Fprintln(out, "Server rejected chunk as expected.")

	fmt.Fprintf(out, "Retrying %s%s chunk %d/%d%s with correct hash...\n", encryptionLogPrefix(cfg.encrypt), cfg.mediaType, chunkIndex, cfg.chunks, uploadModeLogSuffix(cfg.uploadMode))
	if err := uploadChunkForConfiguredMode(ctx, sim, cfg, relaySession, chunk); err != nil {
		return err
	}
	fmt.Fprintln(out, "Retry succeeded.")
	return verifyFirstChunkIdempotentReplay(ctx, out, sim, cfg, chunk, chunkIndex)
}

func uploadChunkForConfiguredMode(ctx context.Context, sim client, cfg config, relaySession relaySession, chunk chunkUpload) error {
	if cfg.uploadMode == uploadModeRelay {
		return sim.uploadRelayChunk(ctx, relaySession, chunk)
	}
	return sim.uploadChunk(ctx, chunk)
}

func expectHashMismatchForConfiguredMode(ctx context.Context, sim client, cfg config, relaySession relaySession, chunk chunkUpload) error {
	if cfg.uploadMode == uploadModeRelay {
		return sim.expectRelayHashMismatch(ctx, relaySession, chunk)
	}
	return sim.expectHashMismatch(ctx, chunk)
}

func verifyFirstChunkIdempotentReplay(ctx context.Context, out io.Writer, sim client, cfg config, chunk chunkUpload, chunkIndex int) error {
	if cfg.uploadMode != uploadModeDirect {
		return nil
	}
	if chunkIndex != 1 {
		return nil
	}
	fmt.Fprintln(out, "Verifying idempotent replay for chunk 1...")
	if err := sim.expectIdempotentReplay(ctx, chunk); err != nil {
		return err
	}
	fmt.Fprintln(out, "Idempotent replay succeeded.")
	if cfg.reconcileDuplicate {
		if err := verifyDuplicateReconciliationDrill(ctx, out, sim, chunk); err != nil {
			return err
		}
	}
	return nil
}

func verifyDuplicateReconciliationDrill(ctx context.Context, out io.Writer, sim client, chunk chunkUpload) error {
	fmt.Fprintln(out, "Reconciling accepted chunk metadata...")
	if err := sim.expectChunkReconciliationMatch(ctx, chunk); err != nil {
		return err
	}
	fmt.Fprintln(out, "Chunk reconciliation matched.")

	fmt.Fprintln(out, "Verifying reconciliation conflict response...")
	conflict := chunk
	conflict.sha256Hex = badHashFor(chunk.sha256Hex)
	if err := sim.expectChunkReconciliationConflict(ctx, conflict); err != nil {
		return err
	}
	fmt.Fprintln(out, "Reconciliation conflict drill succeeded.")
	return nil
}

func downloadAndVerifyBundle(ctx context.Context, out io.Writer, sim client, cfg config, token, incidentID, streamID string, encryption simulatorEncryption, contactWrapped bool) error {
	fmt.Fprintln(out, "Testing incident stream bundle download...")
	bundleBytes, err := sim.downloadStreamBundle(ctx, token, streamID)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Downloaded bundle.")
	if cfg.encrypt && cfg.verifyBundleDecrypt {
		verified, err := verifyStreamBundleDecryption(bundleBytes, encryption, incidentID, streamID, cfg.mediaType)
		if err != nil {
			return err
		}
		if contactWrapped {
			fmt.Fprintf(out, "Verified contact-wrapped decrypt of %d encrypted chunks.\n", verified)
		} else {
			fmt.Fprintf(out, "Verified decrypt of %d encrypted chunks.\n", verified)
		}
	} else {
		fmt.Fprintln(out, "Bundle download succeeded.")
	}
	if cfg.bundleOutput != "" {
		if err := writeEncryptedBundle(cfg.bundleOutput, bundleBytes); err != nil {
			return err
		}
		fmt.Fprintln(out, "Encrypted bundle written.")
	}
	return nil
}

func uploadModeLogSuffix(mode string) string {
	if mode == uploadModeRelay {
		return " through relay"
	}
	return ""
}
