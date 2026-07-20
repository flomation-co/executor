package azure_tables_entity_batch

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
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

// batchResponse builds the multipart/mixed body the service answers a
// transaction with: an outer batch part wrapping a changeset part, each inner
// part a serialised HTTP response.
func batchResponse(inner ...string) string {
	var b strings.Builder
	b.WriteString("--batchresponse_1\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=changesetresponse_1\r\n\r\n")
	for _, status := range inner {
		b.WriteString("--changesetresponse_1\r\n")
		b.WriteString("Content-Type: application/http\r\n")
		b.WriteString("Content-Transfer-Encoding: binary\r\n\r\n")
		b.WriteString("HTTP/1.1 " + status + "\r\n")
		b.WriteString("Content-ID: 1\r\n")
		b.WriteString("Content-Length: 0\r\n\r\n")
		// The extra blank line matters: the multipart reader strips the CRLF
		// that precedes the next boundary, so without it the inner response's
		// header block arrives unterminated.
		b.WriteString("\r\n")
	}
	b.WriteString("--changesetresponse_1--\r\n")
	b.WriteString("--batchresponse_1--\r\n")
	return b.String()
}

func TestExecuteSubmitsTransaction(t *testing.T) {
	var gotPath, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotContentType = r.URL.Path, r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "multipart/mixed; boundary=batchresponse_1")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(batchResponse("204 No Content", "204 No Content")))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("actions", `[
			{"action":"upsert_merge","row":{"PartitionKey":"uk","RowKey":"1001","Total":42}},
			{"action":"delete","row":{"PartitionKey":"uk","RowKey":"1002"}}
		]`)))
	mustSucceed(t, out, err)

	if gotPath != "/devstoreaccount1/$batch" {
		t.Errorf("path = %s", gotPath)
	}
	if !strings.HasPrefix(gotContentType, "multipart/mixed; boundary=batch_") {
		t.Errorf("Content-Type = %q, want the outer batch boundary", gotContentType)
	}
	// The nested changeset is the reason this node takes the SDK rather than
	// hand-rolling the REST call.
	if !strings.Contains(gotBody, "changeset_") {
		t.Errorf("the body carries no nested changeset: %s", gotBody)
	}
	result := out["result"].(map[string]interface{})
	if result["partition_key"] != "uk" || result["count"] != 2 {
		t.Errorf("result = %v", result)
	}
}

// TestExecuteRejectsCrossPartitionBatch is the check Azurite cannot make for
// us: a batch mixing two partition keys comes back SUCCESSFUL from the
// emulator while real Azure returns a 400 that names neither key. Enforcing it
// client-side is the only way an operator finds out before production.
func TestExecuteRejectsCrossPartitionBatch(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("actions", `[
			{"action":"upsert_merge","row":{"PartitionKey":"uk","RowKey":"1001"}},
			{"action":"upsert_merge","row":{"PartitionKey":"us","RowKey":"1002"}}
		]`)))
	mustSoftFail(t, out, err, "must share a PartitionKey")
}

func TestExecuteRejectsDuplicateRowKey(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("actions", `[
			{"action":"upsert_merge","row":{"PartitionKey":"uk","RowKey":"1001"}},
			{"action":"delete","row":{"PartitionKey":"uk","RowKey":"1001"}}
		]`)))
	mustSoftFail(t, out, err, "only once")
}

func TestExecuteRejectsOversizedBatch(t *testing.T) {
	var rows []string
	for i := 0; i < 101; i++ {
		rows = append(rows, fmt.Sprintf(`{"action":"insert","row":{"PartitionKey":"uk","RowKey":"%d"}}`, i))
	}
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("actions", "["+strings.Join(rows, ",")+"]")))
	mustSoftFail(t, out, err, "capped at 100")
}

func TestExecuteRejectsUnknownAction(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("actions", `[{"action":"increment","row":{"PartitionKey":"uk","RowKey":"1001"}}]`)))
	mustSoftFail(t, out, err, "upsert_merge")
}

func TestExecuteRejectsEmptyBatch(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"), obj("actions", `[]`)))
	mustSoftFail(t, out, err, "at least one change")
}

// The service names the failing operation by INDEX. Without the annotation an
// operator gets a bare code and no idea which row broke the transaction.
func TestExecuteIndexedFailureNamesTheRow(t *testing.T) {
	srv := errorServer(http.StatusBadRequest, "1:EntityAlreadyExists")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("actions", `[
			{"action":"insert","row":{"PartitionKey":"uk","RowKey":"1001"}},
			{"action":"insert","row":{"PartitionKey":"uk","RowKey":"1002"}}
		]`)))
	mustSoftFail(t, out, err, `RowKey "1002"`)
	if msg, _ := out["error"].(string); !strings.Contains(msg, "rolled back") {
		t.Errorf("the error must say the whole transaction was rolled back: %q", msg)
	}
}
