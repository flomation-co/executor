package arithmetic_common

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestParseNumber_AcceptsIntsAndFloats(t *testing.T) {
	RegisterTestingT(t)

	// Native int64 / float64 values pass through.
	f, err := ParseNumber(&core.Connection{Value: int64(5)}, "x")
	Expect(err).ToNot(HaveOccurred())
	Expect(f).To(Equal(5.0))

	f, err = ParseNumber(&core.Connection{Value: 3.14}, "x")
	Expect(err).ToNot(HaveOccurred())
	Expect(f).To(BeNumerically("~", 3.14, 1e-9))
}

func TestParseNumber_StringEncodedFloats(t *testing.T) {
	RegisterTestingT(t)

	// This is the regression test for the route-distance bug:
	// calculate_route emits distance_miles as a string "200.08" which
	// previously crashed multiplication's *Number() deref.
	f, err := ParseNumber(&core.Connection{Value: "200.08"}, "distance_miles")
	Expect(err).ToNot(HaveOccurred())
	Expect(f).To(BeNumerically("~", 200.08, 1e-9))

	// Whitespace, leading/trailing
	f, err = ParseNumber(&core.Connection{Value: "  0.40  "}, "multiplier")
	Expect(err).ToNot(HaveOccurred())
	Expect(f).To(BeNumerically("~", 0.40, 1e-9))

	// Scientific notation
	f, err = ParseNumber(&core.Connection{Value: "1.5e3"}, "scientific")
	Expect(err).ToNot(HaveOccurred())
	Expect(f).To(Equal(1500.0))
}

func TestParseNumber_RejectsNilOrEmpty(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseNumber(nil, "missing")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("missing"))

	_, err = ParseNumber(&core.Connection{Value: ""}, "blank")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("blank"))

	_, err = ParseNumber(&core.Connection{Value: "not a number"}, "garbage")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("garbage"))
}

func TestFormatNumber_WholeAndDecimal(t *testing.T) {
	RegisterTestingT(t)

	Expect(FormatNumber(5)).To(Equal("5"))
	Expect(FormatNumber(-12)).To(Equal("-12"))
	Expect(FormatNumber(0)).To(Equal("0"))

	// Decimals don't pad with zeros
	Expect(FormatNumber(3.14)).To(Equal("3.14"))
	Expect(FormatNumber(0.4)).To(Equal("0.4"))

	// Route-distance × multiplier scenario
	Expect(FormatNumber(200.08 * 0.40)).To(Equal("80.032"))
}
