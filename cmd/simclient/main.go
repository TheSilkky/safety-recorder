package main

import (
	"context"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(context.Background(), os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "simclient: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out io.Writer, args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if cfg.verifyBundlePath != "" {
		return runVerifyBundle(out, cfg)
	}
	if cfg.desktopRecorder {
		return runDesktopRecorder(ctx, out, cfg)
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

	fmt.Fprintln(out, "Creating incident...")
	incidentID, err := sim.createIncident(ctx)
	if err != nil {
		return err
	}

	token, err := sim.createIncidentToken(ctx, incidentID)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Incident: %s\n", incidentID)
	fmt.Fprintln(out, "Incident viewer token created; URL omitted from output.")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Creating %s media stream...\n", cfg.mediaType)
	streamID, err := sim.createMediaStream(ctx, incidentID, cfg.mediaType)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Stream: %s\n\n", streamID)

	bundleVerification := encryption
	contactWrappedVerification := false
	if encryption.mode == envelopeModeV1 {
		if wrappedKey, ok, err := prepareContactWrappedKey(out, cfg, incidentID, streamID, encryption.v1Key); err != nil {
			return err
		} else if ok {
			bundleVerification = newV1SimulatorEncryption(wrappedKey)
			contactWrappedVerification = true
		}
	}

	if err := uploadChunks(ctx, out, sim, cfg, incidentID, streamID, encryption); err != nil {
		return err
	}

	if cfg.completeStream && cfg.chunks > 0 {
		fmt.Fprintln(out, "Completing stream...")
		if err := sim.completeMediaStream(ctx, incidentID, streamID, cfg.chunks); err != nil {
			return err
		}
		fmt.Fprintln(out, "Stream complete.")
	}

	if cfg.downloadBundle {
		if err := downloadAndVerifyBundle(ctx, out, sim, cfg, token, incidentID, streamID, bundleVerification, contactWrappedVerification); err != nil {
			return err
		}
	}

	if cfg.closeIncident {
		fmt.Fprintln(out, "Closing incident...")
		if err := sim.closeIncident(ctx, incidentID); err != nil {
			return err
		}
		fmt.Fprintln(out, "Incident closed.")
	}

	fmt.Fprintln(out, "Done.")
	return nil
}
