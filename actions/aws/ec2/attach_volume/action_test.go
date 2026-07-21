package aws_ec2_attach_volume

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// volume_id is required and validated before any AWS call.
func TestRequiresVolumeID(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("volume id"))
}
