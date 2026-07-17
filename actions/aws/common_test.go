package aws

import (
	"context"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestConfigRequiresRegion(t *testing.T) {
	RegisterTestingT(t)

	_, err := Config(context.Background(), Credentials{AccessKey: "AKIA", SecretKey: "s"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("region"))
}

func TestConfigRequiresKeysOrRole(t *testing.T) {
	RegisterTestingT(t)

	// Region alone is not enough: with neither keys nor a role to assume there
	// is no identity to authenticate with.
	_, err := Config(context.Background(), Credentials{Region: "eu-west-2"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("role ARN"))
}

// The brokered model: assuming a role with NO user-supplied keys. The base
// identity comes from the SDK default chain (Flomation's ambient credentials);
// the provider is constructed lazily, so config building succeeds here even
// though STS is never actually called.
func TestConfigAssumeRoleWithoutUserKeys(t *testing.T) {
	RegisterTestingT(t)

	cfg, err := Config(context.Background(), Credentials{
		Region:        "eu-west-2",
		AssumeRoleARN: "arn:aws:iam::123456789012:role/FlomationAccess",
		ExternalID:    "tenant-abc-123",
	})
	Expect(err).To(BeNil())
	Expect(cfg.Region).To(Equal("eu-west-2"))
	Expect(cfg.Credentials).ToNot(BeNil())
}

func TestConfigBuildsWithStaticCredentials(t *testing.T) {
	RegisterTestingT(t)

	cfg, err := Config(context.Background(), Credentials{
		AccessKey: "AKIAEXAMPLE", SecretKey: "secret", Region: "eu-west-2",
	})
	Expect(err).To(BeNil())
	Expect(cfg.Region).To(Equal("eu-west-2"))
	Expect(cfg.Credentials).ToNot(BeNil())

	creds, err := cfg.Credentials.Retrieve(context.Background())
	Expect(err).To(BeNil())
	Expect(creds.AccessKeyID).To(Equal("AKIAEXAMPLE"))
	Expect(creds.SecretAccessKey).To(Equal("secret"))
}

func TestConfigWithAssumeRoleAndExternalIDBuilds(t *testing.T) {
	RegisterTestingT(t)

	// The assume-role provider is constructed (not invoked — STS isn't called
	// until credentials are retrieved), so config building must succeed with a
	// role ARN + external id present.
	cfg, err := Config(context.Background(), Credentials{
		AccessKey:     "AKIA",
		SecretKey:     "secret",
		Region:        "eu-west-2",
		AssumeRoleARN: "arn:aws:iam::123456789012:role/FlomationAccess",
		ExternalID:    "tenant-abc-123",
	})
	Expect(err).To(BeNil())
	Expect(cfg.Credentials).ToNot(BeNil())
}

func TestConfigFromInputsReadsStandardBlock(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		conn(InputAccessKey, "AKIA"),
		conn(InputSecretKey, "shh"),
		conn(InputRegion, "us-east-1"),
	}
	cfg, err := ConfigFromInputs(context.Background(), inputs)
	Expect(err).To(BeNil())
	Expect(cfg.Region).To(Equal("us-east-1"))
}

func TestInputStrings(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{conn("instance_ids", " i-1, i-2 ,,i-3 ")}
	Expect(InputStrings("instance_ids", inputs)).To(Equal([]string{"i-1", "i-2", "i-3"}))

	// Absent input yields nil, not a slice with an empty string.
	Expect(InputStrings("missing", inputs)).To(BeNil())
}

func TestInputStringAbsent(t *testing.T) {
	RegisterTestingT(t)
	Expect(InputString("nope", nil)).To(Equal(""))
}
