package shopify

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestNormaliseShop(t *testing.T) {
	RegisterTestingT(t)
	for input, want := range map[string]string{
		"my-store":                            "my-store",
		"  my-store  ":                        "my-store",
		"my-store.myshopify.com":              "my-store",
		"https://my-store.myshopify.com":      "my-store",
		"https://my-store.myshopify.com/":     "my-store",
		"http://my-store.myshopify.com/admin": "my-store",
	} {
		Expect(NormaliseShop(input)).To(Equal(want), "input: %q", input)
	}
}

func TestBuildURL(t *testing.T) {
	RegisterTestingT(t)
	Expect(BuildURL("acme", "/orders.json")).To(Equal("https://acme.myshopify.com/admin/api/" + APIVersion + "/orders.json"))
}

func TestParseNextPageInfo(t *testing.T) {
	RegisterTestingT(t)
	base := "https://x.myshopify.com/admin/api/2025-01/orders.json"
	Expect(parseNextPageInfo("")).To(Equal(""))
	Expect(parseNextPageInfo(`<` + base + `?limit=2&page_info=PREV>; rel="previous"`)).To(Equal(""))
	Expect(parseNextPageInfo(`<` + base + `?limit=2&page_info=NEXT123>; rel="next"`)).To(Equal("NEXT123"))
	// Both previous and next present — must pick next.
	link := `<` + base + `?page_info=PREV>; rel="previous", <` + base + `?page_info=NEXT456>; rel="next"`
	Expect(parseNextPageInfo(link)).To(Equal("NEXT456"))
}

func TestClampLimit(t *testing.T) {
	RegisterTestingT(t)
	Expect(ClampLimit(0, false)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(0, true)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(500, true)).To(Equal(MaxPageLimit))
	Expect(ClampLimit(100, true)).To(Equal(100))
}

func TestStringifyID(t *testing.T) {
	RegisterTestingT(t)
	Expect(stringifyID(float64(450789469))).To(Equal("450789469"))
	Expect(stringifyID("abc")).To(Equal("abc"))
	Expect(stringifyID(nil)).To(Equal(""))
}

func TestCheckResponse(t *testing.T) {
	RegisterTestingT(t)
	Expect(CheckResponse(&APIResponse{StatusCode: 200})).To(BeNil())

	// String error envelope.
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: []byte(`{"errors":"Not Found"}`)})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Shopify API error (404)"))
	Expect(err.Error()).To(ContainSubstring("Not Found"))

	// Object error envelope (field -> messages).
	err = CheckResponse(&APIResponse{StatusCode: 422, Body: []byte(`{"errors":{"title":["can't be blank"]}}`)})
	Expect(err.Error()).To(ContainSubstring("422"))
	Expect(err.Error()).To(ContainSubstring("can't be blank"))

	// 429 with Retry-After.
	h := http.Header{}
	h.Set("Retry-After", "2.0")
	err = CheckResponse(&APIResponse{StatusCode: 429, Headers: h})
	Expect(err.Error()).To(ContainSubstring("rate limit"))
	Expect(err.Error()).To(ContainSubstring("2.0"))
}

func TestOptionalJSON(t *testing.T) {
	RegisterTestingT(t)
	inputs := []*core.Connection{
		{Name: "arr", Type: core.ConnectionTypeObject, Value: `[{"quantity":1}]`},
		{Name: "obj", Type: core.ConnectionTypeObject, Value: map[string]interface{}{"a": "b"}},
		{Name: "blank", Type: core.ConnectionTypeObject, Value: "  "},
		{Name: "bad", Type: core.ConnectionTypeObject, Value: "{not json"},
	}
	v, err := OptionalJSON("arr", inputs)
	Expect(err).To(BeNil())
	Expect(v).To(Equal([]interface{}{map[string]interface{}{"quantity": float64(1)}}))

	v, err = OptionalJSON("obj", inputs)
	Expect(err).To(BeNil())
	Expect(v).To(Equal(map[string]interface{}{"a": "b"}))

	v, err = OptionalJSON("blank", inputs)
	Expect(err).To(BeNil())
	Expect(v).To(BeNil())

	_, err = OptionalJSON("bad", inputs)
	Expect(err).To(HaveOccurred())

	v, err = OptionalJSON("absent", inputs)
	Expect(err).To(BeNil())
	Expect(v).To(BeNil())
}

func TestGetAuth(t *testing.T) {
	RegisterTestingT(t)
	shop, token, err := GetAuth([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "shpat_x"},
		{Name: "shop", Type: core.ConnectionTypeString, Value: "my-store.myshopify.com"},
	})
	Expect(err).To(BeNil())
	Expect(shop).To(Equal("my-store"))
	Expect(token).To(Equal("shpat_x"))

	// No token and no client creds -> auth error.
	_, _, err = GetAuth([]*core.Connection{{Name: "shop", Type: core.ConnectionTypeString, Value: "x"}})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("authentication required"))
}

