package freshworks_common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// TestValidateBundle_RejectsForeignHosts is the load-bearing security test.
//
// The bundle alias is operator-supplied and becomes part of every request URL,
// with the customer's API key attached. Anything that is not a Freshworks host
// must be refused BEFORE a key is ever sent.
func TestValidateBundle_RejectsForeignHosts(t *testing.T) {
	RegisterTestingT(t)

	for _, hostile := range []string{
		"evil.example.com",
		"https://evil.example.com",
		"https://evil.example.com/crm/sales/api",
		// The suffix check alone would pass these two.
		"evilmyfreshworks.com",
		"https://not-myfreshworks.com",
		"https://myfreshworks.com.evil.example.com",
		// The bare parent domain names no tenant.
		"myfreshworks.com",
		"https://myfreshworks.com",
		// Credentials-in-URL and port tricks.
		"https://widgetz.myfreshworks.com@evil.example.com",
	} {
		_, err := ValidateBundle(hostile)
		Expect(err).To(HaveOccurred(), "must refuse %q", hostile)
	}
}

func TestValidateBundle_AcceptsRealBundles(t *testing.T) {
	RegisterTestingT(t)

	for _, in := range []string{
		"widgetz",
		"Widgetz",
		"widgetz.myfreshworks.com",
		"https://widgetz.myfreshworks.com",
		"https://widgetz.myfreshworks.com/",
		"https://widgetz.myfreshworks.com/crm/sales/api/contacts/144",
		"  widgetz  ",
	} {
		got, err := ValidateBundle(in)
		Expect(err).ToNot(HaveOccurred(), "should accept %q", in)
		Expect(got).To(Equal("https://widgetz.myfreshworks.com"), "for input %q", in)
	}
}

func TestValidateBundle_EmptyIsActionable(t *testing.T) {
	RegisterTestingT(t)

	_, err := ValidateBundle("   ")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("bundle alias"))
}

// TestClientSendsTokenAuth pins the header format. "Token token=" is unusual
// enough that a plausible-looking Bearer would fail confusingly at runtime.
func TestClientSendsTokenAuth(t *testing.T) {
	RegisterTestingT(t)

	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contact":{"id":144,"display_name":"Ada"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "SECRETKEY", "/crm/sales/api")
	resp, err := client.Do(nil, http.MethodGet, "/contacts/144", nil, nil)

	Expect(err).ToNot(HaveOccurred())
	Expect(gotAuth).To(Equal("Token token=SECRETKEY"))
	Expect(gotPath).To(Equal("/crm/sales/api/contacts/144"))
	Expect(resp).To(HaveKey("contact"))
}

// TestRateLimitIsExplained covers the error a Freshworks customer is most
// likely to hit: 1000 requests per hour is low, so "429" on its own tells
// nobody what to do.
func TestRateLimitIsExplained(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", "/crm/sales/api").Do(nil, http.MethodGet, "/contacts", nil, nil)

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("1000 requests per hour"))
	var apiErr *APIError
	Expect(err).To(BeAssignableToTypeOf(apiErr))
}

func TestErrorMessagesAreReadable(t *testing.T) {
	RegisterTestingT(t)

	cases := []struct {
		status     int
		body, want string
	}{
		{http.StatusUnauthorized, `{"errors":{"message":"Invalid API key"}}`, "Invalid API key"},
		{http.StatusNotFound, `{"errors":{"message":"Contact not found"}}`, "Contact not found"},
		{http.StatusBadRequest, `{"errors":{"email":["is invalid"]}}`, "email"},
		{http.StatusInternalServerError, `something broke`, "something broke"},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		_, err := NewClient(srv.URL, "k", "/crm/sales/api").Do(nil, http.MethodGet, "/x", nil, nil)
		srv.Close()

		Expect(err).To(HaveOccurred())
		Expect(strings.ToLower(err.Error())).To(ContainSubstring(strings.ToLower(c.want)),
			"status %d should explain itself", c.status)
	}
}

// TestSetHostForTest confirms the seam relaxes validation, so sibling action
// packages can drive Execute end to end without a real Freshworks account.
func TestSetHostForTest(t *testing.T) {
	RegisterTestingT(t)

	restore := SetHostForTest("http://127.0.0.1:1234")
	got, err := ValidateBundle("anything-at-all")
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal("http://127.0.0.1:1234"))

	restore()
	_, err = ValidateBundle("evil.example.com")
	Expect(err).To(HaveOccurred(), "validation must be restored")
}
