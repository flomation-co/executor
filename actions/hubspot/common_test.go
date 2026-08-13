package hubspot_common

import (
	"net/http"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestGetAPIKey(t *testing.T) {
	RegisterTestingT(t)

	_, err := GetAPIKey(nil)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api_key is required"))

	key, err := GetAPIKey([]*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "pat-123"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(key).To(Equal("pat-123"))
}

func TestCSVToList(t *testing.T) {
	RegisterTestingT(t)

	Expect(CSVToList("")).To(BeNil())
	Expect(CSVToList("  ")).To(BeNil())
	Expect(CSVToList("email, firstname ,, lastname")).To(Equal([]string{"email", "firstname", "lastname"}))
}

func TestBuildProperties(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		strConn("email", "jane@example.com"),
		strConn("firstname", "Jane"),
		strConn("lastname", ""), // empty -> skipped
		{Name: "additional_properties", Type: core.ConnectionTypeKeyValueArray,
			Value: `[{"key":"lifecyclestage","value":"lead"},{"key":"firstname","value":"Janet"}]`},
	}

	props := BuildProperties(inputs, "email", "firstname", "lastname")
	// explicit non-empty fields are present
	Expect(props["email"]).To(Equal("jane@example.com"))
	// additional_properties override explicit fields
	Expect(props["firstname"]).To(Equal("Janet"))
	// additional-only key is added
	Expect(props["lifecyclestage"]).To(Equal("lead"))
	// empty explicit field is omitted
	Expect(props).NotTo(HaveKey("lastname"))
}

func TestBuildSearchBody_SimpleFilter(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		strConn("query", "acme"),
		strConn("filter_property", "email"),
		strConn("filter_value", "jane@example.com"),
		strConn("properties", "email,firstname"),
		{Name: "limit", Type: core.ConnectionTypeInteger, Value: int64(25)},
	}

	body := BuildSearchBody(inputs)
	Expect(body["query"]).To(Equal("acme"))
	Expect(body["properties"]).To(Equal([]string{"email", "firstname"}))
	Expect(body["limit"]).To(Equal(25))

	groups, ok := body["filterGroups"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(groups).To(HaveLen(1))
	group := groups[0].(map[string]interface{})
	filters := group["filters"].([]interface{})
	filter := filters[0].(map[string]interface{})
	Expect(filter["propertyName"]).To(Equal("email"))
	Expect(filter["operator"]).To(Equal("EQ")) // defaulted
	Expect(filter["value"]).To(Equal("jane@example.com"))
}

func TestBuildSearchBody_RawGroupsWins(t *testing.T) {
	RegisterTestingT(t)

	raw := []interface{}{
		map[string]interface{}{"filters": []interface{}{
			map[string]interface{}{"propertyName": "amount", "operator": "GT", "value": "1000"},
		}},
	}
	inputs := []*core.Connection{
		strConn("filter_property", "email"), // should be ignored in favour of raw groups
		strConn("filter_value", "x@y.com"),
		{Name: "filter_groups", Type: core.ConnectionTypeObject, Value: raw},
	}

	body := BuildSearchBody(inputs)
	Expect(body["filterGroups"]).To(Equal(raw))
}

func TestBuildSearchBody_HasPropertyOmitsValue(t *testing.T) {
	RegisterTestingT(t)

	// A populated filter_value must NOT be attached when the operator is an
	// existence check — HubSpot rejects HAS_PROPERTY/NOT_HAS_PROPERTY with a value.
	for _, op := range []string{"HAS_PROPERTY", "NOT_HAS_PROPERTY"} {
		inputs := []*core.Connection{
			strConn("filter_property", "email"),
			strConn("filter_operator", op),
			strConn("filter_value", "leftover"),
		}
		body := BuildSearchBody(inputs)
		filter := body["filterGroups"].([]interface{})[0].(map[string]interface{})["filters"].([]interface{})[0].(map[string]interface{})
		Expect(filter["operator"]).To(Equal(op))
		Expect(filter).NotTo(HaveKey("value"), "operator %s must not carry a value", op)
	}
}

func TestCheckResponse(t *testing.T) {
	RegisterTestingT(t)

	Expect(CheckResponse(&APIResponse{StatusCode: http.StatusOK, Body: []byte(`{}`)})).To(Succeed())

	err := CheckResponse(&APIResponse{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"status":"error","message":"Property \"foo\" does not exist","category":"VALIDATION_ERROR"}`),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("VALIDATION_ERROR"))
	Expect(err.Error()).To(ContainSubstring("does not exist"))

	// Non-JSON body falls back to the raw payload.
	err = CheckResponse(&APIResponse{StatusCode: http.StatusInternalServerError, Body: []byte("upstream boom")})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("upstream boom"))
}

func TestObjectResult(t *testing.T) {
	RegisterTestingT(t)

	obj := map[string]interface{}{
		"id":         "501",
		"properties": map[string]interface{}{"email": "jane@example.com"},
	}
	out := ObjectResult(obj, "Created contact 501")
	Expect(out["id"]).To(Equal("501"))
	Expect(out["success"]).To(Equal(true))
	Expect(out["error"]).To(Equal(""))
	// tool_result now carries the summary PLUS the embedded record JSON so
	// AI callers (which read tool_result verbatim) also receive the data.
	Expect(out["tool_result"]).To(ContainSubstring("Created contact 501"))
	Expect(out["tool_result"]).To(ContainSubstring("jane@example.com"))
	Expect(out["properties"]).To(Equal(map[string]interface{}{"email": "jane@example.com"}))
}

func TestListResult(t *testing.T) {
	RegisterTestingT(t)

	resp := map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{"id": "1"},
			map[string]interface{}{"id": "2"},
		},
		"paging": map[string]interface{}{
			"next": map[string]interface{}{"after": "cursor-xyz"},
		},
	}
	out := ListResult(resp, "Listed 2 contacts")
	Expect(out["count"]).To(Equal(2))
	Expect(out["after"]).To(Equal("cursor-xyz"))
	Expect(out["success"]).To(Equal(true))
}
