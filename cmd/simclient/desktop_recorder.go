package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func runDesktopRecorder(ctx context.Context, out io.Writer, cfg config) error {
	if cfg.keyFile == "" {
		cfg.keyFile = filepath.Join(cfg.desktopStageDir, "simulator-key.json")
	}
	stage, err := openDesktopStage(cfg.desktopStageDir)
	if err != nil {
		return err
	}
	encryption, err := prepareEncryption(out, cfg)
	if err != nil {
		return err
	}

	sim := client{
		httpClient: newHTTPClient(cfg),
		apiBase:    cfg.apiBase,
		viewerBase: cfg.viewerBase,
	}

	fmt.Fprintln(out, "Logging in...")
	sessionToken, err := sim.login(ctx, cfg.username, cfg.password)
	if err != nil {
		return err
	}
	sim.sessionToken = sessionToken

	manifest, err := prepareDesktopStage(ctx, out, sim, cfg, stage, encryption)
	if err != nil {
		if failErr := failIncompleteDesktopStream(ctx, out, sim, cfg, manifest); failErr != nil {
			return failErr
		}
		return err
	}
	bundleVerification := encryption
	contactWrappedVerification := false
	if encryption.mode == envelopeModeV1 {
		if wrappedKey, ok, err := prepareContactWrappedKey(out, cfg, manifest.IncidentID, manifest.StreamID, encryption.v1Key); err != nil {
			return err
		} else if ok {
			bundleVerification = newV1SimulatorEncryption(wrappedKey)
			contactWrappedVerification = true
		}
	}
	if cfg.desktopStageOnly {
		fmt.Fprintln(out, "Upload skipped.")
		return nil
	}

	if err := uploadDesktopStage(ctx, out, sim, cfg, stage, &manifest); err != nil {
		if failErr := failIncompleteDesktopStream(ctx, out, sim, cfg, manifest); failErr != nil {
			return failErr
		}
		return err
	}
	if cfg.completeStream {
		if !manifestAllUploaded(manifest) {
			return fmt.Errorf("stream completion skipped because local staging is not fully uploaded")
		}
		fmt.Fprintln(out, "Completing stream...")
		if err := sim.completeMediaStream(ctx, manifest.IncidentID, manifest.StreamID, len(manifest.Chunks)); err != nil {
			return err
		}
		fmt.Fprintln(out, "Stream complete.")
	}
	if cfg.downloadBundle {
		token, err := sim.createIncidentToken(ctx, manifest.IncidentID)
		if err != nil {
			return err
		}
		bundleCfg := cfg
		bundleCfg.mediaType = manifest.MediaType
		if err := downloadAndVerifyBundle(ctx, out, sim, bundleCfg, token, manifest.IncidentID, manifest.StreamID, bundleVerification, contactWrappedVerification); err != nil {
			return err
		}
	}
	if cfg.closeIncident {
		fmt.Fprintln(out, "Closing incident...")
		if err := sim.closeIncident(ctx, manifest.IncidentID); err != nil {
			return err
		}
		fmt.Fprintln(out, "Incident closed.")
	}

	fmt.Fprintln(out, "Done.")
	return nil
}

func prepareDesktopStage(ctx context.Context, out io.Writer, sim client, cfg config, stage *desktopStage, encryption simulatorEncryption) (desktopStageManifest, error) {
	if cfg.desktopResume {
		manifest, err := stage.loadManifest()
		if err != nil {
			return desktopStageManifest{}, err
		}
		fmt.Fprintf(out, "Resuming %d staged encrypted chunks; %d already uploaded.\n", len(manifest.Chunks), manifestUploadedCount(manifest))
		return manifest, nil
	}

	exists, err := stage.manifestExists()
	if err != nil {
		return desktopStageManifest{}, err
	}
	if exists {
		return desktopStageManifest{}, fmt.Errorf("desktop staging manifest already exists; use --resume-staged")
	}

	fmt.Fprintln(out, "Creating incident...")
	incidentID, err := sim.createIncident(ctx)
	if err != nil {
		return desktopStageManifest{}, err
	}
	fmt.Fprintf(out, "Incident: %s\n", incidentID)

	fmt.Fprintf(out, "Creating %s media stream...\n", cfg.mediaType)
	streamID, err := sim.createMediaStream(ctx, incidentID, cfg.mediaType)
	if err != nil {
		return desktopStageManifest{}, err
	}
	fmt.Fprintf(out, "Stream: %s\n", streamID)
	fmt.Fprintln(out, "Token-bearing viewer URLs are omitted from output.")

	manifest := newDesktopManifest(incidentID, streamID, cfg.mediaType, cfg.desktopSource)
	if err := stage.saveManifest(manifest); err != nil {
		return manifest, err
	}
	switch cfg.desktopSource {
	case desktopSourceGenerated:
		err = stageGeneratedChunks(stage, &manifest, encryption, cfg)
	case desktopSourceFiles:
		err = stageInputFiles(stage, &manifest, encryption, cfg)
	case desktopSourceFFmpeg:
		if cfg.desktopStageOnly {
			err = stageFFmpegSegments(ctx, stage, &manifest, encryption, cfg)
		} else {
			err = stageAndUploadFFmpegSegments(ctx, out, sim, cfg, stage, &manifest, encryption)
		}
	default:
		err = fmt.Errorf("unsupported desktop source")
	}
	if err != nil {
		return manifest, err
	}
	fmt.Fprintf(out, "Staged %d encrypted chunks.\n", len(manifest.Chunks))
	return manifest, nil
}

