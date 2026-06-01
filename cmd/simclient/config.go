package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type config struct {
	apiBase               string
	viewerBase            string
	username              string
	password              string
	chunks                int
	interval              time.Duration
	mediaType             string
	chunkSize             int64
	closeIncident         bool
	completeStream        bool
	downloadBundle        bool
	bundleOutput          string
	verifyBundlePath      string
	encrypt               bool
	keyFile               string
	wrappedKeyOutput      string
	contactKeyFile        string
	wrappedKeyContactID   string
	verifyBundleDecrypt   bool
	simulateFailureEvery  int
	desktopRecorder       bool
	desktopStageDir       string
	desktopResume         bool
	desktopStageOnly      bool
	desktopFailIncomplete bool
	desktopSource         string
	desktopInputFiles     []string
	desktopMaxAttempts    int
	desktopRetryDelay     time.Duration
	networkLatency        time.Duration
	networkJitter         time.Duration
	networkTimeout        time.Duration
	networkBandwidth      int64
	networkOfflineEvery   int
	networkOfflineFor     time.Duration
	networkFailureRate    float64
	networkSeed           int64
	ffmpegBin             string
	ffmpegInputFormat     string
	ffmpegInput           string
	ffmpegVideoCodec      string
	ffmpegDuration        time.Duration
	ffmpegSegmentTime     time.Duration
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("simclient", flag.ContinueOnError)

	var chunkSizeRaw string
	var networkBandwidthRaw string
	var desktopInputFiles stringListFlag
	cfg := config{}
	fs.StringVar(&cfg.apiBase, "api", defaultAPIBase, "Main API base URL")
	fs.StringVar(&cfg.viewerBase, "viewer", defaultViewerBase, "Incident viewer base URL")
	fs.StringVar(&cfg.username, "username", os.Getenv("PROOFLINE_SIM_USERNAME"), "Proofline account username")
	fs.StringVar(&cfg.password, "password", os.Getenv("PROOFLINE_SIM_PASSWORD"), "Proofline account password")
	fs.IntVar(&cfg.chunks, "chunks", defaultChunks, "Number of chunks to upload")
	fs.DurationVar(&cfg.interval, "interval", defaultInterval, "Delay between chunk uploads")
	fs.StringVar(&cfg.mediaType, "media-type", defaultMediaType, "Media type to upload")
	fs.StringVar(&chunkSizeRaw, "chunk-size", defaultChunkSize, "Size of each fake plaintext chunk before optional encryption")
	fs.BoolVar(&cfg.closeIncident, "close", false, "Close the incident when complete")
	fs.BoolVar(&cfg.completeStream, "complete-stream", true, "Mark the uploaded media stream complete")
	fs.BoolVar(&cfg.downloadBundle, "download-bundle", false, "Download the completed stream bundle through the incident viewer")
	fs.StringVar(&cfg.bundleOutput, "bundle-output", "", "Write the downloaded encrypted stream bundle ZIP to this path")
	fs.StringVar(&cfg.verifyBundlePath, "verify-bundle", "", "Verify an existing encrypted stream bundle ZIP and exit")
	fs.BoolVar(&cfg.encrypt, "encrypt", true, "Encrypt simulated chunk bytes before upload")
	fs.StringVar(&cfg.keyFile, "key-file", "", "Optional simulator encryption key file")
	fs.StringVar(&cfg.wrappedKeyOutput, "wrapped-key-output", "", "Write simulator-only contact-wrapped key metadata artifact to this path")
	fs.StringVar(&cfg.contactKeyFile, "contact-key-file", "", "Local simulator trusted-contact private key file for wrapped-key metadata")
	fs.StringVar(&cfg.wrappedKeyContactID, "wrapped-key-contact-id", defaultWrappedKeyContactID, "Local simulator trusted-contact ID for wrapped-key metadata")
	fs.BoolVar(&cfg.verifyBundleDecrypt, "verify-bundle-decryption", true, "Decrypt downloaded stream bundles locally when encryption is enabled")
	fs.IntVar(&cfg.simulateFailureEvery, "simulate-failure-every", 0, "Every Nth chunk should intentionally fail hash verification before retrying")
	fs.BoolVar(&cfg.desktopRecorder, "desktop-recorder", false, "Use durable desktop recorder simulator mode")
	fs.StringVar(&cfg.desktopStageDir, "stage-dir", "", "Durable local staging directory for desktop recorder mode")
	fs.BoolVar(&cfg.desktopResume, "resume-staged", false, "Resume uploading an existing desktop recorder staging queue")
	fs.BoolVar(&cfg.desktopStageOnly, "stage-only", false, "Stage encrypted desktop recorder chunks locally without uploading")
	fs.BoolVar(&cfg.desktopFailIncomplete, "fail-incomplete-stream", false, "Mark the desktop recorder stream failed when staged chunks cannot all upload")
	fs.StringVar(&cfg.desktopSource, "desktop-source", defaultDesktopSource, "Desktop recorder source: generated, files, or ffmpeg")
	fs.Var(&desktopInputFiles, "input-file", "Pre-recorded local input file for desktop recorder mode; may be repeated")
	fs.IntVar(&cfg.desktopMaxAttempts, "desktop-max-attempts", defaultDesktopMaxAttempts, "Maximum upload attempts per staged desktop recorder chunk")
	fs.DurationVar(&cfg.desktopRetryDelay, "desktop-retry-delay", defaultDesktopRetryDelay, "Delay before retrying a failed staged chunk upload")
	fs.DurationVar(&cfg.networkLatency, "network-latency", 0, "Simulated network latency before each request")
	fs.DurationVar(&cfg.networkJitter, "network-jitter", 0, "Additional random simulated network latency before each request")
	fs.DurationVar(&cfg.networkTimeout, "network-timeout", clientRequestTimeout, "HTTP client request timeout")
	fs.StringVar(&networkBandwidthRaw, "network-bandwidth", "", "Simulated upload bandwidth ceiling, for example 256KiB")
	fs.IntVar(&cfg.networkOfflineEvery, "network-offline-every", 0, "Fail every Nth request before sending to simulate an offline window")
	fs.DurationVar(&cfg.networkOfflineFor, "network-offline-for", 0, "Delay used with --network-offline-every before returning the simulated offline error")
	fs.Float64Var(&cfg.networkFailureRate, "network-failure-rate", 0, "Random request failure rate between 0 and 1 before sending")
	fs.Int64Var(&cfg.networkSeed, "network-seed", time.Now().UnixNano(), "Seed for deterministic poor-network simulation")
	fs.StringVar(&cfg.ffmpegBin, "ffmpeg-bin", "ffmpeg", "ffmpeg executable for desktop recorder ffmpeg source")
	fs.StringVar(&cfg.ffmpegInputFormat, "ffmpeg-input-format", defaultFFmpegInputFormat, "ffmpeg input format, such as lavfi or x11grab")
	fs.StringVar(&cfg.ffmpegInput, "ffmpeg-input", defaultFFmpegInput, "ffmpeg input source")
	fs.StringVar(&cfg.ffmpegVideoCodec, "ffmpeg-video-codec", defaultFFmpegVideoCodec, "ffmpeg video codec for segmented desktop capture")
	fs.DurationVar(&cfg.ffmpegDuration, "ffmpeg-duration", defaultFFmpegDuration, "ffmpeg capture duration")
	fs.DurationVar(&cfg.ffmpegSegmentTime, "ffmpeg-segment-time", defaultFFmpegSegmentTime, "ffmpeg segment duration for complete chunk uploads")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.desktopInputFiles = desktopInputFiles
	offlineBundleVerify := strings.TrimSpace(cfg.verifyBundlePath) != ""
	if offlineBundleVerify && cfg.keyFile == "" && strings.TrimSpace(cfg.desktopStageDir) != "" {
		cfg.keyFile = filepath.Join(cfg.desktopStageDir, "simulator-key.json")
	}
	if cfg.chunks < 0 {
		return config{}, fmt.Errorf("--chunks must be non-negative")
	}
	if cfg.interval < 0 {
		return config{}, fmt.Errorf("--interval must be non-negative")
	}
	if !validMediaType(cfg.mediaType) {
		return config{}, fmt.Errorf("--media-type must be audio, video, location, or metadata")
	}
	if !offlineBundleVerify && strings.TrimSpace(cfg.username) == "" {
		return config{}, fmt.Errorf("--username or PROOFLINE_SIM_USERNAME is required")
	}
	if !offlineBundleVerify && cfg.password == "" {
		return config{}, fmt.Errorf("--password or PROOFLINE_SIM_PASSWORD is required")
	}
	chunkSize, err := parseByteSize(chunkSizeRaw)
	if err != nil {
		return config{}, fmt.Errorf("--chunk-size: %w", err)
	}
	if chunkSize <= 0 {
		return config{}, fmt.Errorf("--chunk-size must be greater than zero")
	}
	if cfg.simulateFailureEvery < 0 {
		return config{}, fmt.Errorf("--simulate-failure-every must be non-negative")
	}
	if cfg.networkLatency < 0 {
		return config{}, fmt.Errorf("--network-latency must be non-negative")
	}
	if cfg.networkJitter < 0 {
		return config{}, fmt.Errorf("--network-jitter must be non-negative")
	}
	if cfg.networkTimeout <= 0 {
		return config{}, fmt.Errorf("--network-timeout must be greater than zero")
	}
	if cfg.networkOfflineEvery < 0 {
		return config{}, fmt.Errorf("--network-offline-every must be non-negative")
	}
	if cfg.networkOfflineFor < 0 {
		return config{}, fmt.Errorf("--network-offline-for must be non-negative")
	}
	if cfg.networkFailureRate < 0 || cfg.networkFailureRate > 1 {
		return config{}, fmt.Errorf("--network-failure-rate must be between 0 and 1")
	}
	if strings.TrimSpace(networkBandwidthRaw) != "" {
		networkBandwidth, err := parseByteSize(networkBandwidthRaw)
		if err != nil {
			return config{}, fmt.Errorf("--network-bandwidth: %w", err)
		}
		if networkBandwidth <= 0 {
			return config{}, fmt.Errorf("--network-bandwidth must be greater than zero")
		}
		cfg.networkBandwidth = networkBandwidth
	}
	if strings.TrimSpace(cfg.bundleOutput) != "" && !cfg.downloadBundle {
		return config{}, fmt.Errorf("--bundle-output requires --download-bundle")
	}
	if strings.TrimSpace(cfg.bundleOutput) != "" && !cfg.encrypt {
		return config{}, fmt.Errorf("--bundle-output requires --encrypt=true")
	}
	if offlineBundleVerify {
		if !cfg.encrypt {
			return config{}, fmt.Errorf("--verify-bundle requires --encrypt=true")
		}
		if cfg.keyFile == "" {
			return config{}, fmt.Errorf("--verify-bundle requires --key-file or --stage-dir")
		}
		if cfg.downloadBundle {
			return config{}, fmt.Errorf("--verify-bundle cannot be combined with --download-bundle")
		}
		if strings.TrimSpace(cfg.bundleOutput) != "" {
			return config{}, fmt.Errorf("--verify-bundle cannot be combined with --bundle-output")
		}
		if strings.TrimSpace(cfg.wrappedKeyOutput) != "" {
			return config{}, fmt.Errorf("--verify-bundle cannot be combined with --wrapped-key-output")
		}
		if cfg.desktopRecorder {
			return config{}, fmt.Errorf("--verify-bundle cannot be combined with --desktop-recorder")
		}
	}
	if err := applyWrappedKeyDefaults(&cfg, offlineBundleVerify); err != nil {
		return config{}, err
	}
	if cfg.downloadBundle && !cfg.completeStream {
		return config{}, fmt.Errorf("--download-bundle requires --complete-stream")
	}
	if cfg.downloadBundle && cfg.chunks == 0 && !cfg.desktopRecorder {
		return config{}, fmt.Errorf("--download-bundle requires at least one chunk")
	}
	if err := validateDesktopConfig(cfg); err != nil {
		return config{}, err
	}

	cfg.chunkSize = chunkSize
	cfg.apiBase = cleanBaseURL(cfg.apiBase)
	cfg.viewerBase = cleanBaseURL(cfg.viewerBase)
	cfg.username = strings.TrimSpace(cfg.username)
	return cfg, nil
}

