package azure_storage_blob_set_tags

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

const testKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

func baseInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
	}
	return append(inputs, extra...)
}

// tagSet mirrors the request body's shape for round-trip assertions.
type tagSet struct {
	XMLName xml.Name `xml:"Tags"`
	Tags    []struct {
		Key   string `xml:"Key"`
		Value string `xml:"Value"`
	} `xml:"TagSet>Tag"`
}

// TestExecuteSetsTags — PUT ?comp=tags with an XML TagSet body, keys sorted so
// the payload (and therefore the signature) is deterministic.
func TestExecuteSetsTags(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotContentType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/summary final.pdf"},
		&core.Connection{Name: "tags", Type: core.ConnectionTypeObject, Value: `{"status":"final","project":"alpha"}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if gotMethod != http.MethodPut || gotPath != "/my-container/reports/summary%20final.pdf" || gotQuery != "comp=tags" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if !strings.HasPrefix(gotContentType, "application/xml") {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if !strings.HasPrefix(string(gotBody), xml.Header) {
		t.Errorf("body must open with the XML declaration: %q", gotBody)
	}

	var doc tagSet
	if err := xml.Unmarshal(gotBody, &doc); err != nil {
		t.Fatalf("body %q: %v", gotBody, err)
	}
	if len(doc.Tags) != 2 {
		t.Fatalf("body carried %d tags: %q", len(doc.Tags), gotBody)
	}
	// Sorted by key regardless of the order they were supplied in.
	if doc.Tags[0].Key != "project" || doc.Tags[0].Value != "alpha" {
		t.Errorf("tag[0] = %+v, want project=alpha first", doc.Tags[0])
	}
	if doc.Tags[1].Key != "status" || doc.Tags[1].Value != "final" {
		t.Errorf("tag[1] = %+v", doc.Tags[1])
	}

	echo := out["result"].(map[string]interface{})
	if echo["project"] != "alpha" || echo["status"] != "final" {
		t.Errorf("result = %#v", echo)
	}
	if !strings.Contains(out["tool_result"].(string), "Set 2 tags on reports/summary final.pdf") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

func TestExecuteMissingTagsIsSoftError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []*core.Connection
	}{
		{"absent", nil},
		{"empty object", []*core.Connection{{Name: "tags", Type: core.ConnectionTypeObject, Value: `{}`}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := baseInputs("http://unused.invalid",
				&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
			)
			inputs = append(inputs, tc.extra...)
			out, err := Execute(&core.Flow{}, nil, inputs)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "tags is required") {
				t.Errorf("out = %v", out)
			}
		})
	}
}

// TestExecuteValidatesTagRules — the index-tag limits are enforced client-side
// so the operator gets a readable message instead of a 400.
func TestExecuteValidatesTagRules(t *testing.T) {
	cases := []struct {
		name string
		tags string
		want string
	}{
		{"too many", `{"a":"1","b":"2","c":"3","d":"4","e":"5","f":"6","g":"7","h":"8","i":"9","j":"10","k":"11"}`, "at most 10 index tags"},
		{"illegal key charset", `{"pro*ject":"alpha"}`, "tag key"},
		{"illegal value charset", `{"project":"al*pha"}`, "tag value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
				&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
				&core.Connection{Name: "tags", Type: core.ConnectionTypeObject, Value: tc.tags},
			))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), tc.want) {
				t.Errorf("out = %v, want an error containing %q", out, tc.want)
			}
		})
	}
}

func TestExecuteAPIErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>TagsTooLarge</Code><Message>The tags provided exceed the maximum allowed size.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
		&core.Connection{Name: "tags", Type: core.ConnectionTypeObject, Value: `{"project":"alpha"}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "TagsTooLarge") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
