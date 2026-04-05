package forget

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
	gotMethod   string
	replyStatus int
	replyBody   string
}

func newFakeAPI(replyStatus int, replyBody string) *fakeAPI {
	f := &fakeAPI{replyStatus: replyStatus, replyBody: replyBody}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotPath = r.URL.Path
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

// --- happy path ---

func TestExecute_HappyPath_ReturnsSuccess(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusNoContent, "")
	defer api.close()

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("memory_id", "mem-42"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())

	Expect(api.gotMethod).To(Equal(http.MethodDelete))
	Expect(api.gotPath).To(Equal("/api/v1/internal/memory/mem-42"))
}

// --- input validation ---

func TestExecute_MissingMemoryID_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := flowWithContext("http://example.invalid")
	_, err := Execute(flow, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("memory_id"))
}

func TestExecute_EmptyMemoryID_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := flowWithContext("http://example.invalid")
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("memory_id", ""),
	})
	Expect(err).To(HaveOccurred())
}

// --- error propagation ---

func TestExecute_APINon204_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	// A 200 OK response from the API still counts as a failure for this
	// action because the contract is strict: the delete endpoint returns
	// 204 on success and anything else means the row may not have been
	// removed. Silent success here would leave the user thinking a
	// memory is forgotten when it is not.
	api := newFakeAPI(http.StatusOK, "unexpected success body")
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("memory_id", "mem-42"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("200"))
}

func TestExecute_APINotFound_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusNotFound, "")
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("memory_id", "mem-ghost"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("404"))
}