func TestResourceResult(t *testing.T) {
	RegisterTestingT(t)
	resp := map[string]interface{}{"order": map[string]interface{}{"id": float64(123), "email": "a@b.com"}}
	out := ResourceResult(resp, "order", "Created order")
	Expect(out["id"]).To(Equal("123"))
	Expect(out["success"]).To(Equal(true))
	Expect(out["error"]).To(Equal(""))
	Expect(out["result"]).To(Equal(map[string]interface{}{"id": float64(123), "email": "a@b.com"}))
}

// withHost points hostForShop at a test server for the duration of a test.
func withHost(url string) func() {
	prev := hostForShop
	hostForShop = func(string) string { return url }
	return func() { hostForShop = prev }
}

func TestExecuteAPISendsTokenHeader(t *testing.T) {
	RegisterTestingT(t)
	var gotToken, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Shopify-Access-Token")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":{"id":1}}`))
	}))
	defer server.Close()
	defer withHost(server.URL)()

	resp, err := ExecuteAPI("acme", "shpat_secret", http.MethodGet, "/orders/1.json", nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
	Expect(gotToken).To(Equal("shpat_secret"))
	Expect(gotPath).To(Equal("/admin/api/" + APIVersion + "/orders/1.json"))
}

func TestListResourcesReturnAllPaginates(t *testing.T) {
	RegisterTestingT(t)
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		w.Header().Set("Content-Type", "application/json")
		switch call {
		case 1:
			// First page: one order + a next-page Link cursor. Only the
			// page_info query param is read from the Link URL, so the host
			// is irrelevant here.
			Expect(r.URL.Query().Get("status")).To(Equal("open"))
			w.Header().Set("Link", `<https://x/admin/api/2025-01/orders.json?limit=1&page_info=CUR2>; rel="next"`)
			_, _ = w.Write([]byte(`{"orders":[{"id":1}]}`))
		case 2:
			// Second page: with page_info, only limit/fields survive — no status.
			Expect(r.URL.Query().Get("page_info")).To(Equal("CUR2"))
			Expect(r.URL.Query().Get("status")).To(Equal(""))
			Expect(r.URL.Query().Get("limit")).To(Equal("1"))
			// No Link header → last page.
			_, _ = w.Write([]byte(`{"orders":[{"id":2}]}`))
		default:
			t.Fatalf("unexpected extra page fetch: call %d", call)
		}
	}))
	defer server.Close()
	defer withHost(server.URL)()

	q := url.Values{"status": {"open"}, "limit": {"1"}}
	items, next, _, pages, err := ListResources("acme", "t", "/orders.json", "orders", q, true)
	Expect(err).To(BeNil())
	Expect(pages).To(Equal(2))
	Expect(next).To(Equal(""))
	Expect(len(items)).To(Equal(2))
}

func TestListResourcesSinglePageReturnsCursor(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `<https://x/admin/api/2025-01/products.json?page_info=MORE>; rel="next"`)
		_, _ = w.Write([]byte(`{"products":[{"id":1},{"id":2}]}`))
	}))
	defer server.Close()
	defer withHost(server.URL)()

	items, next, _, pages, err := ListResources("acme", "t", "/products.json", "products", nil, false)
	Expect(err).To(BeNil())
	Expect(pages).To(Equal(1))     // single page even though a next cursor exists
	Expect(next).To(Equal("MORE")) // cursor surfaced for manual resume
	Expect(len(items)).To(Equal(2))
}

func TestNormaliseShopHardening(t *testing.T) {
	RegisterTestingT(t)
	// New-style admin URL extracts the handle.
	Expect(NormaliseShop("https://admin.shopify.com/store/my-store")).To(Equal("my-store"))
	Expect(NormaliseShop("admin.shopify.com/store/my-store/products")).To(Equal("my-store"))
	// Host-injection attempts reduce to a bare label (validated by GetAuth).
	Expect(NormaliseShop("evil.com")).To(Equal("evil"))
	Expect(NormaliseShop("shop@evil.com")).To(Equal("shop"))
	Expect(NormaliseShop("my-store:8080")).To(Equal("my-store"))
}

