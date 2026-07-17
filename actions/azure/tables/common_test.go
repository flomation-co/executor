package tables

// Unit coverage for the plumbing every azure/tables action depends on: auth
// resolution across the three methods, redaction, error mapping, the entity
// validators that stand between operator JSON and an SDK that panics on it,
// and the shapers.

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

// devKey is Azurite's well-known development account key — published in
// Microsoft's own documentation, not a secret.
const devKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

func conn(name, typ string, v interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: v}
}

func sasValues(table, perms string) aztables.SASSignatureValues {
	return aztables.SASSignatureValues{
		TableName:   table,
		Permissions: perms,
		ExpiryTime:  time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

func TestGetAuthSharedKeyDerivesTheEndpoint(t *testing.T) {
	auth, err := GetAuth([]*core.Connection{
		conn("account_name", core.ConnectionTypeString, "mystorageaccount"),
		conn("account_key", core.ConnectionTypeSecret, devKey),
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	// An untouched auth_method dropdown reads as "" and must mean shared key —
	// the operator has already pasted one.
	if auth.Method != AuthSharedKey {
		t.Errorf("method = %q, want shared_key for an unset dropdown", auth.Method)
	}
	if auth.ServiceURL != "https://mystorageaccount.table.core.windows.net" {
		t.Errorf("service URL = %q — Tables lives on .table., not .blob.", auth.ServiceURL)
	}
}

func TestGetAuthRejectsBadAccountName(t *testing.T) {
	_, err := GetAuth([]*core.Connection{
		conn("account_name", core.ConnectionTypeString, "My-Storage-Account"),
		conn("account_key", core.ConnectionTypeSecret, devKey),
	})
	if err == nil || !strings.Contains(err.Error(), "lowercase") {
		t.Errorf("err = %v, want the account-name rule named", err)
	}
}

func TestGetAuthCustomEndpointWins(t *testing.T) {
	auth, err := GetAuth([]*core.Connection{
		conn("account_name", core.ConnectionTypeString, "devstoreaccount1"),
		conn("account_key", core.ConnectionTypeSecret, devKey),
		conn("endpoint", core.ConnectionTypeString, "http://127.0.0.1:10002/devstoreaccount1/"),
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	// Azurite serves Tables on 10002 with the account as a PATH segment, and
	// the trailing slash must not survive into a URL we concatenate onto.
	if auth.ServiceURL != "http://127.0.0.1:10002/devstoreaccount1" {
		t.Errorf("service URL = %q", auth.ServiceURL)
	}
}

func TestGetAuthRejectsNonHTTPEndpoint(t *testing.T) {
	_, err := GetAuth([]*core.Connection{
		conn("account_name", core.ConnectionTypeString, "devstoreaccount1"),
		conn("account_key", core.ConnectionTypeSecret, devKey),
		conn("endpoint", core.ConnectionTypeString, "ftp://nope"),
	})
	if err == nil || !strings.Contains(err.Error(), "http(s)") {
		t.Errorf("err = %v", err)
	}
}

// The connection string is parsed here rather than handed to
// NewServiceClientFromConnectionString so that `endpoint` means the same thing
// under every auth method. This pins that it yields the same three facts.
func TestGetAuthConnectionString(t *testing.T) {
	auth, err := GetAuth([]*core.Connection{
		conn("auth_method", core.ConnectionTypeString, AuthConnectionString),
		conn("connection_string", core.ConnectionTypeSecret,
			"DefaultEndpointsProtocol=https;AccountName=mystorageaccount;AccountKey="+devKey+";EndpointSuffix=core.windows.net"),
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth.AccountName != "mystorageaccount" || auth.AccountKey != devKey {
		t.Errorf("auth = %+v", auth)
	}
	if auth.ServiceURL != "https://mystorageaccount.table.core.windows.net" {
		t.Errorf("service URL = %q", auth.ServiceURL)
	}
}

func TestGetAuthConnectionStringHonoursTableEndpoint(t *testing.T) {
	// This is the string Azurite hands out; TableEndpoint is what makes it
	// reach the emulator rather than the public cloud.
	auth, err := GetAuth([]*core.Connection{
		conn("auth_method", core.ConnectionTypeString, AuthConnectionString),
		conn("connection_string", core.ConnectionTypeSecret,
			"DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey="+devKey+
				";TableEndpoint=http://127.0.0.1:10002/devstoreaccount1;"),
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth.ServiceURL != "http://127.0.0.1:10002/devstoreaccount1" {
		t.Errorf("service URL = %q", auth.ServiceURL)
	}
}

func TestGetAuthExplicitEndpointBeatsConnectionString(t *testing.T) {
	auth, err := GetAuth([]*core.Connection{
		conn("auth_method", core.ConnectionTypeString, AuthConnectionString),
		conn("connection_string", core.ConnectionTypeSecret,
			"AccountName=devstoreaccount1;AccountKey="+devKey+";TableEndpoint=http://from-conn-string:10002/devstoreaccount1"),
		conn("endpoint", core.ConnectionTypeString, "http://override:10002/devstoreaccount1"),
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth.ServiceURL != "http://override:10002/devstoreaccount1" {
		t.Errorf("service URL = %q — the explicit endpoint field must win", auth.ServiceURL)
	}
}

func TestGetAuthRejectsSASConnectionString(t *testing.T) {
	_, err := GetAuth([]*core.Connection{
		conn("auth_method", core.ConnectionTypeString, AuthConnectionString),
		conn("connection_string", core.ConnectionTypeSecret,
			"AccountName=devstoreaccount1;SharedAccessSignature=sv=2019-02-02&sig=abc"),
	})
	if err == nil || !strings.Contains(err.Error(), "AccountKey") {
		t.Errorf("err = %v, want the SAS form refused by name", err)
	}
}

func TestGetAuthRejectsMalformedConnectionString(t *testing.T) {
	_, err := GetAuth([]*core.Connection{
		conn("auth_method", core.ConnectionTypeString, AuthConnectionString),
		conn("connection_string", core.ConnectionTypeSecret, "this is not a connection string"),
	})
	if err == nil {
		t.Fatal("a malformed connection string must be rejected")
	}
	// The whole string is what the operator pasted and may itself be a key.
	if strings.Contains(err.Error(), "this is not a connection string") {
		t.Errorf("the connection string was echoed back into the error: %v", err)
	}
}

func TestGetAuthEntraNeedsEveryField(t *testing.T) {
	_, err := GetAuth([]*core.Connection{
		conn("account_name", core.ConnectionTypeString, "devstoreaccount1"),
		conn("auth_method", core.ConnectionTypeString, AuthEntra),
		conn("azure_tenant_id", core.ConnectionTypeString, "tenant"),
	})
	if err == nil || !strings.Contains(err.Error(), "azure_client_id") {
		t.Errorf("err = %v, want the missing field named", err)
	}
}

func TestGetAuthRejectsUnknownMethod(t *testing.T) {
	_, err := GetAuth([]*core.Connection{
		conn("account_name", core.ConnectionTypeString, "devstoreaccount1"),
		conn("auth_method", core.ConnectionTypeString, "magic"),
	})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("err = %v", err)
	}
}

// A service principal has no key to sign a SAS with. The error must say that
// rather than letting a nil credential reach the SDK.
func TestSharedKeyCredentialUnavailableUnderEntra(t *testing.T) {
	auth := Auth{Method: AuthEntra, AccountName: "devstoreaccount1"}
	if _, err := auth.SharedKeyCredential(); err == nil || !strings.Contains(err.Error(), "needs the account key") {
		t.Errorf("err = %v", err)
	}
}

func TestSharedKeyCredentialRejectsNonBase64Key(t *testing.T) {
	auth := Auth{Method: AuthSharedKey, AccountName: "devstoreaccount1", AccountKey: "not base64!!"}
	if _, err := auth.SharedKeyCredential(); err == nil || !strings.Contains(err.Error(), "base64") {
		t.Errorf("err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// Redaction
// ---------------------------------------------------------------------------

func TestRedactScrubsEveryCredential(t *testing.T) {
	auth := Auth{
		AccountKey:   devKey,
		ClientSecret: "s3cret-value",
		connString:   "AccountName=x;AccountKey=" + devKey,
	}
	msg := auth.redact("failed with key " + devKey + " secret s3cret-value string AccountName=x;AccountKey=" + devKey)
	for _, leak := range []string{devKey, "s3cret-value"} {
		if strings.Contains(msg, leak) {
			t.Errorf("%q survived redaction: %s", leak, msg)
		}
	}
}

// Only sig= is a credential in a SAS. The rest (sv/sp/se) says WHICH link was
// used, which is provenance worth keeping.
func TestRedactScrubsOnlyTheSASSignature(t *testing.T) {
	msg := Auth{}.redact("GET https://x.table.core.windows.net/t?sv=2019-02-02&sp=r&sig=abc%2Bdef123 failed")
	if strings.Contains(msg, "abc%2Bdef123") {
		t.Errorf("the SAS signature leaked: %s", msg)
	}
	if !strings.Contains(msg, "sv=2019-02-02") || !strings.Contains(msg, "sp=r") {
		t.Errorf("the non-secret SAS params were scrubbed too: %s", msg)
	}
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// responseErr builds the error azcore raises for a non-2xx, as close to the
// real shape as matters: ResponseError.Error() walks RawResponse.Request.URL,
// so a fixture without one panics rather than failing.
func responseErr(status int, code string) error {
	u, _ := url.Parse("https://devstoreaccount1.table.core.windows.net/Orders")
	return &azcore.ResponseError{
		StatusCode: status,
		ErrorCode:  code,
		RawResponse: &http.Response{
			StatusCode: status,
			Body:       http.NoBody,
			Request:    &http.Request{Method: http.MethodGet, URL: u},
		},
	}
}

func TestErrorfMapsTheCodesAnOperatorHits(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"UpdateConditionNotSatisfied", "modified since it was read"},
		{"TableBeingDeleted", "40 seconds"},
		{"AuthorizationPermissionMismatch", "Storage Table Data Contributor"},
		{"EntityAlreadyExists", "Upsert Row"},
		{"InvalidAuthenticationInfo", "10002"},
	}
	for _, c := range cases {
		got := Auth{}.Errorf(responseErr(http.StatusConflict, c.code))
		if !strings.Contains(got, c.want) {
			t.Errorf("Errorf(%s) = %q, want it to mention %q", c.code, got, c.want)
		}
		if !strings.Contains(got, c.code) {
			t.Errorf("Errorf(%s) = %q — the service's own code must survive for anyone searching it", c.code, got)
		}
	}
}

// A batch failure arrives as "1:EntityAlreadyExists". The index is stripped so
// the code still maps; BatchErrorf reports which change that was.
func TestErrorfMapsAnIndexedBatchCode(t *testing.T) {
	got := Auth{}.Errorf(responseErr(http.StatusBadRequest, "1:EntityAlreadyExists"))
	if !strings.Contains(got, "Upsert Row") {
		t.Errorf("Errorf = %q, want the indexed code to still map", got)
	}
}

func TestErrorfTruncatesANonResponseError(t *testing.T) {
	got := Auth{}.Errorf(errors.New(strings.Repeat("x", 900)))
	if len(got) > 600 {
		t.Errorf("an unbounded error reached the output: %d chars", len(got))
	}
	if !strings.Contains(got, "request failed") {
		t.Errorf("got = %q", got)
	}
}

func TestErrorCodeAndStatusIgnoreAPlainError(t *testing.T) {
	if code := ErrorCode(errors.New("dial tcp: connection refused")); code != "" {
		t.Errorf("ErrorCode = %q, want empty for a transport error", code)
	}
	if status := StatusCode(errors.New("nope")); status != 0 {
		t.Errorf("StatusCode = %d", status)
	}
}

func TestIsNotFoundAcceptsEitherCodeOrStatus(t *testing.T) {
	// The service is inconsistent about which name it uses.
	for _, code := range []string{"ResourceNotFound", "EntityNotFound", "TableNotFound"} {
		if !IsNotFound(responseErr(http.StatusNotFound, code)) {
			t.Errorf("IsNotFound(%s) = false", code)
		}
	}
	if !IsNotFound(responseErr(http.StatusNotFound, "")) {
		t.Error("a bare 404 with no code must still read as not-found")
	}
	if IsNotFound(responseErr(http.StatusConflict, "TableAlreadyExists")) {
		t.Error("a 409 is not a 404")
	}
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func TestValidateTableName(t *testing.T) {
	valid := []string{"Orders", "MyTable1", "abc"}
	for _, n := range valid {
		if err := ValidateTableName(n); err != nil {
			t.Errorf("ValidateTableName(%q) = %v, want nil", n, err)
		}
	}
	// The hyphen case is the one that matters: blob CONTAINER names take
	// hyphens and table names do not, so this is the mistake an operator
	// arriving from Blob Storage makes.
	invalid := []string{"my-table", "1Table", "ab", "my_table", "", strings.Repeat("a", 64)}
	for _, n := range invalid {
		if err := ValidateTableName(n); err == nil {
			t.Errorf("ValidateTableName(%q) = nil, want an error", n)
		}
	}
}

func TestValidateKey(t *testing.T) {
	if err := ValidateKey("partition_key", "uk south"); err != nil {
		t.Errorf("a space is legal in a key: %v", err)
	}
	for _, bad := range []string{"a/b", `a\b`, "a#b", "a?b", "a\x01b", "", strings.Repeat("k", 1025)} {
		if err := ValidateKey("row_key", bad); err == nil {
			t.Errorf("ValidateKey(%q) = nil, want an error", bad)
		}
	}
}

// ---------------------------------------------------------------------------
// Entities — the panic guard
// ---------------------------------------------------------------------------

// aztables asserts entity["PartitionKey"].(string) with no comma-ok, so every
// case here is a live executor crash if it reaches the SDK. The entity JSON
// comes from operator input and flow variables, so all of them are reachable.
func TestEntityKeysRejectsEverythingTheSDKWouldPanicOn(t *testing.T) {
	cases := []struct {
		name   string
		entity map[string]interface{}
		want   string
	}{
		{"missing PartitionKey", map[string]interface{}{"RowKey": "1"}, "no PartitionKey"},
		{"missing RowKey", map[string]interface{}{"PartitionKey": "p"}, "no RowKey"},
		{"numeric PartitionKey", map[string]interface{}{"PartitionKey": float64(42), "RowKey": "1"}, "must be a string"},
		{"null RowKey", map[string]interface{}{"PartitionKey": "p", "RowKey": nil}, "must be a string"},
		{"object RowKey", map[string]interface{}{"PartitionKey": "p", "RowKey": map[string]interface{}{}}, "must be a string"},
		{"empty PartitionKey", map[string]interface{}{"PartitionKey": "", "RowKey": "1"}, "must not be empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := EntityKeys(c.entity)
			if err == nil {
				t.Fatalf("EntityKeys accepted %v — the SDK would panic on it", c.entity)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestParseEntityRejectsNonObjects(t *testing.T) {
	for _, raw := range []string{`[1,2]`, `"a string"`, `42`} {
		_, _, err := ParseEntity([]*core.Connection{conn("entity", core.ConnectionTypeObject, raw)}, "entity")
		if err == nil || !strings.Contains(err.Error(), "must be a JSON object") {
			t.Errorf("ParseEntity(%s) err = %v", raw, err)
		}
	}
}

func TestParseEntityRejectsMalformedJSON(t *testing.T) {
	_, _, err := ParseEntity([]*core.Connection{conn("entity", core.ConnectionTypeObject, `{"PartitionKey":`)}, "entity")
	if err == nil || !strings.Contains(err.Error(), "valid JSON") {
		t.Errorf("err = %v", err)
	}
}

func TestParseEntityRequiresTheInput(t *testing.T) {
	if _, _, err := ParseEntity(nil, "entity"); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// Shaping
// ---------------------------------------------------------------------------

func TestShapeEntityDropsTheNoiseAndKeepsTheRest(t *testing.T) {
	raw := []byte(`{
		"odata.metadata":"http://127.0.0.1:10002/devstoreaccount1/$metadata#Orders/@Element",
		"odata.etag":"W/\"from-body\"",
		"odata.type":"devstoreaccount1.Orders",
		"PartitionKey":"uk","RowKey":"1001",
		"Timestamp":"2026-07-17T10:00:00Z",
		"Big":"9007199254740993","Big@odata.type":"Edm.Int64"
	}`)
	entity, err := ShapeEntity(raw, "")
	if err != nil {
		t.Fatalf("ShapeEntity: %v", err)
	}
	for _, gone := range []string{"odata.metadata", "odata.etag", "odata.type"} {
		if _, present := entity[gone]; present {
			t.Errorf("%s survived shaping: %v", gone, entity)
		}
	}
	if entity["etag"] != `W/"from-body"` {
		t.Errorf("etag = %v, want it lifted out of odata.etag", entity["etag"])
	}
	if entity["Timestamp"] == nil {
		t.Error("Timestamp is the operator's data, not noise")
	}
	// Strip the sidecar and an Int64 read here and written back by Upsert Row
	// silently becomes a Double.
	if entity["Big@odata.type"] != "Edm.Int64" {
		t.Errorf("the EDM type sidecar must survive so the value round-trips: %v", entity)
	}
}

func TestShapeEntityPrefersTheHeaderETag(t *testing.T) {
	entity, err := ShapeEntity([]byte(`{"odata.etag":"W/\"body\"","PartitionKey":"p","RowKey":"r"}`), `W/"header"`)
	if err != nil {
		t.Fatalf("ShapeEntity: %v", err)
	}
	if entity["etag"] != `W/"header"` {
		t.Errorf("etag = %v, want the response header to win", entity["etag"])
	}
}

func TestShapeEntityHandlesAnEmptyBody(t *testing.T) {
	entity, err := ShapeEntity(nil, `W/"x"`)
	if err != nil {
		t.Fatalf("ShapeEntity: %v", err)
	}
	if entity["etag"] != `W/"x"` || len(entity) != 1 {
		t.Errorf("entity = %v", entity)
	}
}

func TestSelectFieldsKeepsTheIdentityAndSidecars(t *testing.T) {
	entity := map[string]interface{}{
		"PartitionKey": "uk", "RowKey": "1001", "etag": "W/\"x\"",
		"Total": 42, "Big": "9007199254740993", "Big@odata.type": "Edm.Int64",
		"Customer": "Acme",
	}
	got := SelectFields(entity, "Total, Big")
	for _, want := range []string{"PartitionKey", "RowKey", "etag", "Total", "Big", "Big@odata.type"} {
		if _, present := got[want]; !present {
			t.Errorf("%s was dropped: %v", want, got)
		}
	}
	if _, present := got["Customer"]; present {
		t.Errorf("Customer was not selected: %v", got)
	}
}

func TestEntityIDJoinsBothKeys(t *testing.T) {
	if got := EntityID("uk", "1001"); got != "uk/1001" {
		t.Errorf("EntityID = %q", got)
	}
}

// ---------------------------------------------------------------------------
// Update mode
// ---------------------------------------------------------------------------

// Blank MUST mean merge. If it ever meant replace, every operator who left the
// dropdown alone would silently delete the columns they did not mention.
func TestUpdateModeForDefaultsToMerge(t *testing.T) {
	mode, err := UpdateModeFor(nil)
	if err != nil || mode != "merge" {
		t.Errorf("mode = %q, err = %v — an unset dropdown must merge", mode, err)
	}
}

func TestUpdateModeForRejectsRubbish(t *testing.T) {
	_, err := UpdateModeFor([]*core.Connection{conn("update_mode", core.ConnectionTypeString, "clobber")})
	if err == nil || !strings.Contains(err.Error(), "merge or replace") {
		t.Errorf("err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// Paging
// ---------------------------------------------------------------------------

func TestPageLimitClamps(t *testing.T) {
	if got := PageLimit(nil); got != DefaultPageLimit {
		t.Errorf("unset limit = %d, want %d", got, DefaultPageLimit)
	}
	if got := PageLimit([]*core.Connection{conn("limit", core.ConnectionTypeInteger, 99999)}); got != MaxPageLimit {
		t.Errorf("limit = %d, want the %d page cap", got, MaxPageLimit)
	}
	if got := PageLimit([]*core.Connection{conn("limit", core.ConnectionTypeInteger, 0)}); got != DefaultPageLimit {
		t.Errorf("zero limit = %d, want the default", got)
	}
}

// A table big enough to page forever must not be able to spin unbounded
// requests. The cap is what makes the worst case finite, and reporting it is
// what stops a truncated result reading as a complete one.
func TestWalkPagesStopsAtTheSafetyCap(t *testing.T) {
	calls := 0
	items, capped, err := WalkPages(nil, true,
		func() bool { return true }, // the service always offers another page
		func() (interface{}, error) {
			calls++
			return []interface{}{calls}, nil
		})
	if err != nil {
		t.Fatalf("WalkPages: %v", err)
	}
	if calls != maxListPages {
		t.Errorf("made %d requests, want the %d-page cap", calls, maxListPages)
	}
	if !capped {
		t.Error("capped = false — a truncated walk must say so or it reads as complete")
	}
	if len(items) != maxListPages {
		t.Errorf("collected %d items", len(items))
	}
}

func TestWalkPagesSinglePageIgnoresMore(t *testing.T) {
	calls := 0
	_, capped, err := WalkPages(nil, false,
		func() bool { return true },
		func() (interface{}, error) { calls++; return []interface{}{1}, nil })
	if err != nil || calls != 1 || capped {
		t.Errorf("calls = %d, capped = %v, err = %v — without return_all exactly one page", calls, capped, err)
	}
}

func TestWalkPagesPropagatesTheError(t *testing.T) {
	want := errors.New("boom")
	if _, _, err := WalkPages(nil, true, func() bool { return true },
		func() (interface{}, error) { return nil, want }); !errors.Is(err, want) {
		t.Errorf("err = %v", err)
	}
}

func TestListSummaryReportsACappedWalk(t *testing.T) {
	if got := ListSummary("row", 5, true, true); !strings.Contains(got, "safety cap") {
		t.Errorf("got = %q", got)
	}
	if got := ListSummary("row", 5, true, false); !strings.Contains(got, "all 5") {
		t.Errorf("got = %q", got)
	}
	if got := ListSummary("row", 5, false, false); !strings.Contains(got, "Found 5") {
		t.Errorf("got = %q", got)
	}
}

// ---------------------------------------------------------------------------
// Batch parsing
// ---------------------------------------------------------------------------

func batchInput(raw string) []*core.Connection {
	return []*core.Connection{conn("actions", core.ConnectionTypeObject, raw)}
}

func TestParseBatchAcceptsEveryActionVerb(t *testing.T) {
	batch, err := ParseBatch(batchInput(`[
		{"action":"insert","row":{"PartitionKey":"uk","RowKey":"1"}},
		{"action":"merge","row":{"PartitionKey":"uk","RowKey":"2"}},
		{"action":"replace","row":{"PartitionKey":"uk","RowKey":"3"}},
		{"action":"upsert_merge","row":{"PartitionKey":"uk","RowKey":"4"}},
		{"action":"upsert_replace","row":{"PartitionKey":"uk","RowKey":"5"}},
		{"action":"delete","row":{"PartitionKey":"uk","RowKey":"6"}}
	]`), "actions")
	if err != nil {
		t.Fatalf("ParseBatch: %v", err)
	}
	if len(batch.Actions) != 6 || batch.PartitionKey != "uk" {
		t.Errorf("batch = %+v", batch)
	}
	if len(batch.RowKeys) != 6 || batch.RowKeys[0] != "1" {
		t.Errorf("row keys = %v — they are what names the failing change later", batch.RowKeys)
	}
}

// The check Azurite cannot make for us: it accepts a cross-partition batch
// that real Azure rejects, so without this the emulator would false-pass the
// single most important batch constraint.
func TestParseBatchRejectsCrossPartition(t *testing.T) {
	_, err := ParseBatch(batchInput(`[
		{"action":"insert","row":{"PartitionKey":"uk","RowKey":"1"}},
		{"action":"insert","row":{"PartitionKey":"us","RowKey":"2"}}
	]`), "actions")
	if err == nil || !strings.Contains(err.Error(), "must share a PartitionKey") {
		t.Errorf("err = %v", err)
	}
	// The message must name both partitions or the operator cannot find the
	// offending change in a 90-row array.
	if err != nil && (!strings.Contains(err.Error(), `"us"`) || !strings.Contains(err.Error(), `"uk"`)) {
		t.Errorf("err = %v, want both partition keys named", err)
	}
}

func TestParseBatchRejectsDuplicateRowKey(t *testing.T) {
	_, err := ParseBatch(batchInput(`[
		{"action":"insert","row":{"PartitionKey":"uk","RowKey":"1"}},
		{"action":"delete","row":{"PartitionKey":"uk","RowKey":"1"}}
	]`), "actions")
	if err == nil || !strings.Contains(err.Error(), "only once") {
		t.Errorf("err = %v", err)
	}
}

func TestParseBatchRejectsAnEntityTheSDKWouldPanicOn(t *testing.T) {
	_, err := ParseBatch(batchInput(`[{"action":"insert","row":{"PartitionKey":42,"RowKey":"1"}}]`), "actions")
	if err == nil || !strings.Contains(err.Error(), "must be a string") {
		t.Errorf("err = %v", err)
	}
}

func TestParseBatchRejectsAChangeWithNoRow(t *testing.T) {
	_, err := ParseBatch(batchInput(`[{"action":"delete"}]`), "actions")
	if err == nil || !strings.Contains(err.Error(), `no "row" object`) {
		t.Errorf("err = %v", err)
	}
}

func TestParseBatchCarriesTheETag(t *testing.T) {
	batch, err := ParseBatch(batchInput(
		`[{"action":"merge","row":{"PartitionKey":"uk","RowKey":"1"},"etag":"W/\"x\""}]`), "actions")
	if err != nil {
		t.Fatalf("ParseBatch: %v", err)
	}
	if batch.Actions[0].IfMatch == nil || string(*batch.Actions[0].IfMatch) != `W/"x"` {
		t.Errorf("IfMatch = %v", batch.Actions[0].IfMatch)
	}
}

func TestParseBatchRejectsNonArray(t *testing.T) {
	_, err := ParseBatch(batchInput(`{"action":"insert"}`), "actions")
	if err == nil || !strings.Contains(err.Error(), "JSON array") {
		t.Errorf("err = %v", err)
	}
}

func TestBatchErrorfNamesTheFailingRow(t *testing.T) {
	batch := Batch{RowKeys: []string{"1001", "1002", "1003"}}
	got := Auth{}.BatchErrorf(responseErr(http.StatusBadRequest, "1:EntityAlreadyExists"), batch)
	if !strings.Contains(got, `RowKey "1002"`) {
		t.Errorf("got = %q, want the index resolved to a row key", got)
	}
	if !strings.Contains(got, "change 2 of 3") || !strings.Contains(got, "rolled back") {
		t.Errorf("got = %q", got)
	}
}

// A 202-with-inner-4xx loses the index before we see it. That must degrade to
// the plain message rather than guess at a row.
func TestBatchErrorfWithoutAnIndexIsPlain(t *testing.T) {
	batch := Batch{RowKeys: []string{"1001"}}
	got := Auth{}.BatchErrorf(responseErr(http.StatusAccepted, ""), batch)
	if strings.Contains(got, "rolled back") {
		t.Errorf("got = %q, want no change named when the service did not name one", got)
	}
}

// An index we cannot resolve to a row is worse than none: naming the wrong row
// would send an operator to edit a change that was fine.
func TestBatchErrorfIgnoresAnOutOfRangeIndex(t *testing.T) {
	batch := Batch{RowKeys: []string{"1001"}}
	got := Auth{}.BatchErrorf(responseErr(http.StatusBadRequest, "7:EntityAlreadyExists"), batch)
	if strings.Contains(got, "rolled back") || strings.Contains(got, "1001") {
		t.Errorf("got = %q, want no change named for an index we cannot resolve", got)
	}
}

// ---------------------------------------------------------------------------
// Access policies
// ---------------------------------------------------------------------------

func policyInput(raw string) []*core.Connection {
	return []*core.Connection{conn("policies", core.ConnectionTypeObject, raw)}
}

func TestParseAccessPolicies(t *testing.T) {
	policies, err := ParseAccessPolicies(policyInput(
		`[{"id":"readonly","permissions":"r","start":"2026-01-01T00:00:00Z","expiry":"2027-01-01T00:00:00Z"}]`), "policies")
	if err != nil {
		t.Fatalf("ParseAccessPolicies: %v", err)
	}
	if len(policies) != 1 || *policies[0].ID != "readonly" || *policies[0].AccessPolicy.Permission != "r" {
		t.Fatalf("policies = %+v", policies[0])
	}
	if policies[0].AccessPolicy.Expiry.Year() != 2027 {
		t.Errorf("expiry = %v", policies[0].AccessPolicy.Expiry)
	}
}

// An empty array is the ONLY way to clear the set, so it must parse rather
// than read as "nothing supplied".
func TestParseAccessPoliciesEmptyArrayIsValid(t *testing.T) {
	policies, err := ParseAccessPolicies(policyInput(`[]`), "policies")
	if err != nil {
		t.Fatalf("an empty array must be valid — it is the only way to revoke every policy: %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("policies = %v", policies)
	}
}

func TestParseAccessPoliciesRejectsBadTime(t *testing.T) {
	_, err := ParseAccessPolicies(policyInput(`[{"id":"x","permissions":"r","expiry":"next tuesday"}]`), "policies")
	if err == nil || !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("err = %v", err)
	}
}

func TestShapeAccessPolicyRoundTrips(t *testing.T) {
	parsed, err := ParseAccessPolicies(policyInput(
		`[{"id":"readonly","permissions":"r","expiry":"2027-01-01T00:00:00Z"}]`), "policies")
	if err != nil {
		t.Fatalf("ParseAccessPolicies: %v", err)
	}
	shaped := ShapeAccessPolicy(parsed[0])
	// Get Access Policies must emit what Set Access Policies accepts, or a
	// read-modify-write flow silently mangles the set.
	if shaped["id"] != "readonly" || shaped["permissions"] != "r" || shaped["expiry"] != "2027-01-01T00:00:00Z" {
		t.Errorf("shaped = %v", shaped)
	}
}

func TestShapeAccessPolicyToleratesNils(t *testing.T) {
	if got := ShapeAccessPolicy(nil); len(got) != 0 {
		t.Errorf("got = %v", got)
	}
}

// ---------------------------------------------------------------------------
// SAS
// ---------------------------------------------------------------------------

// aztables v1.4.1 signs the partition/row range into the string-to-sign but
// never encodes it into the token, so the service recomputes a different
// signature and rejects the link with a bare 403. SignTableSAS appends them,
// which is what makes the token agree with its own signature.
func TestSignTableSASAppendsTheRangeTheSDKDrops(t *testing.T) {
	auth := Auth{Method: AuthSharedKey, AccountName: "devstoreaccount1", AccountKey: devKey}
	cred, err := auth.SharedKeyCredential()
	if err != nil {
		t.Fatalf("SharedKeyCredential: %v", err)
	}
	values := sasValues("Orders", "r")
	values.StartPartitionKey, values.EndPartitionKey = "uk", "uk"
	values.StartRowKey, values.EndRowKey = "0001", "9999"

	bare, err := values.Sign(cred)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if strings.Contains(bare, "spk=") {
		t.Skip("aztables now encodes the range itself — SignTableSAS's workaround can go")
	}

	token, err := SignTableSAS(values, cred)
	if err != nil {
		t.Fatalf("SignTableSAS: %v", err)
	}
	for _, want := range []string{"spk=uk", "epk=uk", "srk=0001", "erk=9999"} {
		if !strings.Contains(token, want) {
			t.Errorf("%s missing from the token: %s", want, token)
		}
	}
}

func TestSignTableSASAppendsNothingWithoutARange(t *testing.T) {
	auth := Auth{Method: AuthSharedKey, AccountName: "devstoreaccount1", AccountKey: devKey}
	cred, _ := auth.SharedKeyCredential()
	token, err := SignTableSAS(sasValues("Orders", "r"), cred)
	if err != nil {
		t.Fatalf("SignTableSAS: %v", err)
	}
	for _, unwanted := range []string{"spk=", "srk=", "epk=", "erk="} {
		if strings.Contains(token, unwanted) {
			t.Errorf("%s appeared with no range set: %s", unwanted, token)
		}
	}
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

func TestListResultEmitsAnArrayNotNull(t *testing.T) {
	// A nil slice serialises as null and breaks a downstream Loop.
	out := ListResult(nil, "none")
	results, ok := out["results"].([]interface{})
	if !ok || results == nil {
		t.Errorf("results = %#v, want an empty array", out["results"])
	}
	if out["count"] != 0 || out["success"] != true {
		t.Errorf("out = %v", out)
	}
}

func TestErrorResultCarriesTheMessageEverywhere(t *testing.T) {
	out := ErrorResult("it broke")
	if out["success"] != false || out["error"] != "it broke" || out["tool_result"] != "it broke" {
		t.Errorf("out = %v", out)
	}
}

func TestResourceResultToleratesANilBody(t *testing.T) {
	out := ResourceResult("uk/1001", nil, "done")
	if out["id"] != "uk/1001" || out["success"] != true {
		t.Errorf("out = %v", out)
	}
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Errorf("result = %#v, want an empty object", out["result"])
	}
}
