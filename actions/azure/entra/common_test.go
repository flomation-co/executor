package entra

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func testAuthInputs(endpoint string) []*core.Connection {
	return []*core.Connection{
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "tenant-guid"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "client-guid"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "s3cr3t-value"},
		{Name: "graph_endpoint", Type: core.ConnectionTypeString, Value: endpoint},
	}
}

func TestGetAuthDefaultsAndNormalisesEndpoint(t *testing.T) {
	// Blank endpoint → global cloud.
	a, err := GetAuth(testAuthInputs(""))
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if a.Endpoint != DefaultGraphEndpoint {
		t.Fatalf("default endpoint = %q", a.Endpoint)
	}
	if a.BaseURL() != "https://graph.microsoft.com/v1.0" {
		t.Fatalf("BaseURL = %q", a.BaseURL())
	}
	// The token scope follows the endpoint so sovereign clouds get a
	// correctly-audienced token; for the default it is exactly the global one.
	if a.Scope() != "https://graph.microsoft.com/.default" {
		t.Fatalf("Scope = %q", a.Scope())
	}

	// Sovereign override: trailing slash and a pasted /v1.0 suffix are shed,
	// and the scope tracks the host.
	a, err = GetAuth(testAuthInputs("https://graph.microsoft.us/v1.0/"))
	if err != nil {
		t.Fatalf("GetAuth sovereign: %v", err)
	}
	if a.Endpoint != "https://graph.microsoft.us" {
		t.Fatalf("sovereign endpoint = %q", a.Endpoint)
	}
	if a.Scope() != "https://graph.microsoft.us/.default" {
		t.Fatalf("sovereign scope = %q", a.Scope())
	}

	// Bare host gains https.
	a, err = GetAuth(testAuthInputs("graph.microsoft.us"))
	if err != nil {
		t.Fatalf("GetAuth bare host: %v", err)
	}
	if a.Endpoint != "https://graph.microsoft.us" {
		t.Fatalf("bare-host endpoint = %q", a.Endpoint)
	}

	// A non-http scheme is rejected.
	if _, err := GetAuth(testAuthInputs("ftp://graph.example.com")); err == nil {
		t.Fatal("expected error for non-http endpoint")
	}
}

func TestGetAuthRequiresCredentials(t *testing.T) {
	for _, missing := range []string{"azure_tenant_id", "azure_client_id", "azure_client_secret"} {
		inputs := []*core.Connection{}
		for _, c := range testAuthInputs("") {
			if c.Name != missing {
				inputs = append(inputs, c)
			}
		}
		if _, err := GetAuth(inputs); err == nil || !strings.Contains(err.Error(), missing) {
			t.Errorf("missing %s: err = %v, want it named", missing, err)
		}
	}
}

func TestExecuteAPISendsBearerAndAcceptHeaders(t *testing.T) {
	var gotAuth, gotAccept, gotConsistency string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotConsistency = r.Header.Get("ConsistencyLevel")
		_, _ = w.Write([]byte(`{"id":"u1"}`))
	}))
	defer srv.Close()
	defer SetTokenForTest("tok-123")()

	a, err := GetAuth(testAuthInputs(srv.URL))
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	resp, err := ExecuteAPI(nil, a, "GET", "/users/u1", nil)
	if err != nil {
		t.Fatalf("ExecuteAPI: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if gotAuth != "Bearer tok-123" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
	// Plain (non-list) calls must NOT carry the advanced-query header.
	if gotConsistency != "" {
		t.Fatalf("ConsistencyLevel = %q on a non-list call", gotConsistency)
	}
}

func TestCheckResponseSurfacesCodeAndMessage(t *testing.T) {
	err := CheckResponse(&APIResponse{
		StatusCode: 400,
		Body:       []byte(`{"error":{"code":"Request_BadRequest","message":"Invalid object identifier 'x'."}}`),
	})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	for _, want := range []string{"400", "Request_BadRequest: Invalid object identifier"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestCheckResponseFriendlyMappings(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"resource not found",
			`{"error":{"code":"Request_ResourceNotFound","message":"Resource 'u1' does not exist."}}`,
			"check the ID/UPN",
		},
		{
			"already a member",
			`{"error":{"code":"Request_BadRequest","message":"One or more added object references already exist for the following modified properties: 'members'."}}`,
			"already a member",
		},
		{
			"license needs usageLocation",
			`{"error":{"code":"Request_BadRequest","message":"License assignment cannot be done for user with invalid usage location."}}`,
			"usageLocation",
		},
	}
	for _, tc := range cases {
		err := CheckResponse(&APIResponse{StatusCode: 400, Body: []byte(tc.body)})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}
}

func TestRedactScrubsSecretAndToken(t *testing.T) {
	a := Auth{ClientSecret: "SUPERSECRET"}
	msg := redact(a, "TOKENVALUE", "boom SUPERSECRET and TOKENVALUE here")
	if strings.Contains(msg, "SUPERSECRET") || strings.Contains(msg, "TOKENVALUE") {
		t.Fatalf("credentials not redacted: %s", msg)
	}
}

func TestListAllSendsAdvancedQueryPairAndFollowsNextLinkVerbatim(t *testing.T) {
	// ListAll walks pages sequentially, so plain slice appends are race-free.
	calls := []string{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.String())
		if r.Header.Get("ConsistencyLevel") != "eventual" {
			t.Errorf("ConsistencyLevel = %q, want eventual", r.Header.Get("ConsistencyLevel"))
		}
		if r.URL.Query().Get("$count") != "true" {
			t.Errorf("$count missing from %s", r.URL.String())
		}
		if strings.Contains(r.URL.RawQuery, "skiptoken") {
			// Second page: the nextLink must have been followed VERBATIM — the
			// caller's $top must not have been re-appended on top of it.
			_, _ = w.Write([]byte(`{"value":[{"id":"3"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"value":[{"id":"1"},{"id":"2"}],"@odata.nextLink":"` + srv.URL + `/v1.0/users?$count=true&$skiptoken=abc"}`))
	}))
	defer srv.Close()
	defer SetTokenForTest("tok")()

	a, _ := GetAuth(testAuthInputs(srv.URL))
	q := url.Values{}
	q.Set("$top", "2")
	items, next, err := ListAll(nil, a, "/users", q, true)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	if next != "" {
		t.Fatalf("nextLink = %q after an exhausted walk", next)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[1] != "/v1.0/users?$count=true&$skiptoken=abc" {
		t.Fatalf("second call = %q — the nextLink was not followed verbatim", calls[1])
	}

	// returnAll=false → single page, and the pending nextLink is reported.
	calls = nil
	items, next, err = ListAll(nil, a, "/users", url.Values{"$top": {"2"}}, false)
	if err != nil {
		t.Fatalf("ListAll single: %v", err)
	}
	if len(items) != 2 || len(calls) != 1 {
		t.Fatalf("single page: %d items in %d calls", len(items), len(calls))
	}
	if next == "" {
		t.Fatal("single page with more data should report the pending nextLink")
	}
}

