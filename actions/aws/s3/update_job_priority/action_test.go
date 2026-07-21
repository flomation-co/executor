package aws_s3_update_job_priority

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// Required inputs are validated before any AWS call, so an empty input set
// fails fast without loading credentials, resolving the account, or reaching S3.
func TestRequiresJobID(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("job_id"))
}
