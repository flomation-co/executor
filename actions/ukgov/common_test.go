package ukgov_common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestBasicAuthHeader(t *testing.T) {
	RegisterTestingT(t)
	// Companies House: API key as username, blank password.
	Expect(BasicAuthHeader("mykey", "")).To(Equal("Basic bXlrZXk6"))
}

func TestOptionalAndRequiredString(t *testing.T) {
	RegisterTestingT(t)
	inputs := []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "cafe"},
	}
	Expect(OptionalString("name", inputs)).To(Equal("cafe"))
	Expect(OptionalString("missing", inputs)).To(Equal(""))

	v, err := RequiredString("name", inputs)
	Expect(err).To(BeNil())
	Expect(v).To(Equal("cafe"))

	_, err = RequiredString("missing", inputs)
	Expect(err).To(HaveOccurred())
}

func TestOptionalInt(t *testing.T) {
	RegisterTestingT(t)
	inputs := []*core.Connection{
		{Name: "page_size", Type: core.ConnectionTypeInteger, Value: int64(25)},
	}
	Expect(OptionalInt("page_size", inputs, 10)).To(Equal(25))
	Expect(OptionalInt("missing", inputs, 10)).To(Equal(10))
}

func TestFetch(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Header.Get("X-Test")).To(Equal("yes"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	status, body, err := Fetch(context.Background(), http.MethodGet, srv.URL, map[string]string{"X-Test": "yes"})
	Expect(err).To(BeNil())
	Expect(status).To(Equal(http.StatusOK))
	Expect(string(body)).To(ContainSubstring(`"ok":true`))
}
