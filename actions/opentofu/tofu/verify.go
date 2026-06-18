package tofu

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// opentofuSigningKey is the ASCII-armored OpenTofu release-signing public key,
// vendored into the binary so trust is anchored here rather than fetched from
// the same origin that serves the release. Sourced from
// https://get.opentofu.org/opentofu.asc.
//
//go:embed opentofu_signing_key.asc
var opentofuSigningKey []byte

// SigningKeyFingerprint is the expected fingerprint of the embedded key. It is
// asserted at load time so a swapped or corrupted key file fails loudly rather
// than silently weakening verification.
const SigningKeyFingerprint = "E3E6E43D84CB852EADB0051D0C0AF313E5FD9F80"

// verifyChecksumsSignature checks the detached GPG signature (sig) over the raw
// SHA256SUMS bytes (sums) using the embedded OpenTofu signing key. A nil return
// means the checksums file is authentic and can be trusted to verify the binary.
//
// This is what makes the subsequent SHA-256 comparison meaningful: the checksum
// is only as trustworthy as the file it comes from, and that file is now
// authenticated against a key we ship, not one downloaded alongside the binary.
func verifyChecksumsSignature(sums, sig []byte) error {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(opentofuSigningKey))
	if err != nil {
		return fmt.Errorf("loading embedded OpenTofu signing key: %w", err)
	}
	if err := assertKeyFingerprint(keyring); err != nil {
		return err
	}
	if _, err := openpgp.CheckDetachedSignature(keyring, bytes.NewReader(sums), bytes.NewReader(sig), nil); err != nil {
		return fmt.Errorf("SHA256SUMS signature does not verify against the embedded OpenTofu key: %w", err)
	}
	return nil
}

func assertKeyFingerprint(keyring openpgp.EntityList) error {
	for _, e := range keyring {
		if fmt.Sprintf("%X", e.PrimaryKey.Fingerprint) == SigningKeyFingerprint {
			return nil
		}
	}
	return fmt.Errorf("embedded signing key does not match expected fingerprint %s", SigningKeyFingerprint)
}
