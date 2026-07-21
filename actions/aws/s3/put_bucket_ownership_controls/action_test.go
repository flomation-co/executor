package aws_s3_put_bucket_ownership_controls

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// Required inputs are validated before any AWS call, so an empty input set
// fails fast without loading credentials or reaching S3.
func TestRequiresBucket(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("bucket"))
}
