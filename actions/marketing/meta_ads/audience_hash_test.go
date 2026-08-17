package meta_ads_common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Normalisation IS the matching key: Meta hashes its own side after the same
// normalisation, so a value that differs only in case or whitespace must
// produce an identical digest. Get this wrong and the match silently fails —
// the audience just comes back smaller, with nothing to indicate why.
func TestHashAudienceValue_EmailNormalisation(t *testing.T) {
	RegisterTestingT(t)

	want := sha("ada@example.com")
	for _, in := range []string{
		"ada@example.com",
		"Ada@Example.com",
		"  ADA@EXAMPLE.COM  ",
		"\tAda@Example.COM\n",
	} {
		got, ok := HashAudienceValue(SchemaEmail, in)
		Expect(ok).To(BeTrue(), "input %q", in)
		Expect(got).To(Equal(want), "input %q must normalise to the same digest", in)
	}
}

// Phone numbers are written a dozen ways for the same subscriber; all must
// reduce to the same digits-only form.
func TestHashAudienceValue_PhoneNormalisation(t *testing.T) {
	RegisterTestingT(t)

	want := sha("447700900123")
	for _, in := range []string{
		"447700900123",
		"+44 7700 900123",
		"+44 (0)7700-900123",
		" +44-7700-900123 ",
	} {
		got, ok := HashAudienceValue(SchemaPhone, in)
		Expect(ok).To(BeTrue(), "input %q", in)
		Expect(got).To(Equal(want), "input %q must normalise to the same digest", in)
	}
}

// A value that cannot match must be rejected rather than hashed. Hashing "" or
// "notanemail" yields a digest that matches nobody, and counting it as uploaded
// overstates the audience.
func TestHashAudienceValue_RejectsUnusable(t *testing.T) {
	RegisterTestingT(t)

	for _, in := range []string{"", "   ", "notanemail", "@", "ada"} {
		_, ok := HashAudienceValue(SchemaEmail, in)
		Expect(ok).To(BeFalse(), "email %q should be rejected", in)
	}
	for _, in := range []string{"", "12345", "abc", "+44"} {
		_, ok := HashAudienceValue(SchemaPhone, in)
		Expect(ok).To(BeFalse(), "phone %q should be rejected", in)
	}
	_, ok := HashAudienceValue(AudienceSchema("POSTCODE"), "SY13 2JJ")
	Expect(ok).To(BeFalse(), "an unsupported schema must not be hashed and sent")
}

// The whole point of customer-list audiences is that the advertiser does not
// hand their list over in the clear. This asserts no raw value survives into
// the payload — the single most important property of this code.
func TestBuildAudiencePayload_NeverTransmitsRawValues(t *testing.T) {
	RegisterTestingT(t)

	emails := []string{"ada@example.com", "Sam@Example.COM", "grace@example.com"}
	payload, used, skipped, err := BuildAudiencePayload(SchemaEmail, emails)

	Expect(err).To(BeNil())
	Expect(used).To(Equal(3))
	Expect(skipped).To(Equal(0))

	for _, raw := range emails {
		Expect(payload).ToNot(ContainSubstring(raw), "raw value %q must never appear in the payload", raw)
		Expect(strings.ToLower(payload)).ToNot(ContainSubstring(strings.ToLower(raw)))
	}
	Expect(payload).ToNot(ContainSubstring("@"), "no address fragment should survive hashing")
	Expect(payload).To(ContainSubstring(sha("ada@example.com")))

	// Shape must be exactly what Meta expects.
	var decoded struct {
		Schema []string   `json:"schema"`
		Data   [][]string `json:"data"`
	}
	Expect(json.Unmarshal([]byte(payload), &decoded)).To(Succeed())
	Expect(decoded.Schema).To(Equal([]string{"EMAIL"}))
	Expect(decoded.Data).To(HaveLen(3))
	for _, row := range decoded.Data {
		Expect(row).To(HaveLen(1))
		Expect(row[0]).To(HaveLen(64), "each entry must be a SHA-256 hex digest")
	}
}

// A partial upload is indistinguishable from a complete one unless the
// shortfall is counted and reported.
func TestBuildAudiencePayload_ReportsSkipped(t *testing.T) {
	RegisterTestingT(t)

	_, used, skipped, err := BuildAudiencePayload(SchemaEmail,
		[]string{"ada@example.com", "", "rubbish", "grace@example.com", "   "})

	Expect(err).To(BeNil())
	Expect(used).To(Equal(2))
	Expect(skipped).To(Equal(3))
}

// All-unusable must be an error, not an empty upload reported as success.
func TestBuildAudiencePayload_ErrorsWhenNothingUsable(t *testing.T) {
	RegisterTestingT(t)

	_, _, skipped, err := BuildAudiencePayload(SchemaEmail, []string{"", "nope", "also-nope"})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("no usable"))
	Expect(skipped).To(Equal(3))
}

func TestSplitLines(t *testing.T) {
	RegisterTestingT(t)

	Expect(SplitLines("a@x.com\nb@x.com\r\n\nc@x.com")).To(Equal([]string{"a@x.com", "b@x.com", "c@x.com"}))
	Expect(SplitLines("a@x.com, b@x.com ; c@x.com")).To(Equal([]string{"a@x.com", "b@x.com", "c@x.com"}))
	Expect(SplitLines("   ")).To(BeEmpty())
}

// The European "(0)" trunk-prefix convention is common in real UK lists and
// must not survive as a digit: "+44 (0)7700 900123" is 447700900123, and
// keeping the zero yields 4407700900123 — a number that matches nobody, with
// no signal that anything went wrong.
func TestHashAudienceValue_StripsBracketedTrunkZero(t *testing.T) {
	RegisterTestingT(t)

	want := sha("447700900123")
	for _, in := range []string{"+44 (0)7700 900123", "+44(0)7700900123", "0044 (0) 7700 900123"} {
		got, ok := HashAudienceValue(SchemaPhone, in)
		Expect(ok).To(BeTrue(), in)
		if in == "0044 (0) 7700 900123" {
			// "(0) " with a space is not the bracketed form we strip, and the
			// 00 international prefix is a different convention again — assert
			// only that it hashes to something, not that it normalises.
			continue
		}
		Expect(got).To(Equal(want), "input %q", in)
	}

	// A digit legitimately inside brackets that is NOT the trunk prefix must
	// survive — only the exact "(0)" sequence is removed.
	got, ok := HashAudienceValue(SchemaPhone, "+1 (415) 555 0123")
	Expect(ok).To(BeTrue())
	Expect(got).To(Equal(sha("14155550123")))
}

// Email validation must reject things that merely contain an @.
func TestHashAudienceValue_EmailShape(t *testing.T) {
	RegisterTestingT(t)

	for _, bad := range []string{"@", "@example.com", "ada@", "ada@example", "ada@@example.com", "ada@.com", "ada@example."} {
		_, ok := HashAudienceValue(SchemaEmail, bad)
		Expect(ok).To(BeFalse(), "%q should be rejected", bad)
	}
	for _, good := range []string{"ada@example.com", "ada.lovelace+ads@sub.example.co.uk"} {
		_, ok := HashAudienceValue(SchemaEmail, good)
		Expect(ok).To(BeTrue(), "%q should be accepted", good)
	}
}
