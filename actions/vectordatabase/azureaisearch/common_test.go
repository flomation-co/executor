// Tests for the shared Azure AI Search client: auth resolution (endpoint wins
// over service name, host-injection rejection), the api-key header + mandatory
// api-version wiring, the Azure error envelope, the OData value unwrap, the
// text/plain $count parse (with and without BOM), and key redaction.
package azureaisearch

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func conn(name, typ string, value interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: value}
}

func TestGetAuth(t *testing.T) {
	// Service name derives the public-cloud host.
	a, err := GetAuth([]*core.Connection{
		conn("service_name", core.ConnectionTypeString, "my-svc"),
		conn("api_key", core.ConnectionTypeSecret, "K3Y"),
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if a.BaseURL != "https://my-svc.search.windows.net" {
		t.Fatalf("BaseURL = %q", a.BaseURL)
	}
	if a.APIVersion != DefaultAPIVersion {
		t.Fatalf("APIVersion = %q", a.APIVersion)
	}

	// A custom endpoint wins over the service name, trailing slash trimmed.
	a, err = GetAuth([]*core.Connection{
		conn("service_name", core.ConnectionTypeString, "my-svc"),
		conn("endpoint", core.ConnectionTypeString, "https://sovereign.example.com/"),
		conn("api_key", core.ConnectionTypeSecret, "K3Y"),
		conn("api_version", core.ConnectionTypeString, "2025-05-01-preview"),
	})
	if err != nil {
		t.Fatalf("GetAuth endpoint: %v", err)
	}
	if a.BaseURL != "https://sovereign.example.com" {
		t.Fatalf("BaseURL = %q", a.BaseURL)
	}
	if a.APIVersion != "2025-05-01-preview" {
		t.Fatalf("APIVersion = %q", a.APIVersion)
	}

	// Hard failures: no key, neither name nor endpoint, a host-injection
	// service name, a non-http endpoint.
	cases := [][]*core.Connection{
		{conn("service_name", core.ConnectionTypeString, "my-svc")},
		{conn("api_key", core.ConnectionTypeSecret, "K3Y")},
		{conn("service_name", core.ConnectionTypeString, "evil.com/x?"), conn("api_key", core.ConnectionTypeSecret, "K3Y")},
		{conn("endpoint", core.ConnectionTypeString, "ftp://host"), conn("api_key", core.ConnectionTypeSecret, "K3Y")},
	}
	for i, inputs := range cases {
		if _, err := GetAuth(inputs); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

func TestExecuteAPISendsKeyAndAPIVersion(t *testing.T) {
	var gotKey, gotAccept, gotContentType, gotVersion, gotCustom, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("api-key")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		gotVersion = r.URL.Query().Get("api-version")
		gotCustom = r.URL.Query().Get("$select")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	a := Auth{BaseURL: srv.URL, APIKey: "adminkey", APIVersion: "2024-07-01"}
	q := url.Values{"$select": []string{"name"}}
	resp, err := ExecuteAPI(nil, a, http.MethodPost, "/indexes", q, map[string]interface{}{"name": "x"}, nil)
	if err != nil {
		t.Fatalf("ExecuteAPI: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if gotKey != "adminkey" {
		t.Fatalf("api-key header = %q — Azure AI Search uses the bare api-key header, not a Bearer token", gotKey)
	}
	if gotAccept != "application/json" || gotContentType != "application/json" {
		t.Fatalf("Accept/Content-Type = %q/%q", gotAccept, gotContentType)
	}
	if gotVersion != "2024-07-01" {
		t.Fatalf("api-version = %q — every call must carry it", gotVersion)
	}
	if gotCustom != "name" {
		t.Fatalf("$select = %q", gotCustom)
	}
	if gotPath != "/indexes" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestCheckResponseParsesAzureEnvelope(t *testing.T) {
	a := Auth{APIKey: "sekret"}

	// The standard envelope surfaces as "code: message".
	err := CheckResponse(a, &APIResponse{
		StatusCode: 404,
		Body:       []byte(`{"error":{"code":"ResourceNotFound","message":"The index 'x' was not found"}}`),
	})
	if err == nil || !strings.Contains(err.Error(), "ResourceNotFound: The index 'x' was not found") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "(404)") {
		t.Fatalf("status missing from %v", err)
	}

	// Non-JSON bodies are passed through truncated, with the key redacted.
	err = CheckResponse(a, &APIResponse{StatusCode: 500, Body: []byte("boom sekret boom")})
	if err == nil || strings.Contains(err.Error(), "sekret") {
		t.Fatalf("key leaked: %v", err)
	}
	if !strings.Contains(err.Error(), "********") {
		t.Fatalf("expected redaction marker in %v", err)
	}

	// 2xx (including 207 Multi-Status from /docs/index) is not an error.
	for _, code := range []int{200, 201, 204, 207} {
		if err := CheckResponse(a, &APIResponse{StatusCode: code}); err != nil {
			t.Errorf("status %d: unexpected error %v", code, err)
		}
	}
}

func TestDecodeValue(t *testing.T) {
	items, err := DecodeValue(&APIResponse{Body: []byte(`{"value":[{"name":"a"},{"name":"b"}]}`)})
	if err != nil || len(items) != 2 {
		t.Fatalf("items = %v, err = %v", items, err)
	}
	// A missing value array is an empty list, never nil.
	items, err = DecodeValue(&APIResponse{Body: []byte(`{}`)})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, err = %v", items, err)
	}
}

func TestParseCount(t *testing.T) {
	for input, want := range map[string]int64{
		"42":               42,
		" 42\n":            42,
		"\xef\xbb\xbf1005": 1005, // the service prefixes a UTF-8 BOM
		"0":                0,
	} {
		got, err := ParseCount([]byte(input))
		if err != nil || got != want {
			t.Errorf("ParseCount(%q) = %d, %v — want %d", input, got, err, want)
		}
	}
	if _, err := ParseCount([]byte(`{"count":1}`)); err == nil {
		t.Errorf("expected error for JSON body")
	}
}

func TestEscapeDocKey(t *testing.T) {
	// Single quotes are doubled per OData string-literal rules before the
	// segment is percent-encoded.
	got := EscapeDocKey("o'brien/1")
	if strings.Contains(got, "/") {
		t.Fatalf("slash survived encoding: %q", got)
	}
	if !strings.Contains(got, "''") && !strings.Contains(got, "%27%27") {
		t.Fatalf("quote not doubled: %q", got)
	}
}

func TestClampLimit(t *testing.T) {
	for _, tc := range []struct {
		limit int
		set   bool
		want  int
	}{
		{0, false, DefaultTop},
		{-3, true, DefaultTop},
		{10, true, 10},
		{99999, true, MaxTop},
	} {
		if got := ClampLimit(tc.limit, tc.set); got != tc.want {
			t.Errorf("ClampLimit(%d, %v) = %d, want %d", tc.limit, tc.set, got, tc.want)
		}
	}
}
