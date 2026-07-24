package aws_ec2_get_password_data

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	. "github.com/onsi/gomega"
)

// encB64 encrypts a plaintext with an RSA public key (PKCS#1 v1.5, exactly as AWS
// does at launch) and base64-encodes it — the shape GetPasswordData returns.
func encB64(t *testing.T, pub *rsa.PublicKey, plain string) string {
	t.Helper()
	ct, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return base64.StdEncoding.EncodeToString(ct)
}

func TestDecryptPassword_PKCS1RoundTrip(t *testing.T) {
	RegisterTestingT(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	const pw = "Sup3r-Secret!Pa$$word"
	got, err := decryptPassword(encB64(t, &priv.PublicKey, pw), string(pemBytes))
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal(pw))
}

func TestDecryptPassword_PKCS8RoundTrip(t *testing.T) {
	RegisterTestingT(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	Expect(err).ToNot(HaveOccurred())
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	const pw = "Another-Pass-123"
	got, err := decryptPassword(encB64(t, &priv.PublicKey, pw), string(pemBytes))
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal(pw))
}

func TestDecryptPassword_Ed25519Rejected(t *testing.T) {
	RegisterTestingT(t)
	// ED25519 is not supported for Windows passwords — a clear error, not a panic.
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).ToNot(HaveOccurred())
	der, err := x509.MarshalPKCS8PrivateKey(edPriv)
	Expect(err).ToNot(HaveOccurred())
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, derr := decryptPassword(base64.StdEncoding.EncodeToString([]byte("x")), string(pemBytes))
	Expect(derr).To(HaveOccurred())
	Expect(derr.Error()).To(ContainSubstring("RSA"))
}

func TestDecryptPassword_BadInputs(t *testing.T) {
	RegisterTestingT(t)
	// Not PEM.
	_, err := decryptPassword("aGVsbG8=", "not a pem")
	Expect(err).To(HaveOccurred())
	// Not base64.
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	_, err = decryptPassword("!!!not-base64!!!", string(pemBytes))
	Expect(err).To(HaveOccurred())
}
