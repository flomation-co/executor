package cosmosdb

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// emulatorKey is the Cosmos DB emulator's well-known, public master key —
// ideal test material because it is real key-shaped base64 with no secrecy.
const emulatorKey = "C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw=="

func strIn(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}
func secretIn(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeSecret, Value: v}
}
func boolIn(name string, v bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: v}
}

// TestMasterKeyAuthHeaderVector pins the signing algorithm against fixed
// vectors generated with an independent implementation of the documented
// algorithm (HMAC-SHA256 over "{verb}\n{type}\n{rid}\n{date}\n\n", lowercased
// verb/type/date, base64 signature, whole header value URL-encoded). A drift
// in payload assembly, key decoding, or encoding breaks these.
func TestMasterKeyAuthHeaderVector(t *testing.T) {
	RegisterTestingT(t)
	const date = "Thu, 27 Apr 2017 00:51:12 GMT"

	for _, tc := range []struct {
		verb, rtype, rid, want string
	}{
		// Point read of a database — resourceId keeps its case.
		{"GET", "dbs", "dbs/ToDoList", "type%3Dmaster%26ver%3D1.0%26sig%3DmM0rMK%2F7o1MZhQwDitGRoGisPRPx09os%2F86Jk0YwquY%3D"},
		// Single offer — the CALLER passes the offer _rid lowercased.
		{"PUT", "offers", "4qez", "type%3Dmaster%26ver%3D1.0%26sig%3DaI9qsTuAjhlYUU%2FpLPwACTVzu3RHmn1B2lXWnc%2FzUMo%3D"},
		// Offers feed/query — EMPTY resourceId.
		{"POST", "offers", "", "type%3Dmaster%26ver%3D1.0%26sig%3DFIhlZydHuW2r3NHmGYsAVgJa0wCItsCqUO%2BzEbzxw2o%3D"},
	} {
		got, err := MasterKeyAuthHeader(tc.verb, tc.rtype, tc.rid, date, emulatorKey)
		Expect(err).To(BeNil(), "%s %s", tc.verb, tc.rid)
		Expect(got).To(Equal(tc.want), "%s %s %q", tc.verb, tc.rtype, tc.rid)
	}
}

func TestMasterKeyAuthHeaderRejectsBadBase64(t *testing.T) {
	RegisterTestingT(t)
	_, err := MasterKeyAuthHeader("GET", "dbs", "", "Thu, 27 Apr 2017 00:51:12 GMT", "not!!base64")
	Expect(err).To(HaveOccurred())
	// The bad key material must not be echoed back.
	Expect(err.Error()).NotTo(ContainSubstring("not!!base64"))
}

func TestAADAuthHeaderIsURLEncoded(t *testing.T) {
	RegisterTestingT(t)
	got := AADAuthHeader("ey.J+ab/c=")
	Expect(got).To(Equal(url.QueryEscape("type=aad&ver=1.0&sig=ey.J+ab/c=")))
	Expect(got).NotTo(ContainSubstring("="), "every = must be percent-encoded")
}

