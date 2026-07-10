package google_common

import (
	"testing"

	. "github.com/onsi/gomega"
)

// Sheets' USER_ENTERED mode coerces phone/ID-shaped strings into numbers,
// dropping leading zeros and evaluating a leading "+". ProtectPhoneLikeText
// prefixes exactly those cells with an apostrophe so they land as text,
// while leaving numbers, dates, formulas and ordinary text untouched.

func TestPhoneLikeNeedsTextGuard(t *testing.T) {
	RegisterTestingT(t)

	guarded := []string{
		"07700900123",       // UK mobile, leading zero
		"01978291000",       // UK landline
		"+447700900123",     // international
		"+44 7700 900123",   // international with spaces
		"(0770) 090-0123",   // separators + leading zero
		"00447700900123",    // 00 international prefix
		"0044",              // short leading-zero run
	}
	for _, s := range guarded {
		Expect(phoneLikeNeedsTextGuard(s)).To(BeTrue(), "expected %q to be text-guarded", s)
	}

	untouched := []string{
		"",                  // empty
		"Alice",             // name
		"hello@example.com", // email
		"123.45",            // decimal number (no leading zero)
		"42",                // integer, no leading zero
		"=SUM(A1:A2)",       // formula — respect user intent
		"-5",                // negative number
		"2026-07-10",        // ISO date
		"Line 1, Flat 2",    // address text
		"+",                 // just a plus, no digits
		"07A99",             // leading zero but not all digits
	}
	for _, s := range untouched {
		Expect(phoneLikeNeedsTextGuard(s)).To(BeFalse(), "expected %q to be left untouched", s)
	}
}

func TestAnchorAppendRange(t *testing.T) {
	RegisterTestingT(t)

	// Bare sheet names get anchored to A1 so append aligns to column A.
	Expect(AnchorAppendRange("Sheet1")).To(Equal("Sheet1!A1"))
	Expect(AnchorAppendRange("Responses")).To(Equal("Responses!A1"))

	// Explicit cell/range targets are respected (contains "!").
	Expect(AnchorAppendRange("Sheet1!A1")).To(Equal("Sheet1!A1"))
	Expect(AnchorAppendRange("Sheet1!C1")).To(Equal("Sheet1!C1"))
	Expect(AnchorAppendRange("Sheet1!A1:N1")).To(Equal("Sheet1!A1:N1"))

	// Empty passes through (caller validates required-ness separately).
	Expect(AnchorAppendRange("")).To(Equal(""))
}

func TestProtectPhoneLikeText_PrefixesOnlyPhoneCells(t *testing.T) {
	RegisterTestingT(t)

	values := [][]interface{}{
		{"Alice", "07700900123", 42},
		{"Bob", "+447700900123", "note text"},
		{"Carol", "=A1+A2", "123.45"},
	}
	ProtectPhoneLikeText(values)

	// Phone-shaped strings get an apostrophe prefix…
	Expect(values[0][1]).To(Equal("'07700900123"))
	Expect(values[1][1]).To(Equal("'+447700900123"))
	// …everything else is untouched (names, numbers, formulas, text).
	Expect(values[0][0]).To(Equal("Alice"))
	Expect(values[0][2]).To(Equal(42))
	Expect(values[1][2]).To(Equal("note text"))
	Expect(values[2][1]).To(Equal("=A1+A2"))
	Expect(values[2][2]).To(Equal("123.45"))
}