func applyWrappedKeyDefaults(cfg *config, offlineBundleVerify bool) error {
	cfg.wrappedKeyContactID = strings.TrimSpace(cfg.wrappedKeyContactID)
	if !validWrappedKeyContactID(cfg.wrappedKeyContactID) {
		return fmt.Errorf("--wrapped-key-contact-id must contain only letters, numbers, underscores, and hyphens")
	}
	if strings.TrimSpace(cfg.wrappedKeyOutput) == "" {
		if strings.TrimSpace(cfg.contactKeyFile) != "" {
			return fmt.Errorf("--contact-key-file requires --wrapped-key-output")
		}
		return nil
	}
	if offlineBundleVerify {
		return fmt.Errorf("--verify-bundle cannot be combined with --wrapped-key-output")
	}
	if !cfg.encrypt {
		return fmt.Errorf("--wrapped-key-output requires --encrypt=true")
	}
	if strings.TrimSpace(cfg.contactKeyFile) == "" {
		cfg.contactKeyFile = filepath.Join(filepath.Dir(cfg.wrappedKeyOutput), defaultContactKeyFileName)
	}
	return nil
}

func validateDesktopConfig(cfg config) error {
	if !cfg.desktopRecorder {
		return nil
	}
	if strings.TrimSpace(cfg.desktopStageDir) == "" {
		return fmt.Errorf("--desktop-recorder requires --stage-dir")
	}
	if !cfg.encrypt {
		return fmt.Errorf("--desktop-recorder requires --encrypt=true")
	}
	if !cfg.completeStream && cfg.downloadBundle {
		return fmt.Errorf("--download-bundle requires --complete-stream")
	}
	if cfg.desktopMaxAttempts <= 0 {
		return fmt.Errorf("--desktop-max-attempts must be greater than zero")
	}
	if cfg.desktopRetryDelay < 0 {
		return fmt.Errorf("--desktop-retry-delay must be non-negative")
	}
	switch cfg.desktopSource {
	case desktopSourceGenerated, desktopSourceFiles, desktopSourceFFmpeg:
	default:
		return fmt.Errorf("--desktop-source must be generated, files, or ffmpeg")
	}
	if cfg.desktopResume {
		return nil
	}
	if cfg.desktopSource == desktopSourceFiles && len(cfg.desktopInputFiles) == 0 {
		return fmt.Errorf("--desktop-source=files requires at least one --input-file")
	}
	if cfg.desktopSource != desktopSourceFiles && len(cfg.desktopInputFiles) > 0 {
		return fmt.Errorf("--input-file requires --desktop-source=files")
	}
	if cfg.desktopSource == desktopSourceFFmpeg {
		if cfg.mediaType != "video" {
			return fmt.Errorf("--desktop-source=ffmpeg requires --media-type=video")
		}
		if strings.TrimSpace(cfg.ffmpegBin) == "" {
			return fmt.Errorf("--ffmpeg-bin is required")
		}
		if strings.TrimSpace(cfg.ffmpegInput) == "" {
			return fmt.Errorf("--ffmpeg-input is required")
		}
		if strings.TrimSpace(cfg.ffmpegVideoCodec) == "" {
			return fmt.Errorf("--ffmpeg-video-codec is required")
		}
		if cfg.ffmpegDuration <= 0 {
			return fmt.Errorf("--ffmpeg-duration must be greater than zero")
		}
		if cfg.ffmpegSegmentTime <= 0 {
			return fmt.Errorf("--ffmpeg-segment-time must be greater than zero")
		}
	}
	return nil
}

