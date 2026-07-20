package azure_tables_table_generate_sas

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"
)

// devKey is Azurite's well-known development account key — published in
// Microsoft's own documentation, not a secret.
const devKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// baseInputs is the Azurite-shaped credential block: the account in the URL
// path rather than the host, which is the endpoint style these tests and the
// emulator share.
func baseInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: devKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint + "/devstoreaccount1"},
	}
	return append(inputs, extra...)
}

func str(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}

func obj(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: v}
}

func flag(name string, v bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: v}
}

// errorServer answers every request with one Table service error. The
// x-ms-error-code header is what azcore reads the code from, exactly as the
// real service and Azurite send it.
func errorServer(status int, code string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-error-code", code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"odata.error":{"code":"` + code + `","message":{"value":"the service said no"}}}`))
	}))
}

func mustSoftFail(t *testing.T, out map[string]interface{}, err error, want string) {
	t.Helper()
	if err != nil {
		t.Fatalf("must be a soft failure, got hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("expected failure, got %v", out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, want) {
		t.Errorf("error = %q, want it to mention %q", msg, want)
	}
}

func mustSucceed(t *testing.T, out map[string]interface{}, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("expected success, got error %v", out["error"])
	}
}

func TestExecuteSignsSAS(t *testing.T) {
	restore := tables.SetNowForTest(time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC))
	defer restore()

	// No server: signing is local and must issue no request at all.
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://127.0.0.1:10002",
		str("table", "Orders")))
	mustSucceed(t, out, err)

	token, _ := out["sas_token"].(string)
	q, parseErr := url.ParseQuery(token)
	if parseErr != nil {
		t.Fatalf("the SAS token is not a query string: %v", parseErr)
	}
	if q.Get("sig") == "" {
		t.Errorf("no signature in the token: %s", token)
	}
	if q.Get("sp") != "r" {
		t.Errorf("permissions = %q, want the r default", q.Get("sp"))
	}
	if q.Get("tn") != "orders" {
		t.Errorf("tn = %q — the table name is lowercased into the signature", q.Get("tn"))
	}
	if out["expires_at"] != "2026-07-18T10:00:00Z" {
		t.Errorf("expires_at = %v, want 24 hours on from the pinned clock", out["expires_at"])
	}
	// The URL carries the name as typed. Lowercasing it to match the
	// signature's canonical name looks right and 404s: the service lowercases
	// for signature validation but looks the table up by the path verbatim.
	if sasURL, _ := out["sas_url"].(string); !strings.Contains(sasURL, "/devstoreaccount1/Orders?") {
		t.Errorf("sas_url = %q, want the table name as typed", sasURL)
	}
}

func TestExecuteRangeLimitsAreSigned(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://127.0.0.1:10002",
		str("table", "Orders"),
		str("permissions", "ra"),
		str("start_partition_key", "uk"),
		str("end_partition_key", "uk")))
	mustSucceed(t, out, err)

	q, _ := url.ParseQuery(out["sas_token"].(string))
	if q.Get("spk") != "uk" || q.Get("epk") != "uk" {
		t.Errorf("the partition range did not reach the token: %v", q)
	}
}

func TestExecuteRejectsBadPermissions(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://127.0.0.1:10002",
		str("table", "Orders"), str("permissions", "rwx")))
	mustSoftFail(t, out, err, "raud")
}

// A service principal cannot sign a SAS — only the account key can. The error
// must say so rather than surfacing a nil-credential panic from the SDK.
func TestExecuteEntraCannotSign(t *testing.T) {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "entra"},
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "tenant"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "client"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "secret"},
		str("table", "Orders"),
	}
	out, err := Execute(&core.Flow{}, nil, inputs)
	mustSoftFail(t, out, err, "needs the account key")
}
