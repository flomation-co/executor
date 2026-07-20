// Happy-path + error-path coverage for the azure/cosmosdb container and
// throughput actions, including the offers signing quirk end-to-end.
package cosmosdb_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	cosmosdb "flomation.app/automate/executor/actions/azure/cosmosdb"
	container_create "flomation.app/automate/executor/actions/azure/cosmosdb/container_create"
	container_delete "flomation.app/automate/executor/actions/azure/cosmosdb/container_delete"
	container_get "flomation.app/automate/executor/actions/azure/cosmosdb/container_get"
	container_get_all "flomation.app/automate/executor/actions/azure/cosmosdb/container_get_all"
	container_replace "flomation.app/automate/executor/actions/azure/cosmosdb/container_replace"
	throughput_get "flomation.app/automate/executor/actions/azure/cosmosdb/throughput_get"
	throughput_update "flomation.app/automate/executor/actions/azure/cosmosdb/throughput_update"
)

func TestContainerCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotBody map[string]interface{}
	var gotAutoscale, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAutoscale = r.Header.Get("x-ms-cosmos-offer-autopilot-setting")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"c1"}`))
	}))
	defer server.Close()

	out, err := container_create.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		str("partition_key_path", "/category"),
		str("unique_key_paths", "/email, /employeeId"),
		integer("autoscale_max", 4000)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotPath).To(Equal("/dbs/d1/colls"))
	// The autoscale header is the documented JSON object, not a bare number.
	Expect(gotAutoscale).To(Equal(`{"maxThroughput":4000}`))
	Expect(gotBody["id"]).To(Equal("c1"))
	Expect(gotBody["partitionKey"]).To(Equal(map[string]interface{}{
		"paths": []interface{}{"/category"}, "kind": "Hash", "version": float64(2),
	}))
	Expect(gotBody["uniqueKeyPolicy"]).To(Equal(map[string]interface{}{
		"uniqueKeys": []interface{}{
			map[string]interface{}{"paths": []interface{}{"/email"}},
			map[string]interface{}{"paths": []interface{}{"/employeeId"}},
		},
	}))
}

// TestContainerCreateDefaultsToIDPartitionKey pins the spec's default: leaving
// Partition Key Path blank creates an /id-partitioned container. That default
// is what makes the whole item layer ergonomic — a /id container lets every
// item action derive the partition key from the item id, so the operator never
// sees a partition-key field. It also covers the optional policy inputs and
// the manual (non-autoscale) throughput header, none of which the autoscale
// test exercises.
func TestContainerCreateDefaultsToIDPartitionKey(t *testing.T) {
	RegisterTestingT(t)
	var gotBody map[string]interface{}
	var gotThroughput, gotAutoscale string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotThroughput = r.Header.Get("x-ms-offer-throughput")
		gotAutoscale = r.Header.Get("x-ms-cosmos-offer-autopilot-setting")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"c1"}`))
	}))
	defer server.Close()

	out, err := container_create.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		object("indexing_policy", `{"indexingMode":"consistent","automatic":true}`),
		integer("default_ttl", 3600),
		integer("throughput", 400)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotBody["partitionKey"]).To(Equal(map[string]interface{}{
		"paths": []interface{}{"/id"}, "kind": "Hash", "version": float64(2),
	}), "a blank Partition Key Path must default to /id")
	Expect(gotBody["indexingPolicy"]).To(Equal(map[string]interface{}{
		"indexingMode": "consistent", "automatic": true,
	}))
	Expect(gotBody["defaultTtl"]).To(Equal(float64(3600)))
	Expect(gotBody).ToNot(HaveKey("uniqueKeyPolicy"), "no unique_key_paths ⇒ no uniqueKeyPolicy at all")
	Expect(gotThroughput).To(Equal("400"), "manual throughput rides x-ms-offer-throughput")
	Expect(gotAutoscale).To(BeEmpty(), "manual throughput must not also send the autopilot header")
}

