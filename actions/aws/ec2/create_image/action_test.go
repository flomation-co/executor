package aws_ec2_create_image

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// instance_id is required and validated before any AWS call.
func TestRequiresInstanceID(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("instance id"))
}