func TestGetAuthDefaultsAndOverrides(t *testing.T) {
	RegisterTestingT(t)

	// Master key by default (untouched dropdown), derived endpoint.
	a, err := GetAuth([]*core.Connection{
		strIn("account_name", "myaccount"),
		secretIn("master_key", emulatorKey),
	})
	Expect(err).To(BeNil())
	Expect(a.Method).To(Equal(AuthMethodMasterKey))
	Expect(a.BaseURL).To(Equal("https://myaccount.documents.azure.com"))
	Expect(a.Insecure).To(BeFalse())

	// Custom endpoint (emulator style) + insecure opt-in.
	a, err = GetAuth([]*core.Connection{
		strIn("account_name", "localhost"),
		secretIn("master_key", emulatorKey),
		strIn("endpoint", "https://localhost:8081/"),
		boolIn("allow_insecure", true),
	})
	Expect(err).To(BeNil())
	Expect(a.BaseURL).To(Equal("https://localhost:8081"))
	Expect(a.Insecure).To(BeTrue())

	// Entra requires the service-principal triple.
	_, err = GetAuth([]*core.Connection{
		strIn("account_name", "myaccount"),
		strIn("auth_method", "entra"),
		strIn("azure_tenant_id", "tenant"),
		strIn("azure_client_id", "client"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("azure_client_secret"))

	// Host-charset guard on the account name.
	_, err = GetAuth([]*core.Connection{
		strIn("account_name", "evil.example.com/x"),
		secretIn("master_key", emulatorKey),
	})
	Expect(err).To(HaveOccurred())

	// Missing master key.
	_, err = GetAuth([]*core.Connection{strIn("account_name", "myaccount")})
	Expect(err).To(HaveOccurred())

	// Non-http endpoint scheme.
	_, err = GetAuth([]*core.Connection{
		strIn("account_name", "myaccount"),
		secretIn("master_key", emulatorKey),
		strIn("endpoint", "ftp://localhost:8081"),
	})
	Expect(err).To(HaveOccurred())
}

func TestEntraScopeDerivesFromEndpointHost(t *testing.T) {
	RegisterTestingT(t)
	Expect(EntraScope(Auth{AccountName: "acct", BaseURL: "https://acct.documents.azure.com"})).
		To(Equal("https://acct.documents.azure.com/.default"))
	// Custom endpoint: scope follows the host, port stripped.
	Expect(EntraScope(Auth{AccountName: "acct", BaseURL: "https://acct.documents.chinacloudapi.cn:443"})).
		To(Equal("https://acct.documents.chinacloudapi.cn/.default"))
}

func TestCheckResponseUnwrapsNestedErrors(t *testing.T) {
	RegisterTestingT(t)

	err := CheckResponse(&APIResponse{
		StatusCode: 409,
		Body:       []byte(`{"code":"Conflict","message":"Message: {\"Errors\":[\"Resource with specified id or name already exists.\"]}\r\nActivityId: aaa, Request URI: /apps/x"}`),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Conflict"))
	Expect(err.Error()).To(ContainSubstring("Resource with specified id or name already exists."))
	Expect(err.Error()).NotTo(ContainSubstring("ActivityId"), "diagnostics noise must be dropped")

	// Plain message without the nested payload survives untouched.
	err = CheckResponse(&APIResponse{StatusCode: 401, Body: []byte(`{"code":"Unauthorized","message":"token expired"}`)})
	Expect(err.Error()).To(ContainSubstring("token expired"))

	// Non-JSON body is truncated raw.
	err = CheckResponse(&APIResponse{StatusCode: 502, Body: []byte("bad gateway")})
	Expect(err.Error()).To(ContainSubstring("bad gateway"))

	// 2xx is fine.
	Expect(CheckResponse(&APIResponse{StatusCode: 201, Body: []byte("{}")})).To(BeNil())
}

func TestPartitionKeyHeaderShapes(t *testing.T) {
	RegisterTestingT(t)
	Expect(PartitionKeyHeader("alpha")).To(Equal(`["alpha"]`))
	Expect(PartitionKeyHeader(float64(42))).To(Equal(`[42]`))
	Expect(PartitionKeyHeader(true)).To(Equal(`[true]`))
}

func TestSimplifyStripsSystemProps(t *testing.T) {
	RegisterTestingT(t)
	got := Simplify(map[string]interface{}{
		"id": "a", "_rid": "x", "_etag": "y", "_ts": 1.0, "name": "n",
	})
	Expect(got).To(Equal(map[string]interface{}{"id": "a", "name": "n"}))

	items := SimplifyItems([]interface{}{
		map[string]interface{}{"id": "a", "_self": "s"},
		"not-an-object",
	}, true)
	Expect(items[0]).To(Equal(map[string]interface{}{"id": "a"}))
	Expect(items[1]).To(Equal("not-an-object"))

	// Off: untouched.
	raw := []interface{}{map[string]interface{}{"_rid": "keep"}}
	Expect(SimplifyItems(raw, false)).To(Equal(raw))
}

func TestBoolDefaultTrue(t *testing.T) {
	RegisterTestingT(t)
	Expect(BoolDefaultTrue("simplify", nil)).To(BeTrue())
	Expect(BoolDefaultTrue("simplify", []*core.Connection{boolIn("simplify", false)})).To(BeFalse())
	Expect(BoolDefaultTrue("simplify", []*core.Connection{boolIn("simplify", true)})).To(BeTrue())
}

func TestClampLimit(t *testing.T) {
	RegisterTestingT(t)
	Expect(ClampLimit(0, false)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(-3, true)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(10, true)).To(Equal(10))
	Expect(ClampLimit(99999, true)).To(Equal(MaxPageLimit))
}

func TestThroughputHeaders(t *testing.T) {
	RegisterTestingT(t)
	intIn := func(name string, v int) *core.Connection {
		return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: v}
	}

	h, err := ThroughputHeaders([]*core.Connection{intIn("throughput", 400)})
	Expect(err).To(BeNil())
	Expect(h).To(Equal(map[string]string{"x-ms-offer-throughput": "400"}))

	// Autoscale is the JSON object form, not n8n's bare number.
	h, err = ThroughputHeaders([]*core.Connection{intIn("autoscale_max", 4000)})
	Expect(err).To(BeNil())
	Expect(h).To(Equal(map[string]string{"x-ms-cosmos-offer-autopilot-setting": `{"maxThroughput":4000}`}))

	_, err = ThroughputHeaders([]*core.Connection{intIn("throughput", 400), intIn("autoscale_max", 4000)})
	Expect(err).To(HaveOccurred())
	_, err = ThroughputHeaders([]*core.Connection{intIn("throughput", 100)})
	Expect(err).To(HaveOccurred())
	_, err = ThroughputHeaders([]*core.Connection{intIn("autoscale_max", 500)})
	Expect(err).To(HaveOccurred())

	h, err = ThroughputHeaders(nil)
	Expect(err).To(BeNil())
	Expect(h).To(BeEmpty())
}

func TestQueryParameters(t *testing.T) {
	RegisterTestingT(t)
	params, err := QueryParameters([]*core.Connection{
		{Name: "parameters", Type: core.ConnectionTypeObject, Value: `{"@status":"open","limit":5}`},
	})
	Expect(err).To(BeNil())
	// Sorted by name; a missing @ is prefixed.
	Expect(params).To(Equal([]map[string]interface{}{
		{"name": "@status", "value": "open"},
		{"name": "@limit", "value": float64(5)},
	}))

	params, err = QueryParameters(nil)
	Expect(err).To(BeNil())
	Expect(params).To(BeNil())

	_, err = QueryParameters([]*core.Connection{
		{Name: "parameters", Type: core.ConnectionTypeObject, Value: `["not","an","object"]`},
	})
	Expect(err).To(HaveOccurred())
}

func TestSumCharges(t *testing.T) {
	RegisterTestingT(t)
	Expect(SumCharges("2.29", "3.5", "")).To(Equal("5.79"))
	Expect(SumCharges()).To(Equal("0.00"))
}

// TestDoRequestSignsAndVersions exercises the full request path against an
// httptest server: the x-ms-date/x-ms-version headers must be present and the
// authorization header must verify against the served date — recomputing it
// proves the signature covers exactly what was sent.
func TestDoRequestSignsAndVersions(t *testing.T) {
	RegisterTestingT(t)
	var gotAuth, gotDate, gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotDate = r.Header.Get("x-ms-date")
		gotVersion = r.Header.Get("x-ms-version")
		w.Header().Set("x-ms-request-charge", "2.29")
		_, _ = w.Write([]byte(`{"id":"db1"}`))
	}))
	defer server.Close()

	a := Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: server.URL}
	resp, err := DoRequest(nil, a, http.MethodGet, "/dbs/db1", "dbs", "dbs/db1", nil, nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
	Expect(gotVersion).To(Equal(APIVersion))
	Expect(gotDate).NotTo(BeEmpty())
	Expect(RequestCharge(resp)).To(Equal("2.29"))

	want, err := MasterKeyAuthHeader(http.MethodGet, "dbs", "dbs/db1", gotDate, emulatorKey)
	Expect(err).To(BeNil())
	Expect(gotAuth).To(Equal(want))
}

