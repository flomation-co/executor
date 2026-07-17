// Happy-path + error-path coverage for the azure/cosmosdb item actions:
// strict create vs upsert, partition-key discovery and headers, query wire
// shape and pagination, replace/patch/delete semantics.
package cosmosdb_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	. "github.com/onsi/gomega"

	core "flomation.app/automate/executor"
	container_create "flomation.app/automate/executor/actions/azure/cosmosdb/container_create"
	container_delete "flomation.app/automate/executor/actions/azure/cosmosdb/container_delete"
	container_replace "flomation.app/automate/executor/actions/azure/cosmosdb/container_replace"
	item_create "flomation.app/automate/executor/actions/azure/cosmosdb/item_create"
	item_delete "flomation.app/automate/executor/actions/azure/cosmosdb/item_delete"
	item_get "flomation.app/automate/executor/actions/azure/cosmosdb/item_get"
	item_get_all "flomation.app/automate/executor/actions/azure/cosmosdb/item_get_all"
	item_patch "flomation.app/automate/executor/actions/azure/cosmosdb/item_patch"
	item_query "flomation.app/automate/executor/actions/azure/cosmosdb/item_query"
	item_replace "flomation.app/automate/executor/actions/azure/cosmosdb/item_replace"
)

// itemServer serves the container definition (for partition-key discovery)
// and hands everything else to handle.
func itemServer(pkPath string, handle http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/colls/c1") {
			_, _ = w.Write([]byte(`{"id":"c1","partitionKey":{"paths":["` + pkPath + `"]}}`))
			return
		}
		handle(w, r)
	}))
}

func TestItemCreateStrictByDefault(t *testing.T) {
	RegisterTestingT(t)
	var gotUpsert, gotPK string
	var gotBody map[string]interface{}
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		gotUpsert = r.Header.Get("x-ms-documentdb-is-upsert")
		gotPK = r.Header.Get("x-ms-documentdb-partitionkey")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("x-ms-request-charge", "6.29")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(raw)
	})
	defer server.Close()

	out, err := item_create.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		object("item", `{"id":"i1","status":"open"}`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotUpsert).To(BeEmpty(), "strict create must NOT send the upsert header")
	Expect(gotPK).To(Equal(`["i1"]`), "a /id container derives the partition key from the item id")
	Expect(gotBody["id"]).To(Equal("i1"))
	Expect(out["id"]).To(Equal("i1"))
	Expect(out["request_charge"]).To(Equal("6.29"))
}

func TestItemCreateUpsertHeader(t *testing.T) {
	RegisterTestingT(t)
	var gotUpsert string
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		gotUpsert = r.Header.Get("x-ms-documentdb-is-upsert")
		raw, _ := io.ReadAll(r.Body)
		_, _ = w.Write(raw)
	})
	defer server.Close()

	out, err := item_create.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		object("item", `{"id":"i1"}`), boolean("upsert", true)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotUpsert).To(Equal("True"))
}

func TestItemCreateConflictIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"Conflict","message":"Message: {\"Errors\":[\"Resource with specified id or name already exists.\"]}"}`))
	})
	defer server.Close()

	out, err := item_create.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), object("item", `{"id":"i1"}`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("enable Upsert"))
}

func TestItemCreateRequiresIDWhenStrict(t *testing.T) {
	RegisterTestingT(t)
	out, err := item_create.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "d1"), str("container", "c1"), object("item", `{"status":"open"}`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring(`"id"`))
}

func TestItemGetSendsPartitionKeyHeader(t *testing.T) {
	RegisterTestingT(t)
	var gotPK, gotPath string
	server := itemServer("/category", func(w http.ResponseWriter, r *http.Request) {
		gotPK = r.Header.Get("x-ms-documentdb-partitionkey")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"i1","category":"books","_rid":"sys"}`))
	})
	defer server.Close()

	out, err := item_get.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "i1"), str("partition_key", "books")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotPK).To(Equal(`["books"]`))
	Expect(gotPath).To(Equal("/dbs/d1/colls/c1/docs/i1"))
	Expect(out["result"]).To(Equal(map[string]interface{}{"id": "i1", "category": "books"}), "simplify defaults on")
}

