package aws_s3_copy_object

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// destination bucket and key are required and validated before any AWS call.
func TestRequiresDestination(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("destination bucket and key"))
}
