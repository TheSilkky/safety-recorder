package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-proofline/server/internal/envelope"
)

func runVerifyBundle(out io.Writer, cfg config) error {
	encryption, err := loadExistingSimulatorEncryption(cfg)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "Encryption: enabled")
	switch encryption.mode {
	case envelopeModeV1:
		fmt.Fprintln(out, "Envelope: v1 compatibility")
		fmt.Fprintf(out, "Key ID: %s\n", encryption.v1Key.KeyID)
	}
	fmt.Fprintln(out, "Key file configured; path omitted from output.")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Verifying encrypted bundle...")
	bundleBytes, err := readEncryptedBundle(cfg.verifyBundlePath)
	if err != nil {
		return err
	}
	verified, err := verifyStreamBundleDecryption(bundleBytes, encryption, "", "", "")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Verified decrypt of %d encrypted chunks.\n", verified)
	fmt.Fprintln(out, "Done.")
	return nil
}

func loadExistingSimulatorKey(path string) (envelope.Key, error) {
	if strings.TrimSpace(path) == "" {
		return envelope.Key{}, fmt.Errorf("key file is required")
	}
	key, err := envelope.LoadKeyFile(path)
	if err != nil {
		return envelope.Key{}, safePathError("load key file", err)
	}
	return key, nil
}

func readEncryptedBundle(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("bundle path is required")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, safePathError("read encrypted bundle", err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("encrypted bundle is empty")
	}
	return body, nil
}

func writeEncryptedBundle(path string, body []byte) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("bundle output path is required")
	}
	if len(body) == 0 {
		return fmt.Errorf("encrypted bundle is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return safePathError("create bundle output directory", err)
	}
	if err := writeFileAtomicNoReplace(path, body, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("write bundle output: output file already exists")
		}
		return safePathError("write bundle output", err)
	}
	return nil
}