func failIncompleteDesktopStream(ctx context.Context, out io.Writer, sim client, cfg config, manifest desktopStageManifest) error {
	if !cfg.desktopFailIncomplete {
		return nil
	}
	if manifest.IncidentID == "" || manifest.StreamID == "" {
		return nil
	}
	fmt.Fprintln(out, "Failing stream because local staged chunks did not fully upload.")
	return sim.failMediaStream(ctx, manifest.IncidentID, manifest.StreamID, "desktop recorder simulator could not upload a contiguous staged queue")
}

func stageGeneratedChunks(stage *desktopStage, manifest *desktopStageManifest, encryption simulatorEncryption, cfg config) error {
	if cfg.chunks <= 0 {
		return fmt.Errorf("--desktop-source=generated requires --chunks greater than zero")
	}
	startedAt := time.Now().UTC()
	for i := 1; i <= cfg.chunks; i++ {
		body, err := randomChunkBytes(cfg.chunkSize)
		if err != nil {
			return err
		}
		chunkStartedAt := startedAt.Add(time.Duration(i-1) * chunkDuration)
		if err := stage.stageChunk(manifest, encryption, body, chunkStartedAt, chunkStartedAt.Add(chunkDuration)); err != nil {
			return err
		}
	}
	return nil
}

func stageInputFiles(stage *desktopStage, manifest *desktopStageManifest, encryption simulatorEncryption, cfg config) error {
	startedAt := time.Now().UTC()
	for i, inputFile := range cfg.desktopInputFiles {
		body, err := os.ReadFile(inputFile)
		if err != nil {
			return safePathError("read input file", err)
		}
		if len(body) == 0 {
			return fmt.Errorf("input file is empty")
		}
		chunkStartedAt := startedAt.Add(time.Duration(i) * chunkDuration)
		if err := stage.stageChunk(manifest, encryption, body, chunkStartedAt, chunkStartedAt.Add(chunkDuration)); err != nil {
			return err
		}
	}
	return nil
}

func uploadDesktopStage(ctx context.Context, out io.Writer, sim client, cfg config, stage *desktopStage, manifest *desktopStageManifest) error {
	if len(manifest.Chunks) == 0 {
		return fmt.Errorf("desktop staging manifest has no chunks")
	}
	for i := range manifest.Chunks {
		if manifest.Chunks[i].Uploaded {
			continue
		}
		if err := uploadDesktopChunk(ctx, out, sim, cfg, stage, manifest, i); err != nil {
			return err
		}
	}
	return nil
}

func uploadDesktopChunk(ctx context.Context, out io.Writer, sim client, cfg config, stage *desktopStage, manifest *desktopStageManifest, chunkOffset int) error {
	chunk := manifest.Chunks[chunkOffset]
	for attempt := 1; attempt <= cfg.desktopMaxAttempts; attempt++ {
		upload, err := stage.chunkUpload(*manifest, chunk)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Uploading staged %s chunk %d...\n", manifest.MediaType, chunk.ChunkIndex)
		manifest.Chunks[chunkOffset].UploadAttempts++
		if err := stage.saveManifest(*manifest); err != nil {
			return err
		}
		if err := sim.uploadChunk(ctx, upload); err == nil {
			now := time.Now().UTC()
			manifest.Chunks[chunkOffset].Uploaded = true
			manifest.Chunks[chunkOffset].UploadedAt = &now
			if err := stage.saveManifest(*manifest); err != nil {
				return err
			}
			fmt.Fprintf(out, "Uploaded staged chunk %d.\n", chunk.ChunkIndex)
			return nil
		} else if attempt == cfg.desktopMaxAttempts {
			return fmt.Errorf("upload staged chunk %d failed after %d attempts: %w", chunk.ChunkIndex, attempt, err)
		}
		time.Sleep(cfg.desktopRetryDelay)
	}
	return fmt.Errorf("upload staged chunk %d failed", chunk.ChunkIndex)
}
