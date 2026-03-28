package cli

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	sigstoretlog "github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const testBundleVersion = "v0.3"

func newVirtualSigstoreSignedEntity(t *testing.T, artifact []byte, identity, issuer string) (*ca.VirtualSigstore, verify.SignedEntity) {
	t.Helper()

	virtualSigstore, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatalf("NewVirtualSigstore: %v", err)
	}

	entity, err := virtualSigstore.SignWithVersion(identity, issuer, artifact, testBundleVersion)
	if err != nil {
		t.Fatalf("SignWithVersion: %v", err)
	}

	return virtualSigstore, entity
}

func writeSignedEntityBundle(t *testing.T, virtualSigstore *ca.VirtualSigstore, entity verify.SignedEntity, path string) []byte {
	t.Helper()

	b := signedEntityBundle(t, virtualSigstore, entity)
	data, err := b.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return data
}

func writeVirtualSigstoreTrustedRoot(t *testing.T, virtualSigstore *ca.VirtualSigstore, path string) []byte {
	t.Helper()

	trustedRoot, err := root.NewTrustedRoot(
		root.TrustedRootMediaType01,
		virtualSigstore.FulcioCertificateAuthorities(),
		virtualSigstore.CTLogs(),
		virtualSigstore.TimestampingAuthorities(),
		virtualSigstore.RekorLogs(),
	)
	if err != nil {
		t.Fatalf("NewTrustedRoot: %v", err)
	}

	data, err := trustedRoot.MarshalJSON()
	if err != nil {
		t.Fatalf("TrustedRoot.MarshalJSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return data
}

func signedEntityBundle(t *testing.T, virtualSigstore *ca.VirtualSigstore, entity verify.SignedEntity) *sigstorebundle.Bundle {
	t.Helper()

	version, err := entity.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	mediaType, err := sigstorebundle.MediaTypeString(version)
	if err != nil {
		t.Fatalf("MediaTypeString(%q): %v", version, err)
	}

	verificationContent, err := entity.VerificationContent()
	if err != nil {
		t.Fatalf("VerificationContent: %v", err)
	}
	certificate := verificationContent.Certificate()
	if certificate == nil {
		t.Fatal("VerificationContent returned nil certificate")
	}

	signatureContent, err := entity.SignatureContent()
	if err != nil {
		t.Fatalf("SignatureContent: %v", err)
	}
	messageSignature := signatureContent.MessageSignatureContent()
	if messageSignature == nil {
		t.Fatal("SignatureContent returned nil message signature")
	}

	timestamps, err := entity.Timestamps()
	if err != nil {
		t.Fatalf("Timestamps: %v", err)
	}

	tlogEntries, err := entity.TlogEntries()
	if err != nil {
		t.Fatalf("TlogEntries: %v", err)
	}

	protoTlogEntries := make([]*protorekor.TransparencyLogEntry, 0, len(tlogEntries))
	for _, entry := range tlogEntries {
		tle := entry.TransparencyLogEntry()
		if tle.KindVersion == nil {
			var kindVersion struct {
				Kind       string `json:"kind"`
				APIVersion string `json:"apiVersion"`
			}
			if err := json.Unmarshal(tle.CanonicalizedBody, &kindVersion); err != nil {
				t.Fatalf("json.Unmarshal(tlog body): %v", err)
			}
			tle.KindVersion = &protorekor.KindVersion{
				Kind:    kindVersion.Kind,
				Version: kindVersion.APIVersion,
			}
		}
		if tle.InclusionProof == nil {
			rekorBody, err := base64.StdEncoding.DecodeString(entry.Body().(string))
			if err != nil {
				t.Fatalf("base64 decode tlog body: %v", err)
			}
			proof, err := virtualSigstore.GetInclusionProof(rekorBody)
			if err != nil {
				t.Fatalf("GetInclusionProof: %v", err)
			}
			rootHash, err := hex.DecodeString(*proof.RootHash)
			if err != nil {
				t.Fatalf("decode root hash: %v", err)
			}
			hashes := make([][]byte, 0, len(proof.Hashes))
			for _, hash := range proof.Hashes {
				hashBytes, err := hex.DecodeString(hash)
				if err != nil {
					t.Fatalf("decode proof hash: %v", err)
				}
				hashes = append(hashes, hashBytes)
			}
			tle.InclusionProof = &protorekor.InclusionProof{
				LogIndex: *proof.LogIndex,
				RootHash: rootHash,
				TreeSize: *proof.TreeSize,
				Hashes:   hashes,
				Checkpoint: &protorekor.Checkpoint{
					Envelope: *proof.Checkpoint,
				},
			}
		}
		if tle.InclusionPromise == nil {
			payload := sigstoretlog.RekorPayload{
				Body:           entry.Body(),
				IntegratedTime: tle.IntegratedTime,
				LogIndex:       tle.LogIndex,
				LogID:          hex.EncodeToString(tle.GetLogId().GetKeyId()),
			}
			signedEntryTimestamp, err := virtualSigstore.RekorSignPayload(payload)
			if err != nil {
				t.Fatalf("RekorSignPayload: %v", err)
			}
			tle.InclusionPromise = &protorekor.InclusionPromise{
				SignedEntryTimestamp: signedEntryTimestamp,
			}
		}
		protoTlogEntries = append(protoTlogEntries, tle)
	}

	protoTimestamps := make([]*protocommon.RFC3161SignedTimestamp, 0, len(timestamps))
	for _, timestamp := range timestamps {
		protoTimestamps = append(protoTimestamps, &protocommon.RFC3161SignedTimestamp{
			SignedTimestamp: append([]byte(nil), timestamp...),
		})
	}

	b, err := sigstorebundle.NewBundle(&protobundle.Bundle{
		MediaType: mediaType,
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: testHashAlgorithm(t, messageSignature.DigestAlgorithm()),
					Digest:    append([]byte(nil), messageSignature.Digest()...),
				},
				Signature: append([]byte(nil), messageSignature.Signature()...),
			},
		},
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_Certificate{
				Certificate: &protocommon.X509Certificate{
					RawBytes: append([]byte(nil), certificate.Raw...),
				},
			},
			TlogEntries: protoTlogEntries,
			TimestampVerificationData: &protobundle.TimestampVerificationData{
				Rfc3161Timestamps: protoTimestamps,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	return b
}

func testHashAlgorithm(t *testing.T, name string) protocommon.HashAlgorithm {
	t.Helper()

	switch name {
	case "SHA2_256":
		return protocommon.HashAlgorithm_SHA2_256
	case "SHA2_384":
		return protocommon.HashAlgorithm_SHA2_384
	case "SHA2_512":
		return protocommon.HashAlgorithm_SHA2_512
	default:
		t.Fatalf("unsupported digest algorithm %q", name)
		return protocommon.HashAlgorithm_HASH_ALGORITHM_UNSPECIFIED
	}
}