func TestApplyPagingClampsTop(t *testing.T) {
	cases := []struct {
		name      string
		inputs    []*core.Connection
		wantTop   string
		returnAll bool
	}{
		{"default", nil, "50", false},
		{"clamped high", []*core.Connection{{Name: "limit", Type: core.ConnectionTypeInteger, Value: 5000}}, "999", false},
		{"explicit", []*core.Connection{{Name: "limit", Type: core.ConnectionTypeInteger, Value: 7}}, "7", false},
		{"return all pins max", []*core.Connection{
			{Name: "return_all", Type: core.ConnectionTypeBoolean, Value: true},
			{Name: "limit", Type: core.ConnectionTypeInteger, Value: 7},
		}, "999", true},
	}
	for _, tc := range cases {
		q := url.Values{}
		got := ApplyPaging(q, tc.inputs)
		if got != tc.returnAll || q.Get("$top") != tc.wantTop {
			t.Errorf("%s: returnAll=%v $top=%q, want %v/%q", tc.name, got, q.Get("$top"), tc.returnAll, tc.wantTop)
		}
	}
}

func TestChunkStrings(t *testing.T) {
	ids := make([]string, 45)
	for i := range ids {
		ids[i] = "id"
	}
	chunks := ChunkStrings(ids, 20)
	if len(chunks) != 3 || len(chunks[0]) != 20 || len(chunks[1]) != 20 || len(chunks[2]) != 5 {
		t.Fatalf("chunks = %d (%d/%d/...)", len(chunks), len(chunks[0]), len(chunks[1]))
	}
	if got := ChunkStrings(nil, 20); len(got) != 0 {
		t.Fatalf("empty input → %d chunks", len(got))
	}
}

func TestSplitCommaList(t *testing.T) {
	got := SplitCommaList(" a, ,b ,, c ")
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("SplitCommaList = %v", got)
	}
}

func TestValidators(t *testing.T) {
	if err := ValidateUPN("jane.doe@contoso.com"); err != nil {
		t.Errorf("valid UPN rejected: %v", err)
	}
	if err := ValidateUPN("no-at-sign"); err == nil {
		t.Error("UPN without @ accepted")
	}
	if err := ValidateUPN("bad space@contoso.com"); err == nil {
		t.Error("UPN with space accepted")
	}
	if err := ValidateMailNickname("jane.doe"); err != nil {
		t.Errorf("valid nickname rejected: %v", err)
	}
	if err := ValidateMailNickname("jane@doe"); err == nil {
		t.Error("nickname with @ accepted")
	}
	if err := ValidateMailNickname(strings.Repeat("x", 65)); err == nil {
		t.Error("65-char nickname accepted")
	}
	if err := ValidateDisplayName(strings.Repeat("x", 257)); err == nil {
		t.Error("257-char display name accepted")
	}
	if err := ValidateDisplayName("Jane Doe"); err != nil {
		t.Errorf("valid display name rejected: %v", err)
	}
}

func TestBoolOrDefault(t *testing.T) {
	// Untouched checkbox → the default, not false.
	if !BoolOrDefault("account_enabled", nil, true) {
		t.Fatal("nil input should yield the default true")
	}
	inputs := []*core.Connection{{Name: "account_enabled", Type: core.ConnectionTypeBoolean, Value: false}}
	if BoolOrDefault("account_enabled", inputs, true) {
		t.Fatal("explicit false should win over the default")
	}
	// A variable-bound checkbox arrives as the string "true" (see the
	// Connection.Boolean contract).
	inputs = []*core.Connection{{Name: "account_enabled", Type: core.ConnectionTypeBoolean, Value: "false"}}
	if BoolOrDefault("account_enabled", inputs, true) {
		t.Fatal("string \"false\" should parse and win over the default")
	}
}
