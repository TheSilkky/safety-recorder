package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const ffmpegPollInterval = 250 * time.Millisecond

func stageFFmpegSegments(ctx context.Context, stage *desktopStage, manifest *desktopStageManifest, encryption simulatorEncryption, cfg config) error {
	if err := resetScratchDir(stage.scratchDir); err != nil {
		return err
	}
	outputPattern := filepath.Join(stage.scratchDir, "segment_%06d.mp4")
	args := ffmpegSegmentArgs(cfg, outputPattern)
	command := exec.CommandContext(ctx, cfg.ffmpegBin, args...)
	if err := command.Run(); err != nil {
		return fmt.Errorf("ffmpeg segment capture failed")
	}

	files, err := sortedScratchFiles(stage.scratchDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("ffmpeg produced no complete segments")
	}
	startedAt := time.Now().UTC()
	for i, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			return safePathError("read ffmpeg segment", err)
		}
		if len(body) == 0 {
			return fmt.Errorf("ffmpeg produced an empty segment")
		}
		chunkStartedAt := startedAt.Add(time.Duration(i) * cfg.ffmpegSegmentTime)
		if err := stage.stageChunk(manifest, encryption, body, chunkStartedAt, chunkStartedAt.Add(cfg.ffmpegSegmentTime)); err != nil {
			return err
		}
		if err := os.Remove(file); err != nil {
			return safePathError("remove ffmpeg scratch segment", err)
		}
	}
	return nil
}

func stageAndUploadFFmpegSegments(ctx context.Context, out io.Writer, sim client, cfg config, stage *desktopStage, manifest *desktopStageManifest, encryption simulatorEncryption) error {
	if err := resetScratchDir(stage.scratchDir); err != nil {
		return err
	}
	outputPattern := filepath.Join(stage.scratchDir, "segment_%06d.mp4")
	ffmpegCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	command := exec.CommandContext(ffmpegCtx, cfg.ffmpegBin, ffmpegSegmentArgs(cfg, outputPattern)...)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start ffmpeg segment capture failed")
	}
	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()

	startedAt := time.Now().UTC()
	processed := map[string]struct{}{}
	ticker := time.NewTicker(ffmpegPollInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("ffmpeg segment capture failed")
			}
			if err := stageAvailableFFmpegSegments(ctx, out, sim, cfg, stage, manifest, encryption, startedAt, processed, true); err != nil {
				return err
			}
			if len(manifest.Chunks) == 0 {
				return fmt.Errorf("ffmpeg produced no complete segments")
			}
			return nil
		case <-ticker.C:
			if err := stageAvailableFFmpegSegments(ctx, out, sim, cfg, stage, manifest, encryption, startedAt, processed, false); err != nil {
				cancel()
				<-done
				return err
			}
		case <-ctx.Done():
			cancel()
			<-done
			return ctx.Err()
		}
	}
}

func stageAvailableFFmpegSegments(ctx context.Context, out io.Writer, sim client, cfg config, stage *desktopStage, manifest *desktopStageManifest, encryption simulatorEncryption, captureStartedAt time.Time, processed map[string]struct{}, final bool) error {
	files, err := sortedScratchFiles(stage.scratchDir)
	if err != nil {
		return err
	}
	limit := len(files)
	if !final && limit > 0 {
		// ffmpeg keeps the newest segment open until it starts the next segment.
		// While capture is still running, only stage files older than the newest.
		limit--
	}
	for _, file := range files[:limit] {
		if _, ok := processed[file]; ok {
			continue
		}
		chunkOffset := len(manifest.Chunks)
		if err := stageFFmpegSegmentFile(stage, manifest, encryption, cfg, captureStartedAt, file); err != nil {
			return err
		}
		processed[file] = struct{}{}
		if err := uploadDesktopChunk(ctx, out, sim, cfg, stage, manifest, chunkOffset); err != nil {
			return err
		}
	}
	return nil
}

func stageFFmpegSegmentFile(stage *desktopStage, manifest *desktopStageManifest, encryption simulatorEncryption, cfg config, captureStartedAt time.Time, file string) error {
	body, err := os.ReadFile(file)
	if err != nil {
		return safePathError("read ffmpeg segment", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("ffmpeg produced an empty segment")
	}
	chunkStartedAt := captureStartedAt.Add(time.Duration(len(manifest.Chunks)) * cfg.ffmpegSegmentTime)
	if err := stage.stageChunk(manifest, encryption, body, chunkStartedAt, chunkStartedAt.Add(cfg.ffmpegSegmentTime)); err != nil {
		return err
	}
	if err := os.Remove(file); err != nil {
		return safePathError("remove ffmpeg scratch segment", err)
	}
	return nil
}

func resetScratchDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return safePathError("clear recorder scratch directory", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return safePathError("create recorder scratch directory", err)
	}
	return nil
}

func ffmpegSegmentArgs(cfg config, outputPattern string) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if cfg.ffmpegInputFormat != "" {
		args = append(args, "-f", cfg.ffmpegInputFormat)
	}
	args = append(args,
		"-i", cfg.ffmpegInput,
		"-t", ffmpegDurationValue(cfg.ffmpegDuration),
		"-c:v", cfg.ffmpegVideoCodec,
		"-pix_fmt", "yuv420p",
		"-f", "segment",
		"-segment_time", ffmpegDurationValue(cfg.ffmpegSegmentTime),
		"-reset_timestamps", "1",
		outputPattern,
	)
	return args
}

func ffmpegDurationValue(duration time.Duration) string {
	return strconv.FormatFloat(duration.Seconds(), 'f', 3, 64)
}