func TestItemGetRequiresPartitionKeyForCustomPath(t *testing.T) {
	RegisterTestingT(t)
	server := itemServer("/category", func(w http.ResponseWriter, r *http.Request) {
		t.Error("the point read must not fire when the partition key cannot be resolved")
	})
	defer server.Close()

	out, err := item_get.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "i1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("/category"))
}

func TestItemGetNotFoundIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NotFound","message":"gone"}`))
	})
	defer server.Close()

	out, err := item_get.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "nope")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("not found"))
}

func TestItemGetAllPaginates(t *testing.T) {
	RegisterTestingT(t)
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("x-ms-continuation", "next")
			_, _ = w.Write([]byte(`{"Documents":[{"id":"a"}],"_count":1}`))
			return
		}
		Expect(r.Header.Get("x-ms-continuation")).To(Equal("next"))
		_, _ = w.Write([]byte(`{"Documents":[{"id":"b"}],"_count":1}`))
	}))
	defer server.Close()

	out, err := item_get_all.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), boolean("return_all", true)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(2))
}

func TestItemGetAllErrorIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NotFound","message":"no such container"}`))
	}))
	defer server.Close()

	out, err := item_get_all.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "nope")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
}

func TestItemQueryWireShape(t *testing.T) {
	RegisterTestingT(t)
	var gotContentType, gotIsQuery, gotCross string
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotIsQuery = r.Header.Get("x-ms-documentdb-isquery")
		gotCross = r.Header.Get("x-ms-documentdb-query-enablecrosspartition")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"Documents":[{"id":"a","status":"open"}],"_count":1}`))
	}))
	defer server.Close()

	out, err := item_query.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		text("query", "SELECT * FROM c WHERE c.status = @status"),
		object("parameters", `{"status":"open"}`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(1))
	Expect(gotContentType).To(Equal("application/query+json"))
	Expect(gotIsQuery).To(Equal("True"))
	Expect(gotCross).To(Equal("True"), "no partition key ⇒ cross-partition")
	Expect(gotBody["parameters"]).To(Equal([]interface{}{
		map[string]interface{}{"name": "@status", "value": "open"},
	}), "a missing @ prefix is added")
}

func TestItemQueryScopedToPartition(t *testing.T) {
	RegisterTestingT(t)
	var gotPK, gotCross string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPK = r.Header.Get("x-ms-documentdb-partitionkey")
		gotCross = r.Header.Get("x-ms-documentdb-query-enablecrosspartition")
		_, _ = w.Write([]byte(`{"Documents":[],"_count":0}`))
	}))
	defer server.Close()

	out, err := item_query.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		text("query", "SELECT * FROM c"), str("partition_key", "books")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotPK).To(Equal(`["books"]`))
	Expect(gotCross).To(BeEmpty(), "a scoped query must not also fan out cross-partition")
}

// TestItemQueryPaginatesWithContinuation covers the spec's headline advantage
// over n8n, whose query operation has no pagination and silently truncates at
// the server's default page size. The sharp edge is that a query is a POST:
// every continuation page must re-send the SAME body, or page 2 asks the
// server to run an empty query. The Limit input must reach the wire as
// x-ms-max-item-count, and the reported RU charge must be the SUM across
// pages — a per-page charge would under-report what the account was billed.
func TestItemQueryPaginatesWithContinuation(t *testing.T) {
	RegisterTestingT(t)
	var calls int32
	var bodies []string
	var itemCounts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		itemCounts = append(itemCounts, r.Header.Get("x-ms-max-item-count"))
		w.Header().Set("x-ms-request-charge", "2.50")
		if atomic.AddInt32(&calls, 1) == 1 {
			Expect(r.Header.Get("x-ms-continuation")).To(BeEmpty(), "the first page must not carry a continuation token")
			w.Header().Set("x-ms-continuation", "token-1")
			_, _ = w.Write([]byte(`{"Documents":[{"id":"a"}],"_count":1}`))
			return
		}
		Expect(r.Header.Get("x-ms-continuation")).To(Equal("token-1"), "the token must be echoed back verbatim")
		_, _ = w.Write([]byte(`{"Documents":[{"id":"b"}],"_count":1}`))
	}))
	defer server.Close()

	out, err := item_query.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		text("query", "SELECT * FROM c WHERE c.status = @status"),
		object("parameters", `{"@status":"open"}`),
		boolean("return_all", true), integer("limit", 100)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(2), "both pages' items must be accumulated")
	Expect(calls).To(Equal(int32(2)))

	Expect(bodies).To(HaveLen(2))
	Expect(bodies[1]).To(Equal(bodies[0]),
		"the query body must be re-sent unchanged on every continuation page — a POST feed that drops its body runs an empty query on page 2")
	Expect(bodies[0]).To(ContainSubstring("@status"))

	Expect(itemCounts).To(Equal([]string{"100", "100"}), "the Limit input is the per-page x-ms-max-item-count on every page")
	Expect(out["request_charge"]).To(Equal("5.00"), "the RU charge is summed across pages, not just the last one")
}

// TestItemQuerySinglePageWithoutReturnAll is the other half of the contract:
// with Return All off, a continuation token in the response is ignored rather
// than followed, so a big query costs one page of RU.
func TestItemQuerySinglePageWithoutReturnAll(t *testing.T) {
	RegisterTestingT(t)
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("x-ms-continuation", "more-pages-available")
		_, _ = w.Write([]byte(`{"Documents":[{"id":"a"}],"_count":1}`))
	}))
	defer server.Close()

	out, err := item_query.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		text("query", "SELECT * FROM c")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(1))
	Expect(calls).To(Equal(int32(1)), "Return All is off — the continuation token must not be followed")
}

func TestItemQueryBadQueryIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"BadRequest","message":"Syntax error, incorrect syntax near 'FORM'."}`))
	}))
	defer server.Close()

	out, err := item_query.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), text("query", "SELECT * FORM c")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Syntax error"))
}

