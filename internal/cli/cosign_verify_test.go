package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyCosignBundleWithRoot(t *testing.T) {
	const validIdentity = "https://github.com/thgrace/training-wheels/.github/workflows/release.yml@refs/tags/v1.2.3"

	t.Run("succeeds", func(t *testing.T) {
		artifactPath := filepath.Join(t.TempDir(), "tw")
		artifactBytes := []byte("signed artifact")
		if err := os.WriteFile(artifactPath, artifactBytes, 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", artifactPath, err)
		}

		virtualSigstore, entity := newVirtualSigstoreSignedEntity(t, artifactBytes, validIdentity, expectedOIDCIssuer)
		bundlePath := filepath.Join(t.TempDir(), "tw.bundle")
		writeSignedEntityBundle(t, virtualSigstore, entity, bundlePath)

		if err := verifyCosignBundleWithRoot(artifactPath, bundlePath, virtualSigstore); err != nil {
			t.Fatalf("verifyCosignBundleWithRoot() error = %v, want nil", err)
		}
	})

	t.Run("fails for modified artifact", func(t *testing.T) {
		originalBytes := []byte("signed artifact")
		virtualSigstore, entity := newVirtualSigstoreSignedEntity(t, originalBytes, validIdentity, expectedOIDCIssuer)

		artifactPath := filepath.Join(t.TempDir(), "tw")
		if err := os.WriteFile(artifactPath, []byte("tampered artifact"), 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", artifactPath, err)
		}

		bundlePath := filepath.Join(t.TempDir(), "tw.bundle")
		writeSignedEntityBundle(t, virtualSigstore, entity, bundlePath)

		err := verifyCosignBundleWithRoot(artifactPath, bundlePath, virtualSigstore)
		if err == nil {
			t.Fatal("verifyCosignBundleWithRoot() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "signature verification failed") {
			t.Fatalf("verifyCosignBundleWithRoot() error = %q, want signature verification failure", err)
		}
	})

	t.Run("fails for unexpected SAN", func(t *testing.T) {
		artifactPath := filepath.Join(t.TempDir(), "tw")
		artifactBytes := []byte("signed artifact")
		if err := os.WriteFile(artifactPath, artifactBytes, 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", artifactPath, err)
		}

		virtualSigstore, entity := newVirtualSigstoreSignedEntity(
			t,
			artifactBytes,
			"https://github.com/thgrace/training-wheels/.github/workflows/release.yml@refs/heads/main",
			expectedOIDCIssuer,
		)
		bundlePath := filepath.Join(t.TempDir(), "tw.bundle")
		writeSignedEntityBundle(t, virtualSigstore, entity, bundlePath)

		err := verifyCosignBundleWithRoot(artifactPath, bundlePath, virtualSigstore)
		if err == nil {
			t.Fatal("verifyCosignBundleWithRoot() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "signature verification failed") {
			t.Fatalf("verifyCosignBundleWithRoot() error = %q, want signature verification failure", err)
		}
	})

	t.Run("fails for unexpected issuer", func(t *testing.T) {
		artifactPath := filepath.Join(t.TempDir(), "tw")
		artifactBytes := []byte("signed artifact")
		if err := os.WriteFile(artifactPath, artifactBytes, 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", artifactPath, err)
		}

		virtualSigstore, entity := newVirtualSigstoreSignedEntity(t, artifactBytes, validIdentity, "https://issuer.example.com")
		bundlePath := filepath.Join(t.TempDir(), "tw.bundle")
		writeSignedEntityBundle(t, virtualSigstore, entity, bundlePath)

		err := verifyCosignBundleWithRoot(artifactPath, bundlePath, virtualSigstore)
		if err == nil {
			t.Fatal("verifyCosignBundleWithRoot() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "signature verification failed") {
			t.Fatalf("verifyCosignBundleWithRoot() error = %q, want signature verification failure", err)
		}
	})

	t.Run("fails for invalid bundle JSON", func(t *testing.T) {
		artifactPath := filepath.Join(t.TempDir(), "tw")
		if err := os.WriteFile(artifactPath, []byte("signed artifact"), 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", artifactPath, err)
		}

		bundlePath := filepath.Join(t.TempDir(), "tw.bundle")
		if err := os.WriteFile(bundlePath, []byte("{not-json"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", bundlePath, err)
		}

		virtualSigstore, _ := newVirtualSigstoreSignedEntity(t, []byte("signed artifact"), validIdentity, expectedOIDCIssuer)

		err := verifyCosignBundleWithRoot(artifactPath, bundlePath, virtualSigstore)
		if err == nil {
			t.Fatal("verifyCosignBundleWithRoot() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "loading bundle") {
			t.Fatalf("verifyCosignBundleWithRoot() error = %q, want loading bundle failure", err)
		}
	})

	t.Run("fails when binary missing", func(t *testing.T) {
		artifactBytes := []byte("signed artifact")
		virtualSigstore, entity := newVirtualSigstoreSignedEntity(t, artifactBytes, validIdentity, expectedOIDCIssuer)

		bundlePath := filepath.Join(t.TempDir(), "tw.bundle")
		writeSignedEntityBundle(t, virtualSigstore, entity, bundlePath)

		err := verifyCosignBundleWithRoot(filepath.Join(t.TempDir(), "missing"), bundlePath, virtualSigstore)
		if err == nil {
			t.Fatal("verifyCosignBundleWithRoot() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "opening binary") {
			t.Fatalf("verifyCosignBundleWithRoot() error = %q, want opening binary failure", err)
		}
	})
}
