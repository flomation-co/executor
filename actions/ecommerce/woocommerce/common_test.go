package woocommerce

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestNormaliseBaseURL(t *testing.T) {
	RegisterTestingT(t)
	for input, want := range map[string]string{
		"store.com":                       "https://store.com",
		"  store.com/  ":                  "https://store.com",
		"https://store.com":               "https://store.com",
		"https://store.com/":              "https://store.com",
		"http://store.com":                "http://store.com",
		"https://store.com/shop":          "https://store.com/shop",
		"https://store.com/wp-json/wc/v3": "https://store.com",
		"https://store.com/wp-json":       "https://store.com",
	} {
		got, err := NormaliseBaseURL(input)
		Expect(err).To(BeNil(), "input: %q", input)
		Expect(got).To(Equal(want), "input: %q", input)
	}

	// Empty and non-http schemes are rejected.
	for _, bad := range []string{"", "  ", "ftp://store.com", "https://"} {
		_, err := NormaliseBaseURL(bad)
		Expect(err).To(HaveOccurred(), "input: %q", bad)
	}
}

func TestGetAuth(t *testing.T) {
	RegisterTestingT(t)
	a, err := GetAuth([]*core.Connection{
		{Name: "url", Type: core.ConnectionTypeString, Value: "store.com"},
		{Name: "consumer_key", Type: core.ConnectionTypeSecret, Value: "ck_x"},
		{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Value: "cs_y"},
	})
	Expect(err).To(BeNil())
	Expect(a.BaseURL).To(Equal("https://store.com"))
	Expect(a.ConsumerKey).To(Equal("ck_x"))
	Expect(a.ConsumerSecret).To(Equal("cs_y"))
	Expect(a.InQuery).To(BeFalse())

	// Missing key parts error.
	for _, missing := range []string{"consumer_key", "consumer_secret", "url"} {
		in := []*core.Connection{
			{Name: "url", Type: core.ConnectionTypeString, Value: "store.com"},
			{Name: "consumer_key", Type: core.ConnectionTypeSecret, Value: "ck_x"},
			{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Value: "cs_y"},
		}
		filtered := in[:0]
		for _, c := range in {
			if c.Name != missing {
				filtered = append(filtered, c)
			}
		}
		_, err := GetAuth(filtered)
		Expect(err).To(HaveOccurred(), "missing: %q", missing)
	}
}