func TestItemReplaceInjectsIDAndUsesPut(t *testing.T) {
	RegisterTestingT(t)
	var gotMethod, gotPK string
	var gotBody map[string]interface{}
	server := itemServer("/category", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPK = r.Header.Get("x-ms-documentdb-partitionkey")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write(raw)
	})
	defer server.Close()

	out, err := item_replace.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "i1"),
		object("item", `{"category":"books","status":"closed"}`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotMethod).To(Equal(http.MethodPut))
	Expect(gotBody["id"]).To(Equal("i1"), "the Item ID input is injected into the body")
	Expect(gotPK).To(Equal(`["books"]`), "the partition key comes from the body property")
}

func TestItemReplaceEtagMismatchIsSoft(t *testing.T) {
	RegisterTestingT(t)
	var gotIfMatch string
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		gotIfMatch = r.Header.Get("If-Match")
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`{"code":"PreconditionFailed","message":"etag mismatch"}`))
	})
	defer server.Close()

	out, err := item_replace.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "i1"),
		object("item", `{"a":1}`), str("etag", `"00c0-1234"`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(gotIfMatch).To(Equal(`"00c0-1234"`))
	Expect(out["error"]).To(ContainSubstring("modified since it was read"))
}

func TestItemPatchWireShape(t *testing.T) {
	RegisterTestingT(t)
	var gotMethod, gotContentType string
	var gotBody map[string]interface{}
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":"i1","status":"done","version":2}`))
	})
	defer server.Close()

	out, err := item_patch.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "i1"),
		object("operations", `[{"op":"set","path":"/status","value":"done"},{"op":"incr","path":"/version","value":1}]`),
		str("condition", "FROM c WHERE c.status = 'open'")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotMethod).To(Equal(http.MethodPatch))
	Expect(gotContentType).To(Equal("application/json_patch+json"))
	Expect(gotBody["operations"]).To(HaveLen(2))
	Expect(gotBody["condition"]).To(Equal("FROM c WHERE c.status = 'open'"))
}

