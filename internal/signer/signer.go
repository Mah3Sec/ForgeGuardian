// Package signer implements Sigstore keyless signing, Rekor transparency log
// integration, and SLSA provenance generation for ForgeGuardian artifacts.
package signer

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"hash"
	"time"

	rekorClient "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	sigsig "github.com/sigstore/sigstore/pkg/signature"
	sigopts "github.com/sigstore/sigstore/pkg/signature/options"

	cosigncosign "github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/cosign/bundle"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// RekorPublicURL is Sigstore's public transparency log.
const RekorPublicURL = "https://rekor.sigstore.dev"

// Attestation holds the signing artefacts for a built package.
type Attestation struct {
	Package     core.PackageVersion `json:"package"`
	SHA256      string              `json:"sha256"`
	Signature   string              `json:"signature"`  // base64 DER signature
	PublicKey   string              `json:"public_key"` // PEM ECDSA public key
	RekorLogID  string              `json:"rekor_log_id,omitempty"`
	RekorIndex  int64               `json:"rekor_index,omitempty"`
	RekorURL    string              `json:"rekor_url"`
	SignedAt    time.Time           `json:"signed_at"`
	RekorBundle *bundle.RekorBundle `json:"rekor_bundle,omitempty"`
	Provenance  *SLSAProvenance     `json:"provenance,omitempty"`
}

// VerifyResult is the outcome of verifying an attestation.
type VerifyResult struct {
	Valid         bool   `json:"valid"`
	SHA256Match   bool   `json:"sha256_match"`
	RekorVerified bool   `json:"rekor_verified"`
	Error         string `json:"error,omitempty"`
}

// Signer signs artifacts and logs them to the Rekor transparency log.
type Signer struct {
	rekorURL string
}

// New creates a Signer. If rekorURL is empty, the public Rekor instance is used.
func New(rekorURL string) *Signer {
	if rekorURL == "" {
		rekorURL = RekorPublicURL
	}
	return &Signer{rekorURL: rekorURL}
}

// Sign generates an ephemeral ECDSA keypair, signs the artifact SHA256, uploads
// to Rekor, and returns a complete Attestation. Mirrors cosign keyless signing
// without requiring an OIDC token — suitable for CI environments or local use.
func (s *Signer) Sign(ctx context.Context, artifact core.BuiltArtifact, prov *SLSAProvenance) (*Attestation, error) {
	// Generate ephemeral ECDSA P-256 keypair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("signer: generate key: %w", err)
	}

	// Digest the artifact SHA256 hex string
	digest := sha256.Sum256([]byte(artifact.SHA256))

	signer, err := sigsig.LoadSigner(privKey, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("signer: load signer: %w", err)
	}

	sig, err := signer.SignMessage(nil, sigopts.WithDigest(digest[:]))
	if err != nil {
		return nil, fmt.Errorf("signer: sign: %w", err)
	}

	pubKeyPEM, err := cryptoutils.MarshalPublicKeyToPEM(privKey.Public())
	if err != nil {
		return nil, fmt.Errorf("signer: marshal pubkey: %w", err)
	}

	att := &Attestation{
		Package:    artifact.Source.Package,
		SHA256:     artifact.SHA256,
		Signature:  base64.StdEncoding.EncodeToString(sig),
		PublicKey:  string(pubKeyPEM),
		RekorURL:   s.rekorURL,
		SignedAt:   time.Now().UTC(),
		Provenance: prov,
	}

	// Upload to Rekor transparency log (best-effort: skip on failure)
	rc, err := rekorClient.GetRekorClient(s.rekorURL, rekorClient.WithUserAgent("forgeguardian/1.0"))
	if err == nil {
		var h hash.Hash = sha256.New()
		h.Write(digest[:])
		entry, uploadErr := cosigncosign.TLogUpload(ctx, rc, sig, h, pubKeyPEM)
		if uploadErr == nil && entry != nil {
			if entry.LogID != nil {
				att.RekorLogID = *entry.LogID
			}
			if entry.LogIndex != nil {
				att.RekorIndex = *entry.LogIndex
			}
			att.RekorBundle = bundle.EntryToBundle(entry)
		}
	}

	return att, nil
}

// Verify checks that an attestation's signature is valid against the stored
// public key and (optionally) confirms the Rekor log entry.
func (s *Signer) Verify(ctx context.Context, att *Attestation, artifactSHA256 string) VerifyResult {
	result := VerifyResult{}

	if att.SHA256 != artifactSHA256 {
		result.Error = fmt.Sprintf("SHA256 mismatch: attestation=%s artifact=%s", att.SHA256, artifactSHA256)
		return result
	}
	result.SHA256Match = true

	block, _ := pem.Decode([]byte(att.PublicKey))
	if block == nil {
		result.Error = "invalid public key PEM"
		return result
	}
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(att.PublicKey))
	if err != nil {
		result.Error = fmt.Sprintf("unmarshal pubkey: %v", err)
		return result
	}

	sig, err := base64.StdEncoding.DecodeString(att.Signature)
	if err != nil {
		result.Error = fmt.Sprintf("decode signature: %v", err)
		return result
	}

	digest := sha256.Sum256([]byte(att.SHA256))
	ecKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		result.Error = "public key is not ECDSA"
		return result
	}
	if !ecdsa.VerifyASN1(ecKey, digest[:], sig) {
		result.Error = "signature verification failed"
		return result
	}
	result.Valid = true

	// Optionally verify Rekor log entry is still present
	if att.RekorLogID != "" && att.RekorURL != "" {
		rc, err := rekorClient.GetRekorClient(att.RekorURL, rekorClient.WithUserAgent("forgeguardian/1.0"))
		if err == nil {
			entry, err := cosigncosign.GetTlogEntry(ctx, rc, att.RekorLogID)
			if err == nil && entry != nil {
				result.RekorVerified = true
			}
		}
	}

	return result
}

// SaveAttestation serialises an Attestation to JSON bytes.
func SaveAttestation(att *Attestation) ([]byte, error) {
	return json.MarshalIndent(att, "", "  ")
}

// LoadAttestation parses an Attestation from JSON bytes.
func LoadAttestation(data []byte) (*Attestation, error) {
	var att Attestation
	if err := json.Unmarshal(data, &att); err != nil {
		return nil, fmt.Errorf("signer: load attestation: %w", err)
	}
	return &att, nil
}

// ensure models import is used (LogEntryAnon referenced by cosign internals)
var _ *models.LogEntryAnon