func TestGetAuthRejectsInvalidShop(t *testing.T) {
	RegisterTestingT(t)
	// These survive NormaliseShop with characters no Shopify handle allows
	// (note "shop/../x" is NOT here — it correctly normalises to "shop").
	for _, bad := range []string{"my store", "shop_underscore", "shop!bad", ""} {
		_, _, err := GetAuth([]*core.Connection{
			{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "shpat_x"},
			{Name: "shop", Type: core.ConnectionTypeString, Value: bad},
		})
		Expect(err).To(HaveOccurred(), "shop: %q", bad)
	}
	// Valid handles pass.
	for _, ok := range []string{"my-store", "MyStore123"} {
		shop, _, err := GetAuth([]*core.Connection{
			{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "shpat_x"},
			{Name: "shop", Type: core.ConnectionTypeString, Value: ok},
		})
		Expect(err).To(BeNil(), "shop: %q", ok)
		Expect(shop).To(Equal(ok))
	}
}

func TestListResultCoercesNil(t *testing.T) {
	RegisterTestingT(t)
	out := ListResult(nil, "", nil, "none")
	Expect(out["results"]).To(Equal([]interface{}{})) // never null
	Expect(out["count"]).To(Equal(0))
}

func TestMergeAdditionalFieldsRejectsNonObject(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	// Array is valid JSON but wrong shape → error, not silent drop.
	err := MergeAdditionalFields(body, []*core.Connection{
		{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `[{"a":1}]`},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("must be a JSON object"))
	// Object merges fine.
	err = MergeAdditionalFields(body, []*core.Connection{
		{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `{"currency":"GBP"}`},
	})
	Expect(err).To(BeNil())
	Expect(body["currency"]).To(Equal("GBP"))
}

func TestBuildDefaultVariant(t *testing.T) {
	RegisterTestingT(t)
	Expect(BuildDefaultVariant([]*core.Connection{})).To(BeNil())
	dv := BuildDefaultVariant([]*core.Connection{
		{Name: "price", Type: core.ConnectionTypeString, Value: "19.99"},
		{Name: "sku", Type: core.ConnectionTypeString, Value: "SB-1"},
	})
	Expect(dv).To(Equal([]interface{}{map[string]interface{}{"price": "19.99", "sku": "SB-1"}}))
}

func TestGetAuthMintsFromClientCredentials(t *testing.T) {
	RegisterTestingT(t)
	tokenCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/admin/oauth/access_token"))
		Expect(r.Method).To(Equal(http.MethodPost))
		_ = r.ParseForm()
		Expect(r.Form.Get("grant_type")).To(Equal("client_credentials"))
		Expect(r.Form.Get("client_id")).To(Equal("cid"))
		Expect(r.Form.Get("client_secret")).To(Equal("csecret"))
		tokenCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"shpca_minted","scope":"read_products","expires_in":86399}`))
	}))
	defer server.Close()
	defer SetHostForTest(server.URL)()
	// Fresh cache for this test.
	tokenCache.mu.Lock()
	tokenCache.m = map[string]cachedToken{}
	tokenCache.mu.Unlock()

	in := []*core.Connection{
		{Name: "shop", Type: core.ConnectionTypeString, Value: "acme"},
		{Name: "client_id", Type: core.ConnectionTypeString, Value: "cid"},
		{Name: "client_secret", Type: core.ConnectionTypeSecret, Value: "csecret"},
	}
	shop, tok, err := GetAuth(in)
	Expect(err).To(BeNil())
	Expect(shop).To(Equal("acme"))
	Expect(tok).To(Equal("shpca_minted"))

	// Second call within TTL is served from cache — no extra mint.
	_, tok2, err := GetAuth(in)
	Expect(err).To(BeNil())
	Expect(tok2).To(Equal("shpca_minted"))
	Expect(tokenCalls).To(Equal(1), "token must be minted once and cached")
}

func TestGetAuthPrefersExplicitToken(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("must not mint a token when access_token is supplied")
	}))
	defer server.Close()
	defer SetHostForTest(server.URL)()

	_, tok, err := GetAuth([]*core.Connection{
		{Name: "shop", Type: core.ConnectionTypeString, Value: "acme"},
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "shpat_explicit"},
		{Name: "client_id", Type: core.ConnectionTypeString, Value: "cid"},
		{Name: "client_secret", Type: core.ConnectionTypeSecret, Value: "csecret"},
	})
	Expect(err).To(BeNil())
	Expect(tok).To(Equal("shpat_explicit"))
}

func TestMintAccessTokenSurfacesError(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<html><head><title>400 - Oauth error app_not_installed</title></head></html>`))
	}))
	defer server.Close()
	defer SetHostForTest(server.URL)()

	_, _, err := MintAccessToken("acme", "cid", "csecret")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("app_not_installed"))
	Expect(err.Error()).To(ContainSubstring("installed on the store"))
}