func TestItemPatchValidatesOperations(t *testing.T) {
	RegisterTestingT(t)
	// Unknown op — the Cosmos set is not RFC 6902 (no "test").
	out, err := item_patch.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "d1"), str("container", "c1"), str("item_id", "i1"),
		object("operations", `[{"op":"test","path":"/a","value":1}]`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("add, set, replace, remove, incr, move"))

	// More than 10 operations.
	ops := make([]string, 11)
	for i := range ops {
		ops[i] = `{"op":"set","path":"/a","value":1}`
	}
	out, err = item_patch.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "d1"), str("container", "c1"), str("item_id", "i1"),
		object("operations", "["+strings.Join(ops, ",")+"]")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("at most 10"))
}

func TestItemDelete(t *testing.T) {
	RegisterTestingT(t)
	var gotMethod, gotPK string
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPK = r.Header.Get("x-ms-documentdb-partitionkey")
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	out, err := item_delete.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "i1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotMethod).To(Equal(http.MethodDelete))
	Expect(gotPK).To(Equal(`["i1"]`))
	Expect(out["result"]).To(HaveKeyWithValue("deleted", true))
}

func TestItemDeleteNotFoundIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NotFound","message":"gone"}`))
	})
	defer server.Close()

	out, err := item_delete.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), str("item_id", "nope")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("not found"))
}

// TestItemGetAllReturnAllReportsTruncation is the operator's case: a container
// far bigger than the page budget, listed with Return All. The action must not
// claim a complete list — a downstream reconcile ("delete anything not in this
// list") would act on items that were simply never fetched.
func TestItemGetAllReturnAllReportsTruncation(t *testing.T) {
	RegisterTestingT(t)
	var calls int32
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("x-ms-continuation", fmt.Sprintf("token-%d", n))
		w.Header().Set("x-ms-request-charge", "1")
		_, _ = w.Write([]byte(`{"Documents":[{"id":"i1"}]}`))
	})
	defer server.Close()

	out, err := item_get_all.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"), boolean("return_all", true)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["truncated"]).To(BeTrue())
	Expect(out["next_continuation"]).NotTo(BeEmpty(), "the operator must be able to resume")
	Expect(out["tool_result"]).To(ContainSubstring("PARTIAL"))
	Expect(out["tool_result"]).To(ContainSubstring("safety cap"))
}

// TestItemGetAllResumesFromContinuation: the Next Continuation output feeds
// straight back into the Continuation input.
func TestItemGetAllResumesFromContinuation(t *testing.T) {
	RegisterTestingT(t)
	var gotContinuation string
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		gotContinuation = r.Header.Get("x-ms-continuation")
		_, _ = w.Write([]byte(`{"Documents":[{"id":"i2"}]}`))
	})
	defer server.Close()

	out, err := item_get_all.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		boolean("return_all", true), str("continuation", "token-200")))
	Expect(err).To(BeNil())
	Expect(gotContinuation).To(Equal("token-200"))
	Expect(out["count"]).To(Equal(1))
	Expect(out["truncated"]).To(BeFalse())
}

// TestItemQueryReturnAllReportsTruncation: the same contract on the query path,
// where a silently short result set is most likely to be trusted.
func TestItemQueryReturnAllReportsTruncation(t *testing.T) {
	RegisterTestingT(t)
	server := itemServer("/id", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-continuation", "more")
		_, _ = w.Write([]byte(`{"Documents":[{"id":"i1"}]}`))
	})
	defer server.Close()

	out, err := item_query.Execute(nil, nil, authFor(server.URL,
		str("database", "d1"), str("container", "c1"),
		text("query", "SELECT * FROM c"), boolean("return_all", true)))
	Expect(err).To(BeNil())
	Expect(out["truncated"]).To(BeTrue())
	Expect(out["tool_result"]).To(ContainSubstring("safety cap"))
}

