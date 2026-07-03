package acuity

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestGetAuth(t *testing.T) {
	RegisterTestingT(t)
	_, _, err := GetAuth([]*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("authentication required"))

	uid, key, err := GetAuth([]*core.Connection{
		{Name: "user_id", Type: core.ConnectionTypeString, Value: "39751816"},
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "secret"},
	})
	Expect(err).To(BeNil())
	Expect(uid).To(Equal("39751816"))
	Expect(key).To(Equal("secret"))
}

func TestCheckResponseErrorEnvelope(t *testing.T) {
	RegisterTestingT(t)
	// The exact 403 paywall shape observed live.
	err := CheckResponse(&APIResponse{
		StatusCode: 403,
		Body:       []byte(`{"status_code":403,"message":"API access is only available on Powerhouse plans","error":"unauthorized"}`),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Powerhouse"))

	Expect(CheckResponse(&APIResponse{StatusCode: 200})).To(BeNil())

	err = CheckResponse(&APIResponse{StatusCode: 429})
	Expect(err.Error()).To(ContainSubstring("rate limit"))
}

func TestExecuteAPIBasicAuth(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Header.Get("Authorization")).To(Equal("Basic " + base64.StdEncoding.EncodeToString([]byte("39751816:secret"))))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"name":"Test"}`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	obj, err := GetObject("39751816", "secret", "/me", nil)
	Expect(err).To(BeNil())
	Expect(idOf(obj)).To(Equal("123"))
}

// TestGetListBareArray covers Acuity's bare-array collection responses.
func TestGetListBareArray(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Query().Get("max")).To(Equal("2"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1},{"id":2}]`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	q := map[string][]string{"max": {"2"}}
	items, err := GetList("39751816", "secret", "/appointments", q)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(2))
}

func TestResultShapers(t *testing.T) {
	RegisterTestingT(t)
	r := ResourceResult(map[string]interface{}{"id": float64(42)}, "done")
	Expect(r["id"]).To(Equal("42"))
	Expect(r["success"]).To(Equal(true))

	l := ListResult([]interface{}{1, 2, 3}, "listed")
	Expect(l["count"]).To(Equal(3))
	Expect(l["success"]).To(Equal(true))

	e := ErrorResult("boom")
	Expect(e["success"]).To(Equal(false))
	Expect(e["error"]).To(Equal("boom"))
}

func TestClampMax(t *testing.T) {
	RegisterTestingT(t)
	Expect(ClampMax(0, false)).To(Equal(DefaultListMax))
	Expect(ClampMax(50, true)).To(Equal(50))
	Expect(ClampMax(99999, true)).To(Equal(MaxListMax))
}
