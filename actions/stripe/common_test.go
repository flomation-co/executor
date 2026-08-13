package stripe_common

import (
	"errors"
	"testing"

	core "flomation.app/automate/executor"
	stripe "github.com/stripe/stripe-go/v82"
	. "github.com/onsi/gomega"
)

func TestObjectResult(t *testing.T) {
	RegisterTestingT(t)

	cust := &stripe.Customer{ID: "cus_123", Email: "a@b.com"}
	res := ObjectResult(cust, "Created customer cus_123")

	Expect(res["success"]).To(BeTrue())
	Expect(res["error"]).To(Equal(""))
	Expect(res["id"]).To(Equal("cus_123"))
	// tool_result now embeds the object's JSON data after the summary so AI
	// callers (which read tool_result verbatim) get both the summary and data.
	Expect(res["tool_result"]).To(ContainSubstring("Created customer cus_123"))
	Expect(res["tool_result"]).To(ContainSubstring("cus_123"))
	Expect(res["tool_result"]).To(ContainSubstring("a@b.com"))

	// result is the JSON round-tripped object, reachable as ${input.result.<field>}
	m, ok := res["result"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(m["id"]).To(Equal("cus_123"))
	Expect(m["email"]).To(Equal("a@b.com"))
}

func TestListResult(t *testing.T) {
	RegisterTestingT(t)

	items := []map[string]interface{}{{"id": "cus_1"}, {"id": "cus_2"}}
	res := ListResult(items, true, "Listed 2 customer(s)")

	Expect(res["success"]).To(BeTrue())
	Expect(res["count"]).To(Equal(2))
	Expect(res["has_more"]).To(BeTrue())
	Expect(res["results"]).To(Equal(items))
}

func TestErrorResult(t *testing.T) {
	RegisterTestingT(t)

	res := ErrorResult("card declined")
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(Equal("card declined"))
	Expect(res["tool_result"]).To(Equal("card declined"))
}

func TestMapError_StripeError(t *testing.T) {
	RegisterTestingT(t)

	// A Stripe business error (e.g. card decline) surfaces the readable Msg as
	// a graceful success=false, not a node crash.
	se := &stripe.Error{Msg: "Your card was declined."}
	res := MapError(se)
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(Equal("Your card was declined."))
}

func TestMapError_PlainError(t *testing.T) {
	RegisterTestingT(t)

	res := MapError(errors.New("connection reset"))
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(Equal("connection reset"))
}

func TestRequiredString_Missing(t *testing.T) {
	RegisterTestingT(t)

	_, err := RequiredString("api_key", []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api_key is required"))
}

func TestGetAPIKey(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "sk_test_abc"},
	}
	key, err := GetAPIKey(inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(key).To(Equal("sk_test_abc"))
}

func TestOptionalInt64(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "amount", Type: core.ConnectionTypeInteger, Value: int64(1999)},
	}
	v := OptionalInt64("amount", inputs)
	Expect(v).ToNot(BeNil())
	Expect(*v).To(Equal(int64(1999)))

	// Absent input yields nil so the field is omitted from the Stripe request.
	Expect(OptionalInt64("missing", inputs)).To(BeNil())
}

func TestMetadata(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "order_id", "value": "42"},
			map[string]interface{}{"key": "", "value": "skipme"},
		}},
	}
	m := Metadata(inputs)
	Expect(m).To(HaveKeyWithValue("order_id", "42"))
	// blank keys are dropped
	Expect(m).ToNot(HaveKey(""))

	// No metadata input yields nil so it is omitted entirely.
	Expect(Metadata([]*core.Connection{})).To(BeNil())
}

func TestCSVToList(t *testing.T) {
	RegisterTestingT(t)

	Expect(CSVToList("")).To(BeNil())
	Expect(CSVToList(" customer , invoice ")).To(Equal([]string{"customer", "invoice"}))
}

func moneyInput(value string) []*core.Connection {
	return []*core.Connection{
		{Name: "amount", Type: core.ConnectionTypeMoney, Value: value},
	}
}

func TestMoneyToMinorUnits_TwoDecimal(t *testing.T) {
	RegisterTestingT(t)

	// GBP is a 2-decimal currency: £12.34 → 1234 pence.
	v, err := MoneyToMinorUnits("amount", "gbp", moneyInput("12.34"))
	Expect(err).ToNot(HaveOccurred())
	Expect(v).ToNot(BeNil())
	Expect(*v).To(Equal(int64(1234)))

	// Symbol, thousands separator and whitespace are tolerated.
	v, err = MoneyToMinorUnits("amount", "gbp", moneyInput(" £1,234.50 "))
	Expect(err).ToNot(HaveOccurred())
	Expect(*v).To(Equal(int64(123450)))
}

func TestMoneyToMinorUnits_ZeroDecimal(t *testing.T) {
	RegisterTestingT(t)

	// JPY has no minor unit: ¥1000 → 1000 (NOT 100000).
	v, err := MoneyToMinorUnits("amount", "jpy", moneyInput("1000"))
	Expect(err).ToNot(HaveOccurred())
	Expect(*v).To(Equal(int64(1000)))
}

func TestMoneyToMinorUnits_ThreeDecimal(t *testing.T) {
	RegisterTestingT(t)

	// KWD is a 3-decimal currency: 1.234 KWD → 1234.
	v, err := MoneyToMinorUnits("amount", "kwd", moneyInput("1.234"))
	Expect(err).ToNot(HaveOccurred())
	Expect(*v).To(Equal(int64(1234)))
}

func TestMoneyToMinorUnits_DefaultsToTwoDecimal(t *testing.T) {
	RegisterTestingT(t)

	// Unknown/blank currency falls back to 2 decimal places.
	v, err := MoneyToMinorUnits("amount", "", moneyInput("9.99"))
	Expect(err).ToNot(HaveOccurred())
	Expect(*v).To(Equal(int64(999)))
}

func TestMoneyToMinorUnits_Rounding(t *testing.T) {
	RegisterTestingT(t)

	// Float multiplication (12.34 * 100 = 1233.999…) must round, not truncate.
	v, err := MoneyToMinorUnits("amount", "usd", moneyInput("12.34"))
	Expect(err).ToNot(HaveOccurred())
	Expect(*v).To(Equal(int64(1234)))
}

func TestMoneyToMinorUnits_BlankOmitted(t *testing.T) {
	RegisterTestingT(t)

	// Absent/blank amount yields (nil, nil) so the field is omitted.
	v, err := MoneyToMinorUnits("amount", "gbp", moneyInput(""))
	Expect(err).ToNot(HaveOccurred())
	Expect(v).To(BeNil())

	v, err = MoneyToMinorUnits("amount", "gbp", []*core.Connection{})
	Expect(err).ToNot(HaveOccurred())
	Expect(v).To(BeNil())
}

func TestMoneyToMinorUnits_Invalid(t *testing.T) {
	RegisterTestingT(t)

	_, err := MoneyToMinorUnits("amount", "gbp", moneyInput("twelve"))
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("invalid amount"))

	_, err = MoneyToMinorUnits("amount", "gbp", moneyInput("-5.00"))
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("negative"))
}
