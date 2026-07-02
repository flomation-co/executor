package ecommerce_shopify_order_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
	. "github.com/onsi/gomega"
)

func base(server string) []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "shpat_x"},
		{Name: "shop", Type: core.ConnectionTypeString, Value: "acme"},
	}
}

func TestExecuteMissingAuth(t *testing.T) {
	RegisterTestingT(t)
	// Neither an access token nor client credentials → hard auth error.
	res, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "shop", Type: core.ConnectionTypeString, Value: "acme"},
		{Name: "line_items", Type: core.ConnectionTypeObject, Value: `[{"variant_id":1,"quantity":1}]`},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("authentication required"))
	Expect(res).To(BeNil())
}

func TestExecuteMissingLineItems(t *testing.T) {
	RegisterTestingT(t)
	res, err := Execute(&core.Flow{}, nil, base(""))
	Expect(err).To(BeNil()) // soft error
	Expect(res["success"]).To(Equal(false))
	Expect(res["error"]).To(ContainSubstring("line_items is required"))
}

func TestExecuteBadLineItemsJSON(t *testing.T) {
	RegisterTestingT(t)
	inputs := append(base(""), &core.Connection{Name: "line_items", Type: core.ConnectionTypeObject, Value: "{not json"})
	res, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(res["success"]).To(Equal(false))
	Expect(res["error"]).To(ContainSubstring("valid JSON"))
}

func TestExecuteCreatesOrder(t *testing.T) {
	RegisterTestingT(t)
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/admin/api/" + shopify.APIVersion + "/orders.json"))
		Expect(r.Header.Get("X-Shopify-Access-Token")).To(Equal("shpat_x"))
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"order":{"id":450789469,"email":"jane@example.com"}}`))
	}))
	defer server.Close()
	defer shopify.SetHostForTest(server.URL)()

	inputs := append(base(""),
		&core.Connection{Name: "line_items", Type: core.ConnectionTypeObject, Value: `[{"variant_id":447654529,"quantity":2}]`},
		&core.Connection{Name: "email", Type: core.ConnectionTypeString, Value: "jane@example.com"},
		&core.Connection{Name: "tags", Type: core.ConnectionTypeString, Value: "vip"},
		&core.Connection{Name: "test", Type: core.ConnectionTypeBoolean, Value: true},
		&core.Connection{Name: "shipping_address", Type: core.ConnectionTypeObject, Value: `{"city":"London"}`},
		&core.Connection{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `{"currency":"GBP"}`},
	)
	res, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(res["success"]).To(Equal(true))
	Expect(res["id"]).To(Equal("450789469"))
	Expect(res["tool_result"]).To(Equal("Created order 450789469"))

	// The request body must be wrapped in {order:{...}} with the fields set.
	order := gotBody["order"].(map[string]interface{})
	Expect(order["line_items"]).To(Equal([]interface{}{map[string]interface{}{"variant_id": float64(447654529), "quantity": float64(2)}}))
	Expect(order["email"]).To(Equal("jane@example.com"))
	Expect(order["tags"]).To(Equal("vip"))
	Expect(order["test"]).To(Equal(true))
	Expect(order["shipping_address"]).To(Equal(map[string]interface{}{"city": "London"}))
	Expect(order["currency"]).To(Equal("GBP")) // additional_fields merged
}

func TestExecuteSurfacesAPIError(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":{"order":["is invalid"]}}`))
	}))
	defer server.Close()
	defer shopify.SetHostForTest(server.URL)()

	inputs := append(base(""), &core.Connection{Name: "line_items", Type: core.ConnectionTypeObject, Value: `[{"variant_id":1,"quantity":1}]`})
	res, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(res["success"]).To(Equal(false))
	Expect(res["error"]).To(ContainSubstring("422"))
	Expect(res["error"]).To(ContainSubstring("is invalid"))
}
