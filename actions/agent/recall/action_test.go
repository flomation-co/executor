package recall

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"

	. "github.com/onsi/gomega"
)

type fakeAPI struct {
	server      *httptest.Server
	gotPath     string
	gotRawQuery string
	gotMethod   string
	replyStatus int
	replyBody   string
}

func newFakeAPI(replyStatus int, replyBody string) *fakeAPI {
	f := &fakeAPI{replyStatus: replyStatus, replyBody: replyBody}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotPath = r.URL.Path
		f.gotRawQuery = r.URL.RawQuery
		f.gotMethod = r.Method
		w.WriteHeader(f.replyStatus)
		_, _ = io.WriteString(w, f.replyBody)
	}))
	return f
}

func (f *fakeAPI) close() { f.server.Close() }

func flowWithContext(apiURL string) *core.Flow {
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{APIURL: apiURL})
	return flow
}

func strInput(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolInput(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func intInput(name string, value int64) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: value}
}

// --- happy path ---

func TestExecute_HappyPath_ReturnsMemoriesAndCount(t *testing.T) {
	RegisterTestingT(t)

	replyBody := `[
		{"id":"m1","title":"Preferred name","body":"Andy","pinned":true,"memory_type":"preference"},
		{"id":"m2","title":"Timezone","body":"Europe/London","pinned":true,"memory_type":"preference"}
	]`
	api := newFakeAPI(http.StatusOK, replyBody)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("agent_user_id", "user-abc"),
		boolInput("pinned_only", true),
		intInput("limit", 10),
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(api.gotMethod).To(Equal(http.MethodGet))
	Expect(api.gotPath).To(Equal("/api/v1/internal/agent/agent-1/memory"))
	// Query string params are alphabetically sorted by url.Values.Encode.
	Expect(api.gotRawQuery).To(ContainSubstring("agent_user_id=user-abc"))
	Expect(api.gotRawQuery).To(ContainSubstring("pinned=true"))
	Expect(api.gotRawQuery).To(ContainSubstring("limit=10"))

	memories, ok := out["memories"].([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(memories).To(HaveLen(2))
	Expect(memories[0]["title"]).To(Equal("Preferred name"))
	Expect(out["count"]).To(Equal(2))
}

// --- input validation ---

func TestExecute_MissingAgentUserID_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := flowWithContext("http://example.invalid")
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("agent_user_id"))
}

// --- optional field semantics ---

func TestExecute_PinnedOnlyFalse_NotInQueryString(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusOK, `[]`)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())

	// pinned_only omitted → should not appear as pinned=true in the query.
	// The API defaults to pinned=false when the parameter is absent, so
	// the executor must not send it when the flow author didn't either.
	Expect(api.gotRawQuery).NotTo(ContainSubstring("pinned=true"))
}

func TestExecute_EmptyResult_ReturnsZeroCount(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusOK, `[]`)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["count"]).To(Equal(0))

	// A nil slice would break ${node.memories.length} in downstream
	// branches; force a typed empty slice instead.
	memories, ok := out["memories"].([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(memories).To(HaveLen(0))
}

// --- error propagation ---

func TestExecute_APINon200_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusServiceUnavailable, `{"error":"down"}`)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("503"))
}

// --- response decode safety ---

func TestExecute_MalformedJSONResponse_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusOK, `not json`)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("decode"))
}
