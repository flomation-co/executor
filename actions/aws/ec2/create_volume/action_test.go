package aws_ec2_create_volume

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// availability_zone is required and validated before any AWS call, so an empty
// input set fails fast without attempting to load credentials or reach EC2.
func TestRequiresAvailabilityZone(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("availability zone"))
}
