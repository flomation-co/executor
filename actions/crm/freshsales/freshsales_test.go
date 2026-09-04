package freshsales_common_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	core "flomation.app/automate/executor"
	contact_create "flomation.app/automate/executor/actions/crm/freshsales/contacts/contact_create"
	contact_list_by_view "flomation.app/automate/executor/actions/crm/freshsales/contacts/contact_list_by_view"
	get_selector "flomation.app/automate/executor/actions/crm/freshsales/settings/get_selector"
	freshworks_common "flomation.app/automate/executor/actions/crm/freshworks"
)

func conn(name, typ string, value interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: value}
}

func auth() []*core.Connection {
	return []*core.Connection{
		conn("api_key", core.ConnectionTypeSecret, "SECRETKEY"),
		conn("account", core.ConnectionTypeString, "widgetz"),
	}
}

// TestContactCreate_EndToEnd drives a generated action against a stub, and
// checks the three things that are easy to get wrong: the record is wrapped in
// its entity key, the id comes back out, and the payload is embedded in
// tool_result rather than only in `result`.
func TestContactCreate_EndToEnd(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		Expect(r.URL.Path).To(Equal("/crm/sales/api/contacts"))
		Expect(r.Method).To(Equal(http.MethodPost))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contact":{"id":144,"display_name":"Ada Lovelace","email":"ada@example.com"}}`))
	}))
	defer srv.Close()
	defer freshworks_common.SetHostForTest(srv.URL)()

	inputs := append(auth(),
		conn("first_name", core.ConnectionTypeString, "Ada"),
		conn("last_name", core.ConnectionTypeString, "Lovelace"),
		conn("email", core.ConnectionTypeString, "ada@example.com"),
		conn("sales_account_id", core.ConnectionTypeInteger, int64(9)),
		conn("fields", core.ConnectionTypeText, `{"custom_field":{"cf_region":"EMEA"}}`),
	)

	out, err := contact_create.Execute(nil, nil, inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("144"))

	// The entity wrapper Freshsales expects.
	record, ok := body["contact"].(map[string]interface{})
	Expect(ok).To(BeTrue(), "the payload must be wrapped in a contact key")
	Expect(record["first_name"]).To(Equal("Ada"))
	Expect(record["sales_account_id"]).To(BeNumerically("==", 9))
	// The fields escape hatch is merged over the curated inputs.
	Expect(record).To(HaveKey("custom_field"))

	// The agent reads tool_result and nothing else.
	Expect(out["tool_result"]).To(ContainSubstring("ada@example.com"))
}

// TestContactCreate_RefusesForeignAccount proves the guard is reachable from a
// real action, not just from the transport's own tests.
func TestContactCreate_RefusesForeignAccount(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		conn("api_key", core.ConnectionTypeSecret, "SECRETKEY"),
		conn("account", core.ConnectionTypeString, "evil.example.com"),
		conn("first_name", core.ConnectionTypeString, "Ada"),
	}

	out, err := contact_create.Execute(nil, nil, inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("myfreshworks.com"))
}

// TestListByView covers the filters-then-view pattern and paging.
func TestListByView(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/crm/sales/api/contacts/view/77"))
		Expect(r.URL.Query().Get("page")).To(Equal("2"))
		Expect(r.URL.Query().Get("sort_type")).To(Equal("desc"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contacts":[{"id":1,"display_name":"Ada"},{"id":2,"display_name":"Grace"}]}`))
	}))
	defer srv.Close()
	defer freshworks_common.SetHostForTest(srv.URL)()

	out, err := contact_list_by_view.Execute(nil, nil, append(auth(),
		conn("view_id", core.ConnectionTypeString, "77"),
		conn("page", core.ConnectionTypeInteger, int64(2)),
		conn("sort_type", core.ConnectionTypeString, "desc"),
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(int64(2)))
	Expect(out["tool_result"]).To(ContainSubstring("Grace"))
}

// TestSelectorRejectsUnknownName keeps the closed set closed: the value reaches
// a URL path, so an unchecked one would let a crafted input hit a different
// endpoint on the customer's account.
func TestSelectorRejectsUnknownName(t *testing.T) {
	RegisterTestingT(t)

	out, err := get_selector.Execute(nil, nil, append(auth(),
		conn("selector", core.ConnectionTypeString, "../../settings/api_keys"),
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("not a Freshsales selector"))
}

func TestSelectorReadsItsOwnKey(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/crm/sales/api/selector/deal_stages"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deal_stages":[{"id":1,"name":"Qualified"},{"id":2,"name":"Won"}]}`))
	}))
	defer srv.Close()
	defer freshworks_common.SetHostForTest(srv.URL)()

	out, err := get_selector.Execute(nil, nil, append(auth(),
		conn("selector", core.ConnectionTypeString, "deal_stages"),
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(int64(2)))
	Expect(out["tool_result"]).To(ContainSubstring("Qualified"))
}
