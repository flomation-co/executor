package output

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecute(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		&core.Connection{
			Name:  "name",
			Type:  "string",
			Value: "some-output-name",
		},
		&core.Connection{
			Name:  "value",
			Type:  "string",
			Value: "some-output-value",
		},
	})
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeNil()))

	s, exists := result["set"]
	Expect(exists).To(BeTrue())
	Expect(s).To(BeTrue())
}

func TestExecuteMissingConnection(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeNil()))

	s, exists := result["set"]
	Expect(exists).To(BeTrue())
	Expect(s).To(BeFalse())
}
