package fillpdf

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
)

// A representative pdfcpu-exported form: text fields (string value), a checkbox
// (bool value) and a date field (string value), grouped by field type.
const sampleFormJSON = `{
  "header": {"source": "t.pdf", "version": "pdfcpu"},
  "forms": [
    {
      "textfield": [
        {"name": "company_name", "value": ""},
        {"name": "employees", "value": ""}
      ],
      "checkbox": [
        {"name": "agree", "value": false}
      ],
      "datefield": [
        {"name": "start_date", "value": ""}
      ]
    }
  ]
}`

func fieldByName(list interface{}, name string) map[string]interface{} {
	for _, e := range list.([]interface{}) {
		m := e.(map[string]interface{})
		if m["name"] == name {
			return m
		}
	}
	return nil
}

func TestMergeFormValues_SetsByNameWithTypeCoercion(t *testing.T) {
	RegisterTestingT(t)

	merged, n, err := mergeFormValues([]byte(sampleFormJSON), map[string]string{
		"company_name": "Wurkplace Ltd",
		"employees":    "42",
		"agree":        "yes",
		"unknown_pdf":  "ignored", // no matching field -> not counted
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(n).To(Equal(3))

	var doc map[string]interface{}
	Expect(json.Unmarshal(merged, &doc)).To(Succeed())
	form := doc["forms"].([]interface{})[0].(map[string]interface{})

	// Text fields keep string values.
	Expect(fieldByName(form["textfield"], "company_name")["value"]).To(Equal("Wurkplace Ltd"))
	Expect(fieldByName(form["textfield"], "employees")["value"]).To(Equal("42"))
	// Checkbox coerced to a real bool (pdfcpu requires it).
	Expect(fieldByName(form["checkbox"], "agree")["value"]).To(Equal(true))
	// Untouched field keeps its default.
	Expect(fieldByName(form["datefield"], "start_date")["value"]).To(Equal(""))
}

func TestMergeFormValues_EmptyMapAndBadJSON(t *testing.T) {
	RegisterTestingT(t)

	// No values -> nothing filled, JSON unchanged-shaped.
	_, n, err := mergeFormValues([]byte(sampleFormJSON), map[string]string{})
	Expect(err).ToNot(HaveOccurred())
	Expect(n).To(Equal(0))

	// Malformed JSON is surfaced as an error, not a panic.
	_, _, err = mergeFormValues([]byte("{not json"), map[string]string{"a": "b"})
	Expect(err).To(HaveOccurred())
}

func TestParseBool(t *testing.T) {
	RegisterTestingT(t)
	for _, s := range []string{"true", "1", "yes", "on", "checked", "Y", " TRUE "} {
		Expect(parseBool(s)).To(BeTrue(), "expected %q -> true", s)
	}
	for _, s := range []string{"false", "0", "no", "", "off", "nonsense"} {
		Expect(parseBool(s)).To(BeFalse(), "expected %q -> false", s)
	}
}
