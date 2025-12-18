package manual

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestExecute(t *testing.T) {
	RegisterTestingT(t)

	res, err := Execute(nil, nil, nil)
	Expect(err).To(BeNil())

	start, exists := res["start"]
	Expect(exists).To(BeTrue())
	Expect(start).To(Not(BeNil()))

	quote, exists := res["quote"]
	Expect(exists).To(BeTrue())
	Expect(quote).To(Equal("To err is human"))
}