func TestExecuteAPISendsBasicAuth(t *testing.T) {
	RegisterTestingT(t)
	var gotUser, gotPass, gotPath string
	var hadBasic bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, hadBasic = r.BasicAuth()
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	a := Auth{BaseURL: "https://ignored", ConsumerKey: "ck_x", ConsumerSecret: "cs_y"}
	resp, err := ExecuteAPI(a, http.MethodGet, "/orders/1", nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
	Expect(hadBasic).To(BeTrue())
	Expect(gotUser).To(Equal("ck_x"))
	Expect(gotPass).To(Equal("cs_y"))
	Expect(gotPath).To(Equal("/orders/1"))
}

func TestExecuteAPICredentialsInQuery(t *testing.T) {
	RegisterTestingT(t)
	var gotKey, gotSecret string
	var hadBasic bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, hadBasic = r.BasicAuth()
		gotKey = r.URL.Query().Get("consumer_key")
		gotSecret = r.URL.Query().Get("consumer_secret")
		// Existing query params must survive alongside the creds.
		Expect(r.URL.Query().Get("per_page")).To(Equal("5"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	a := Auth{ConsumerKey: "ck_x", ConsumerSecret: "cs_y", InQuery: true}
	_, err := ExecuteAPI(a, http.MethodGet, "/orders?per_page=5", nil)
	Expect(err).To(BeNil())
	Expect(hadBasic).To(BeFalse())
	Expect(gotKey).To(Equal("ck_x"))
	Expect(gotSecret).To(Equal("cs_y"))
}

func TestCheckResponse(t *testing.T) {
	RegisterTestingT(t)
	Expect(CheckResponse(&APIResponse{StatusCode: 200})).To(BeNil())
	Expect(CheckResponse(&APIResponse{StatusCode: 201})).To(BeNil())

	err := CheckResponse(&APIResponse{StatusCode: 400, Body: []byte(`{"code":"woocommerce_rest_invalid_id","message":"Invalid ID.","data":{"status":400}}`)})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("WooCommerce API error (400)"))
	Expect(err.Error()).To(ContainSubstring("Invalid ID."))
	Expect(err.Error()).To(ContainSubstring("woocommerce_rest_invalid_id"))

	// Non-JSON body still surfaces something useful.
	err = CheckResponse(&APIResponse{StatusCode: 500, Body: []byte(`<html>boom</html>`)})
	Expect(err.Error()).To(ContainSubstring("500"))
}

func TestDecodeAndDecodeList(t *testing.T) {
	RegisterTestingT(t)
	obj, err := decode(&APIResponse{Body: []byte(`{"id":7,"status":"processing"}`)})
	Expect(err).To(BeNil())
	Expect(obj["status"]).To(Equal("processing"))

	// Empty body → empty containers, never nil-typed.
	obj, err = decode(&APIResponse{Body: nil})
	Expect(err).To(BeNil())
	Expect(obj).To(Equal(map[string]interface{}{}))

	arr, err := decodeList(&APIResponse{Body: []byte(`[{"id":1},{"id":2}]`)})
	Expect(err).To(BeNil())
	Expect(len(arr)).To(Equal(2))

	arr, err = decodeList(&APIResponse{Body: nil})
	Expect(err).To(BeNil())
	Expect(arr).To(Equal([]interface{}{}))
}

func TestListResourcesReturnAllPaginates(t *testing.T) {
	RegisterTestingT(t)
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		w.Header().Set("Content-Type", "application/json")
		switch call {
		case 1:
			Expect(r.URL.Query().Get("page")).To(Equal("1"))
			Expect(r.URL.Query().Get("status")).To(Equal("processing"))
			w.Header().Set("X-WP-Total", "2")
			w.Header().Set("X-WP-TotalPages", "2")
			w.Header().Set("Link", `<https://x/wp-json/wc/v3/orders?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`[{"id":1}]`))
		case 2:
			Expect(r.URL.Query().Get("page")).To(Equal("2"))
			// No Link header → last page.
			_, _ = w.Write([]byte(`[{"id":2}]`))
		default:
			t.Fatalf("unexpected extra page fetch: call %d", call)
		}
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	q := url.Values{"status": {"processing"}, "per_page": {"1"}}
	items, total, totalPages, pages, err := ListResources(Auth{}, "/orders", q, true)
	Expect(err).To(BeNil())
	Expect(pages).To(Equal(2))
	Expect(total).To(Equal(2))
	Expect(totalPages).To(Equal(2))
	Expect(len(items)).To(Equal(2))
}

func TestListResourcesSinglePageIgnoresNext(t *testing.T) {
	RegisterTestingT(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-Total", "50")
		w.Header().Set("X-WP-TotalPages", "25")
		w.Header().Set("Link", `<https://x/wp-json/wc/v3/products?page=2>; rel="next"`)
		_, _ = w.Write([]byte(`[{"id":1},{"id":2}]`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	items, total, totalPages, pages, err := ListResources(Auth{}, "/products", nil, false)
	Expect(err).To(BeNil())
	Expect(calls).To(Equal(1)) // single page even though a next link exists
	Expect(pages).To(Equal(1))
	Expect(total).To(Equal(50))
	Expect(totalPages).To(Equal(25))
	Expect(len(items)).To(Equal(2))
}

func TestDeleteResourceForceParam(t *testing.T) {
	RegisterTestingT(t)
	var gotForce, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotForce = r.URL.Query().Get("force")
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9}`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	out, err := DeleteResource(Auth{}, "/orders/9", true)
	Expect(err).To(BeNil())
	Expect(gotMethod).To(Equal(http.MethodDelete))
	Expect(gotForce).To(Equal("true"))
	Expect(out["id"]).To(Equal(float64(9)))

	// force=false omits the param (trash).
	_, err = DeleteResource(Auth{}, "/orders/9", false)
	Expect(err).To(BeNil())
	Expect(gotForce).To(Equal(""))
}

func TestSetIntIfPresent(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	inputs := []*core.Connection{
		{Name: "cid", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "num", Type: core.ConnectionTypeInteger, Value: float64(7)},
		{Name: "blank", Type: core.ConnectionTypeString, Value: ""},
		{Name: "bad", Type: core.ConnectionTypeString, Value: "x"},
	}
	Expect(SetIntIfPresent(body, inputs, "customer_id", "cid")).To(BeNil())
	Expect(body["customer_id"]).To(Equal(42))
	Expect(SetIntIfPresent(body, inputs, "n", "num")).To(BeNil())
	Expect(body["n"]).To(Equal(7))
	// Blank omits.
	Expect(SetIntIfPresent(body, inputs, "skip", "blank")).To(BeNil())
	_, ok := body["skip"]
	Expect(ok).To(BeFalse())
	// Non-numeric errors.
	Expect(SetIntIfPresent(body, inputs, "b", "bad")).To(HaveOccurred())
}

func TestSetIDRefsIfPresent(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	SetIDRefsIfPresent(body, []*core.Connection{
		{Name: "cats", Type: core.ConnectionTypeString, Value: "5, 7 ,x,"},
	}, "categories", "cats")
	Expect(body["categories"]).To(Equal([]interface{}{
		map[string]interface{}{"id": 5},
		map[string]interface{}{"id": 7},
	}))

	// JSON-array form also accepted.
	body = map[string]interface{}{}
	SetIDRefsIfPresent(body, []*core.Connection{
		{Name: "tags", Type: core.ConnectionTypeString, Value: "[3,4]"},
	}, "tags", "tags")
	Expect(body["tags"]).To(Equal([]interface{}{
		map[string]interface{}{"id": 3},
		map[string]interface{}{"id": 4},
	}))

	// Nothing usable → field omitted.
	body = map[string]interface{}{}
	SetIDRefsIfPresent(body, []*core.Connection{{Name: "t", Type: core.ConnectionTypeString, Value: " , "}}, "tags", "t")
	_, ok := body["tags"]
	Expect(ok).To(BeFalse())
}

func TestSetIntListIfPresent(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	SetIntListIfPresent(body, []*core.Connection{
		{Name: "pids", Type: core.ConnectionTypeString, Value: "5, 7 ,x,"},
	}, "product_ids", "pids")
	Expect(body["product_ids"]).To(Equal([]interface{}{5, 7}))

	body = map[string]interface{}{}
	SetIntListIfPresent(body, []*core.Connection{
		{Name: "pids", Type: core.ConnectionTypeString, Value: "[3,4]"},
	}, "product_ids", "pids")
	Expect(body["product_ids"]).To(Equal([]interface{}{3, 4}))

	body = map[string]interface{}{}
	SetIntListIfPresent(body, []*core.Connection{{Name: "p", Type: core.ConnectionTypeString, Value: " , "}}, "product_ids", "p")
	_, ok := body["product_ids"]
	Expect(ok).To(BeFalse())
}

func TestClampLimit(t *testing.T) {
	RegisterTestingT(t)
	Expect(ClampLimit(0, false)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(0, true)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(500, true)).To(Equal(MaxPageLimit))
	Expect(ClampLimit(30, true)).To(Equal(30))
}

func TestStringifyID(t *testing.T) {
	RegisterTestingT(t)
	Expect(stringifyID(float64(123))).To(Equal("123"))
	Expect(stringifyID("abc")).To(Equal("abc"))
	Expect(stringifyID(nil)).To(Equal(""))
}

func TestResourceResultAndListResult(t *testing.T) {
	RegisterTestingT(t)
	out := ResourceResult(map[string]interface{}{"id": float64(55), "status": "processing"}, "done")
	Expect(out["id"]).To(Equal("55"))
	Expect(out["success"]).To(Equal(true))
	Expect(out["error"]).To(Equal(""))
	Expect(out["tool_result"]).To(Equal("done"))

	list := ListResult(nil, 0, 0, "none")
	Expect(list["results"]).To(Equal([]interface{}{})) // never null
	Expect(list["count"]).To(Equal(0))
}

func TestMergeAdditionalFieldsRejectsNonObject(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	err := MergeAdditionalFields(body, []*core.Connection{
		{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `[{"a":1}]`},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("must be a JSON object"))

	err = MergeAdditionalFields(body, []*core.Connection{
		{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `{"created_via":"api"}`},
	})
	Expect(err).To(BeNil())
	Expect(body["created_via"]).To(Equal("api"))
}

// TestExecuteAPICreateRoundTrip exercises CreateResource end-to-end: the body is
// sent verbatim (no envelope) and the bare-object response decodes straight
// through.
func TestExecuteAPICreateRoundTrip(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":101,"status":"pending"}`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	obj, err := CreateResource(Auth{}, "/orders", map[string]interface{}{"status": "pending"})
	Expect(err).To(BeNil())
	Expect(obj["id"]).To(Equal(float64(101)))
	Expect(strconv.FormatFloat(obj["id"].(float64), 'f', -1, 64)).To(Equal("101"))
}