// TestContainerCreateConflictIsSoft — re-creating an existing container is an
// operator mistake, not a crash.
func TestContainerCreateConflictIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"Conflict","message":"Resource with specified id already exists."}`))
	}))
	defer server.Close()

	out, err := container_create.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("already exists"))
}

func TestContainerCreateRejectsBadPartitionKeyPath(t *testing.T) {
	RegisterTestingT(t)
	out, err := container_create.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "d1"), str("container", "c1"), str("partition_key_path", "category")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("must start with /"))
}

func TestContainerGetKeepsSystemPropsWhenSimplifyOff(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"c1","_rid":"keep-me","partitionKey":{"paths":["/id"]}}`))
	}))
	defer server.Close()

	out, err := container_get.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), boolean("simplify", false)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	result := out["result"].(map[string]interface{})
	Expect(result["_rid"]).To(Equal("keep-me"))
}

func TestContainerGetNotFoundIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NotFound","message":"gone"}`))
	}))
	defer server.Close()

	out, err := container_get.Execute(nil, nil, authFor(server.URL, str("database", "d1"), str("container", "nope")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("not found"))
}

func TestContainerGetAll(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"DocumentCollections":[{"id":"c1"},{"id":"c2"}],"_count":2}`))
	}))
	defer server.Close()

	out, err := container_get_all.Execute(nil, nil, authFor(server.URL, str("database", "d1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(2))
}

func TestContainerGetAllErrorIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"Forbidden","message":"nope"}`))
	}))
	defer server.Close()

	out, err := container_get_all.Execute(nil, nil, authFor(server.URL, str("database", "d1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
}

// TestContainerReplaceIsReadModifyWrite: the PUT must carry the CURRENT
// definition with only the requested changes overlaid — the partition key
// unchanged, unrelated fields (uniqueKeyPolicy) preserved, and system props
// stripped.
func TestContainerReplaceIsReadModifyWrite(t *testing.T) {
	RegisterTestingT(t)
	var putBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"c1","_rid":"sys","partitionKey":{"paths":["/pk"],"kind":"Hash"},"defaultTtl":100,"uniqueKeyPolicy":{"uniqueKeys":[{"paths":["/email"]}]}}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &putBody)
		_, _ = w.Write([]byte(`{"id":"c1","defaultTtl":3600}`))
	}))
	defer server.Close()

	out, err := container_replace.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), integer("default_ttl", 3600)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(putBody["defaultTtl"]).To(Equal(float64(3600)))
	Expect(putBody["partitionKey"]).To(Equal(map[string]interface{}{"paths": []interface{}{"/pk"}, "kind": "Hash"}))
	Expect(putBody["uniqueKeyPolicy"]).NotTo(BeNil(), "unchanged fields must survive the round trip")
	Expect(putBody).NotTo(HaveKey("_rid"), "system props must not be written back")
}

func TestContainerReplaceWithNothingToChangeIsSoft(t *testing.T) {
	RegisterTestingT(t)
	out, err := container_replace.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "d1"), str("container", "c1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("nothing to change"))
}

