package aws_s3_describe_multi_region_access_point_operation

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// Required inputs are validated before any AWS call.
func TestRequiresToken(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("request_token_arn"))
}