// TestFeedFollowsContinuation drives the pagination loop: page one answers
// with an x-ms-continuation token which must be echoed back verbatim on page
// two, charges must accumulate across pages, and the envelope property must be
// unwrapped.
func TestFeedFollowsContinuation(t *testing.T) {
	RegisterTestingT(t)
	var calls int32
	var secondContinuation, gotMaxItems string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		gotMaxItems = r.Header.Get("x-ms-max-item-count")
		if n == 1 {
			w.Header().Set("x-ms-continuation", "token-1")
			w.Header().Set("x-ms-request-charge", "2.5")
			_, _ = w.Write([]byte(`{"Databases":[{"id":"a"}],"_count":1}`))
			return
		}
		secondContinuation = r.Header.Get("x-ms-continuation")
		w.Header().Set("x-ms-request-charge", "3.5")
		_, _ = w.Write([]byte(`{"Databases":[{"id":"b"}],"_count":1}`))
	}))
	defer server.Close()

	a := Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: server.URL}
	items, charge, err := Feed(nil, a, http.MethodGet, "/dbs", "dbs", "", "Databases", nil, nil, 25, true)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(2))
	Expect(atomic.LoadInt32(&calls)).To(Equal(int32(2)))
	Expect(secondContinuation).To(Equal("token-1"))
	Expect(gotMaxItems).To(Equal("25"))
	Expect(charge).To(Equal("6.00"))
}

// TestFeedSinglePageWithoutReturnAll: a continuation token on the response
// must NOT be followed when return_all is off.
func TestFeedSinglePageWithoutReturnAll(t *testing.T) {
	RegisterTestingT(t)
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("x-ms-continuation", "more")
		_, _ = w.Write([]byte(`{"Documents":[{"id":"a"}]}`))
	}))
	defer server.Close()

	a := Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: server.URL}
	items, _, err := Feed(nil, a, http.MethodGet, "/dbs/d/colls/c/docs", "docs", "dbs/d/colls/c", "Documents", nil, nil, 50, false)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(1))
	Expect(atomic.LoadInt32(&calls)).To(Equal(int32(1)))
}

