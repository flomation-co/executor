package item_create

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
	. "github.com/onsi/gomega"
)

func TestItemCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotTenant, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("Xero-Tenant-Id")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Items":[{"ItemID":"i-123","Code":"WIDGET-01","Name":"Blue Widget"}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten-xyz"},
		{Name: "code", Type: core.ConnectionTypeString, Value: "WIDGET-01"},
		{Name: "name", Type: core.ConnectionTypeString, Value: "Blue Widget"},
		{Name: "is_sold", Type: core.ConnectionTypeBoolean, Value: true},
		{Name: "fields", Type: core.ConnectionTypeText, Value: `{"Description":"Standard blue widget"}`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("i-123"))
	Expect(gotPath).To(Equal("/Items"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotTenant).To(Equal("ten-xyz"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	Expect(sent["Code"]).To(Equal("WIDGET-01"))
	Expect(sent["Name"]).To(Equal("Blue Widget"))
	Expect(sent["IsSold"]).To(Equal(true))
	// advanced JSON override merged in
	Expect(sent["Description"]).To(Equal("Standard blue widget"))
}

func TestItemCreateValidationError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Type":"ValidationException","Message":"A validation exception occurred","Elements":[{"ValidationErrors":[{"Message":"Item Code must be specified"}]}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten"},
		{Name: "code", Type: core.ConnectionTypeString, Value: "x"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Item Code must be specified"))
}

func TestItemCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyXero}"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "${credentials.MyXero.tenant_id}"},
		{Name: "code", Type: core.ConnectionTypeString, Value: "WIDGET-01"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("connect and authorise a Xero account"))
}