func parseByteSize(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("empty size")
	}

	digitsEnd := 0
	for digitsEnd < len(value) && value[digitsEnd] >= '0' && value[digitsEnd] <= '9' {
		digitsEnd++
	}
	if digitsEnd == 0 {
		return 0, fmt.Errorf("missing numeric value")
	}

	amount, err := strconv.ParseInt(value[:digitsEnd], 10, 64)
	if err != nil {
		return 0, err
	}
	unit := strings.ToLower(strings.TrimSpace(value[digitsEnd:]))
	multiplier, ok := byteSizeMultipliers()[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported unit %q", value[digitsEnd:])
	}
	if amount > 0 && multiplier > 0 && amount > (1<<63-1)/multiplier {
		return 0, fmt.Errorf("size overflows int64")
	}
	return amount * multiplier, nil
}

func byteSizeMultipliers() map[string]int64 {
	return map[string]int64{
		"":    1,
		"b":   1,
		"k":   1000,
		"kb":  1000,
		"kib": 1024,
		"m":   1000 * 1000,
		"mb":  1000 * 1000,
		"mib": 1024 * 1024,
		"g":   1000 * 1000 * 1000,
		"gb":  1000 * 1000 * 1000,
		"gib": 1024 * 1024 * 1024,
	}
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("empty value")
	}
	*f = append(*f, value)
	return nil
}
