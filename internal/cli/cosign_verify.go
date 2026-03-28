package cli

import (
	"fmt"
	"os"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// expectedCertIdentityRegexp matches the release workflow SAN across all tag versions.
const expectedCertIdentityRegexp = `^https://github\.com/thgrace/training-wheels/\.github/workflows/release\.yml@refs/tags/v.*`

// expectedOIDCIssuer is the GitHub Actions OIDC token issuer.
const expectedOIDCIssuer = "https://token.actions.githubusercontent.com"

// testCosignVerifyResultEnv lets subprocess tests force a verification outcome.
const testCosignVerifyResultEnv = "TW_TEST_COSIGN_VERIFY_RESULT"

// verifyCosignBundle verifies a Cosign bundle against a binary file.
// It checks that the signature is valid, the certificate was issued by Fulcio,
// the certificate identity matches the expected GitHub Actions workflow,
// and the signing event is recorded in Rekor.
func verifyCosignBundle(binaryPath, bundlePath string) error {
	if result := os.Getenv(testCosignVerifyResultEnv); result != "" {
		if result == "ok" {
			return nil
		}
		return fmt.Errorf("%s", result)
	}

	trustedRoot, err := root.FetchTrustedRoot()
	if err != nil {
		return fmt.Errorf("fetching Sigstore trusted root: %w", err)
	}
	return verifyCosignBundleWithRoot(binaryPath, bundlePath, trustedRoot)
}

// verifyCosignBundleWithRoot verifies a Cosign bundle using the provided trusted material.
// Extracted for testability — tests supply a VirtualSigstore root instead of fetching
// the real Sigstore trusted root.
func verifyCosignBundleWithRoot(binaryPath, bundlePath string, trustedMaterial root.TrustedMaterial) error {
	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return fmt.Errorf("loading bundle: %w", err)
	}

	verifier, err := verify.NewVerifier(trustedMaterial,
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("creating verifier: %w", err)
	}

	artifact, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("opening binary: %w", err)
	}
	defer artifact.Close()

	sanMatcher, err := verify.NewSANMatcher("", expectedCertIdentityRegexp)
	if err != nil {
		return fmt.Errorf("creating SAN matcher: %w", err)
	}

	certIdentity := verify.CertificateIdentity{
		SubjectAlternativeName: sanMatcher,
		Issuer: verify.IssuerMatcher{
			Issuer: expectedOIDCIssuer,
		},
	}

	policy := verify.NewPolicy(
		verify.WithArtifact(artifact),
		verify.WithCertificateIdentity(certIdentity),
	)

	_, err = verifier.Verify(b, policy)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}