func TestContainerDelete(t *testing.T) {
	RegisterTestingT(t)
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	out, err := container_delete.Execute(nil, nil, authFor(server.URL, str("database", "d1"), str("container", "c1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotMethod).To(Equal(http.MethodDelete))
}

func TestContainerDeleteNotFoundIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NotFound","message":"gone"}`))
	}))
	defer server.Close()

	out, err := container_delete.Execute(nil, nil, authFor(server.URL, str("database", "d1"), str("container", "nope")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
}

// offersTestServer answers the three-call offers dance: GET the container
// (returns its _rid), POST /offers (the lookup query), and PUT /offers/{id}
// (the rescale). It captures the offers-query and PUT requests so tests can
// verify the signing quirks.
func offersTestServer(t *testing.T, offer string) (*httptest.Server, *offersCapture) {
	t.Helper()
	cap := &offersCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/dbs/"):
			_, _ = w.Write([]byte(`{"id":"c1","_rid":"CollRid==","partitionKey":{"paths":["/id"]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/offers":
			cap.queryAuth = r.Header.Get("Authorization")
			cap.queryDate = r.Header.Get("x-ms-date")
			cap.queryContentType = r.Header.Get("Content-Type")
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &cap.queryBody)
			if offer == "" {
				_, _ = w.Write([]byte(`{"Offers":[],"_count":0}`))
				return
			}
			_, _ = w.Write([]byte(`{"Offers":[` + offer + `],"_count":1}`))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/offers/"):
			cap.putAuth = r.Header.Get("Authorization")
			cap.putDate = r.Header.Get("x-ms-date")
			cap.putPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &cap.putBody)
			_, _ = w.Write(raw)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	return server, cap
}

type offersCapture struct {
	queryAuth, queryDate, queryContentType string
	queryBody                              map[string]interface{}
	putAuth, putDate, putPath              string
	putBody                                map[string]interface{}
}

const manualOffer = `{"id":"QoFF","_rid":"QoFF","_self":"offers/QoFF/","offerVersion":"V2","content":{"offerThroughput":500},"resource":"dbs/x/colls/y/","offerResourceId":"CollRid=="}`

func TestThroughputGet(t *testing.T) {
	RegisterTestingT(t)
	server, cap := offersTestServer(t, manualOffer)
	defer server.Close()

	out, err := throughput_get.Execute(nil, nil, authFor(server.URL, str("database", "d1"), str("container", "c1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("QoFF"))
	Expect(out["tool_result"]).To(ContainSubstring("manual, 500 RU/s"))

	// The offers query must go out as a query (content type + parameters)
	// and sign with an EMPTY resourceId — recompute from the served date.
	Expect(cap.queryContentType).To(Equal("application/query+json"))
	wantAuth, err := cosmosdb.MasterKeyAuthHeader(http.MethodPost, "offers", "", cap.queryDate, testMasterKey)
	Expect(err).To(BeNil())
	Expect(cap.queryAuth).To(Equal(wantAuth), "offer list/query signs with an empty resourceId")
	Expect(cap.queryBody["query"]).To(ContainSubstring("offerResourceId = @rid"))
}

func TestThroughputGetNoOfferIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server, _ := offersTestServer(t, "")
	defer server.Close()

	out, err := throughput_get.Execute(nil, nil, authFor(server.URL, str("database", "d1"), str("container", "c1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("serverless"))
}

func TestThroughputUpdateSignsWithLowercasedOfferRid(t *testing.T) {
	RegisterTestingT(t)
	server, cap := offersTestServer(t, manualOffer)
	defer server.Close()

	out, err := throughput_update.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), integer("throughput", 800)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())

	// URL path keeps the offer id's case; the SIGNATURE uses the lowercased
	// _rid ("qoff") — recomputing with anything else would not match.
	Expect(cap.putPath).To(Equal("/offers/QoFF"))
	wantAuth, err := cosmosdb.MasterKeyAuthHeader(http.MethodPut, "offers", "qoff", cap.putDate, testMasterKey)
	Expect(err).To(BeNil())
	Expect(cap.putAuth).To(Equal(wantAuth), "single-offer PUT signs with the offer _rid LOWERCASED")

	content := cap.putBody["content"].(map[string]interface{})
	Expect(content["offerThroughput"]).To(Equal(float64(800)))
}

func TestThroughputUpdateRequiresExactlyOneMode(t *testing.T) {
	RegisterTestingT(t)
	out, err := throughput_update.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "d1"), str("container", "c1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("throughput"))

	out, err = throughput_update.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "d1"), str("container", "c1"), integer("throughput", 500), integer("autoscale_max", 4000)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("mutually exclusive"))
}