// TestRecreatedContainerDoesNotReuseStalePartitionKey walks the reset-and-
// reload flow the pk cache used to break: read an /id container, delete it,
// recreate the SAME name on /customerId, then create an item. The header must
// carry the new path's value ["c-9"], not the old path's ["o-1"] — Cosmos
// rejects the mismatch with a message that blames the (correct) flow.
func TestRecreatedContainerDoesNotReuseStalePartitionKey(t *testing.T) {
	RegisterTestingT(t)
	pkPath := "/id"
	var gotPK string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/colls/orders"):
			_, _ = w.Write([]byte(`{"id":"orders","partitionKey":{"paths":["` + pkPath + `"]}}`))
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/colls/orders"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/colls"):
			raw, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(raw)
		default: // the item create
			gotPK = r.Header.Get("x-ms-documentdb-partitionkey")
			raw, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(raw)
		}
	}))
	defer server.Close()

	base := func(more ...*core.Connection) []*core.Connection {
		return authFor(server.URL, append([]*core.Connection{
			str("database", "d1"), str("container", "orders")}, more...)...)
	}

	// Touch the container so the old /id path lands in the cache.
	out, err := item_get.Execute(nil, nil, base(str("item_id", "o-1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())

	out, err = container_delete.Execute(nil, nil, base())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())

	pkPath = "/customerId" // the container comes back partitioned differently
	out, err = container_create.Execute(nil, nil, base(str("partition_key_path", "/customerId")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())

	out, err = item_create.Execute(nil, nil, base(object("item", `{"id":"o-1","customerId":"c-9"}`)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotPK).To(Equal(`["c-9"]`), "the recreated container is partitioned on /customerId — a cached /id would send [\"o-1\"] and Cosmos would reject the document/header mismatch")
}

// TestContainerLifecycleForcesPartitionKeyRediscovery covers the half of the
// stale-cache window that container_create's seed does NOT: a container that is
// deleted or redefined here and comes back by some other route (a peer flow,
// the portal, an ARM template). The next item op must re-read the definition
// rather than answer from a cache entry describing a container that no longer
// exists.
func TestContainerLifecycleForcesPartitionKeyRediscovery(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(inputs []*core.Connection) (map[string]interface{}, error)
	}{
		{"delete", func(in []*core.Connection) (map[string]interface{}, error) {
			return container_delete.Execute(nil, nil, in)
		}},
		{"replace", func(in []*core.Connection) (map[string]interface{}, error) {
			return container_replace.Execute(nil, nil, append(in, integer("default_ttl", 60)))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			var discoveries int32
			coll := "orders_" + tc.name // a fresh name per case — the cache is package state
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/colls/"+coll):
					atomic.AddInt32(&discoveries, 1)
					_, _ = w.Write([]byte(`{"id":"` + coll + `","partitionKey":{"paths":["/id"]}}`))
				case r.Method == http.MethodDelete:
					w.WriteHeader(http.StatusNoContent)
				default: // the container PUT and the item POST both echo their body
					raw, _ := io.ReadAll(r.Body)
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(raw)
				}
			}))
			defer server.Close()

			base := func(more ...*core.Connection) []*core.Connection {
				return authFor(server.URL, append([]*core.Connection{
					str("database", "d1"), str("container", coll)}, more...)...)
			}

			_, err := item_get.Execute(nil, nil, base(str("item_id", "o-1")))
			Expect(err).To(BeNil())
			before := atomic.LoadInt32(&discoveries)
			Expect(before).To(Equal(int32(1)), "the first item op discovers the path")

			out, err := tc.run(base())
			Expect(err).To(BeNil())
			Expect(out["success"]).To(BeTrue())

			// container_replace reads the definition itself; count only what the
			// item op forces.
			atomic.StoreInt32(&discoveries, 0)
			_, err = item_get.Execute(nil, nil, base(str("item_id", "o-2")))
			Expect(err).To(BeNil())
			Expect(atomic.LoadInt32(&discoveries)).To(Equal(int32(1)),
				"the cached path must not outlive a container_"+tc.name+" — it may describe a container that no longer exists")
		})
	}
}
