package remember

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"

	. "github.com/onsi/gomega"
)

// fakeAPI captures the request body and path, and replies with a canned
// status + body so tests can assert on what the action sent and received.
type fakeAPI struct {
	server      *httptest.Server
	gotPath     string
	gotMethod   string
	gotHeader   http.Header
	gotBody     map[string]interface{}
	replyStatus int
	replyBody   string
}

func newFakeAPI(replyStatus int, replyBody string) *fakeAPI {
	f := &fakeAPI{replyStatus: replyStatus, replyBody: replyBody}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotPath = r.URL.Path
		f.gotMethod = r.Method
		f.gotHeader = r.Header.Clone()
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &f.gotBody)
		}
		w.WriteHeader(f.replyStatus)
		_, _ = io.WriteString(w, f.replyBody)
	}))
	return f
}

func (f *fakeAPI) close() { f.server.Close() }

// flowWithContext builds a Flow with an attached ExecutionContext.
// All action tests in this package want exactly this shape.
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

// --- happy path ---

func TestExecute_HappyPath(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusCreated, `{"id":"mem-42"}`)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	inputs := []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("agent_user_id", "user-abc"),
		strInput("scope", "user"),
		strInput("memory_type", "preference"),
		strInput("title", "Preferred name"),
		strInput("body", "Prefers Andy over Andrew"),
		boolInput("pinned", true),
		strInput("confidence", "0.95"),
	}

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["memory_id"]).To(Equal("mem-42"))

	Expect(api.gotMethod).To(Equal(http.MethodPost))
	Expect(api.gotPath).To(Equal("/api/v1/internal/agent/agent-1/memory"))
	Expect(api.gotHeader.Get("Content-Type")).To(Equal("application/json"))

	Expect(api.gotBody["scope"]).To(Equal("user"))
	Expect(api.gotBody["memory_type"]).To(Equal("preference"))
	Expect(api.gotBody["title"]).To(Equal("Preferred name"))
	Expect(api.gotBody["body"]).To(Equal("Prefers Andy over Andrew"))
	Expect(api.gotBody["agent_user_id"]).To(Equal("user-abc"))
	Expect(api.gotBody["pinned"]).To(BeTrue())
	Expect(api.gotBody["confidence"]).To(BeNumerically("~", 0.95, 0.001))
}

// --- input validation ---

func TestExecute_MissingAgentID_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := flowWithContext("http://example.invalid")
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("scope", "user"),
		strInput("memory_type", "fact"),
		strInput("title", "x"),
		strInput("body", "y"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("agent_id"))
}

func TestExecute_MissingBody_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := flowWithContext("http://example.invalid")
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("scope", "user"),
		strInput("memory_type", "fact"),
		strInput("title", "x"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("body"))
}

func TestExecute_MissingContext_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	// No context at all — simulates an executor bug where the runner
	// didn't hydrate one. Action should fail loudly rather than dereference nil.
	flow := &core.Flow{}
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("scope", "user"),
		strInput("memory_type", "fact"),
		strInput("title", "x"),
		strInput("body", "y"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("API URL"))
}

// --- optional field semantics ---

func TestExecute_OmittedPinnedAndConfidence_NotSentInPayload(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusCreated, `{"id":"mem-1"}`)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("scope", "global"),
		strInput("memory_type", "fact"),
		strInput("title", "UTC"),
		strInput("body", "Operates in UTC"),
	})
	Expect(err).NotTo(HaveOccurred())

	// pinned/confidence omitted — handler-side default applies, but the
	// executor should NOT send them so the omission is visible to the
	// API and it can apply its own defaults.
	_, pinnedPresent := api.gotBody["pinned"]
	_, confPresent := api.gotBody["confidence"]
	Expect(pinnedPresent).To(BeFalse(), "pinned should not be in payload when omitted")
	Expect(confPresent).To(BeFalse(), "confidence should not be in payload when omitted")

	// agent_user_id was also not provided — global-scope memory.
	_, userPresent := api.gotBody["agent_user_id"]
	Expect(userPresent).To(BeFalse())
}

// --- error propagation ---

func TestExecute_APINon201_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusInternalServerError, `{"error":"boom"}`)
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("scope", "user"),
		strInput("memory_type", "fact"),
		strInput("title", "x"),
		strInput("body", "y"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("500"))
	Expect(err.Error()).To(ContainSubstring("boom"))
}