// TestContainerPartitionKeyPathCaches: the discovery GET must run once per
// container per execution; the second resolution answers from the cache.
func TestContainerPartitionKeyPathCaches(t *testing.T) {
	RegisterTestingT(t)
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"id":"c1","partitionKey":{"paths":["/category"],"kind":"Hash"}}`))
	}))
	defer server.Close()

	a := Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: server.URL}
	for i := 0; i < 2; i++ {
		path, err := ContainerPartitionKeyPath(nil, a, "d1", "c1")
		Expect(err).To(BeNil())
		Expect(path).To(Equal("/category"))
	}
	Expect(atomic.LoadInt32(&calls)).To(Equal(int32(1)))
}

func TestResolvePointPartitionKey(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"byid","partitionKey":{"paths":["/id"]}}`))
	}))
	defer server.Close()
	a := Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: server.URL}

	// Explicit input wins without touching the network (unreachable base URL).
	v, has, err := ResolvePointPartitionKey(nil, Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: "http://127.0.0.1:1"},
		[]*core.Connection{strIn("partition_key", "west")}, "d", "c", "item-1")
	Expect(err).To(BeNil())
	Expect(has).To(BeTrue())
	Expect(v).To(Equal("west"))

	// /id container derives from the item id.
	v, has, err = ResolvePointPartitionKey(nil, a, nil, "d", "byid", "item-1")
	Expect(err).To(BeNil())
	Expect(has).To(BeTrue())
	Expect(v).To(Equal("item-1"))
}

func TestResolveBodyPartitionKey(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"bycat","partitionKey":{"paths":["/category"]}}`))
	}))
	defer server.Close()
	a := Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: server.URL}

	// Value read from the body at the discovered path.
	body := map[string]interface{}{"id": "i1", "category": "books"}
	v, has, err := ResolveBodyPartitionKey(nil, a, nil, "d", "bycat", "i1", body)
	Expect(err).To(BeNil())
	Expect(has).To(BeTrue())
	Expect(v).To(Equal("books"))

	// Explicit input wins and is injected into a body that lacks the property.
	body = map[string]interface{}{"id": "i2"}
	v, has, err = ResolveBodyPartitionKey(nil, a, []*core.Connection{strIn("partition_key", "games")}, "d", "bycat", "i2", body)
	Expect(err).To(BeNil())
	Expect(has).To(BeTrue())
	Expect(v).To(Equal("games"))
	Expect(body["category"]).To(Equal("games"))

	// Neither input nor body property: a clear error naming the path.
	_, _, err = ResolveBodyPartitionKey(nil, a, nil, "d", "bycat", "i3", map[string]interface{}{"id": "i3"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("/category"))
}

// TestInsecureClientIsSeparate: allow_insecure must route through a DIFFERENT
// client whose transport skips verification, leaving the default untouched.
func TestInsecureClientIsSeparate(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer server.Close()

	secure := Auth{Method: AuthMethodMasterKey, MasterKey: emulatorKey, BaseURL: server.URL}
	_, err := DoRequest(nil, secure, http.MethodGet, "/dbs/x", "dbs", "dbs/x", nil, nil)
	Expect(err).To(HaveOccurred(), "the default client must refuse a self-signed certificate")

	insecure := secure
	insecure.Insecure = true
	resp, err := DoRequest(nil, insecure, http.MethodGet, "/dbs/x", "dbs", "dbs/x", nil, nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
}

func TestPathAndRIDBuilders(t *testing.T) {
	RegisterTestingT(t)
	// Paths escape segments; RIDs keep the raw ids (the service signs the raw
	// resource link).
	Expect(DocPath("my db", "c/1", "it em")).To(Equal("/dbs/my%20db/colls/c%2F1/docs/it%20em"))
	Expect(DocRID("my db", "c/1", "it em")).To(Equal("dbs/my db/colls/c/1/docs/it em"))
	Expect(CollPath("d", "c")).To(Equal("/dbs/d/colls/c"))
	Expect(CollRID("d", "c")).To(Equal("dbs/d/colls/c"))
	Expect(DBPath("d")).To(Equal("/dbs/d"))
	Expect(DBRID("d")).To(Equal("dbs/d"))
}

func TestOfferPathAndRIDLowercasesRID(t *testing.T) {
	RegisterTestingT(t)
	path, rid, err := OfferPathAndRID(map[string]interface{}{"id": "4qEz", "_rid": "4qEz"})
	Expect(err).To(BeNil())
	Expect(path).To(Equal("/offers/4qEz"))
	Expect(rid).To(Equal("4qez"), "the single-offer signing resourceId is the _rid LOWERCASED")

	_, _, err = OfferPathAndRID(map[string]interface{}{})
	Expect(err).To(HaveOccurred())
}
