package aws_secretsmanager_get_secret_value

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// Required inputs are validated before any AWS call.
func TestRequiresSecret(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("secret id"))
}
