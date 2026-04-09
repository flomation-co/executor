package fulfill_commitment

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"

	. "github.com/onsi/gomega"
)

type fakeAPI struct {
	server    *httptest.Server
	gotPath   string
	gotMethod string
	status    int
}

func newFakeAPI(status int) *fakeAPI {
	f := &fakeAPI{status: status}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotPath = r.URL.Path
		f.gotMethod = r.Method
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, "")
	}))
	return f
}

func (f *fakeAPI) close() { f.server.Close() }

func strInput(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestExecute_HappyPath(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusNoContent)
	defer api.close()

	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{APIURL: api.server.URL})

	out, err := Execute(flow, nil, []*core.Connection{
		strInput("commitment_id", "c-42"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(api.gotMethod).To(Equal(http.MethodPatch))
	Expect(api.gotPath).To(Equal("/api/v1/internal/commitment/c-42"))
}

func TestExecute_MissingID_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{APIURL: "http://example.invalid"})

	_, err := Execute(flow, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("commitment_id"))
}

func TestExecute_APINon204_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI(http.StatusInternalServerError)
	defer api.close()

	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{APIURL: api.server.URL})

	_, err := Execute(flow, nil, []*core.Connection{
		strInput("commitment_id", "c-42"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("500"))
}
