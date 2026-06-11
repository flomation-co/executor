package arithmetic_multiplication

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecute_IntegerInputs(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "a", Value: int64(6)},
		{Name: "b", Value: int64(7)},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["answer"]).To(Equal("42"))
	Expect(out["answer_int"]).To(Equal(int64(42)))
}

func TestExecute_StringEncodedFloats(t *testing.T) {
	RegisterTestingT(t)

	// Regression for the exact user scenario:
	//   ${parent.distance_miles} = "200.08"
	//   multiplier = "0.40"
	// Previously: nil-pointer panic on Number() deref
	// Now: 80.032
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "a", Value: "200.08"},
		{Name: "b", Value: "0.40"},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["answer"]).To(Equal("80.032"))
	Expect(out["tool_result"]).To(ContainSubstring("200.08"))
	Expect(out["tool_result"]).To(ContainSubstring("0.4"))
	Expect(out["tool_result"]).To(ContainSubstring("80.032"))
}

func TestExecute_MissingInputFailsLoudly(t *testing.T) {
	RegisterTestingT(t)

	// Missing b
	_, err := Execute(nil, nil, []*core.Connection{
		{Name: "a", Value: int64(5)},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("\"b\""))

	// Non-numeric input
	_, err = Execute(nil, nil, []*core.Connection{
		{Name: "a", Value: "not a number"},
		{Name: "b", Value: int64(2)},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not a number"))
}
