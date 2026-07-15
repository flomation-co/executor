package asset

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestAsset_PassesThroughReference(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:0123456789abcdef0123456789abcdef?size=10&type=image%2Fpng"
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "file", Type: core.ConnectionTypeFile, Value: token},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["file"]).To(Equal(token))
}

func TestAsset_EmptyIsError(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "file", Type: core.ConnectionTypeFile, Value: ""},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("no file"))
}
