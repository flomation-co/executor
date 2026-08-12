package list_languages

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestStringList(t *testing.T) {
	RegisterTestingT(t)

	// JSON array of strings -> []string, non-strings ignored.
	in := []interface{}{"English", "Spanish", 42, "French", nil}
	Expect(stringList(in)).To(Equal([]string{"English", "Spanish", "French"}))

	// non-array / nil -> empty (never nil, so downstream len() is safe).
	Expect(stringList(nil)).To(Equal([]string{}))
	Expect(stringList("English")).To(Equal([]string{}))
}
