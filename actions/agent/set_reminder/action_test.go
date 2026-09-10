package set_reminder

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	core "flomation.app/automate/executor"

	. "github.com/onsi/gomega"
)

type fakeAPI struct {
	server *httptest.Server
	bodies []map[string]interface{}
	status int
}

func newFakeAPI() *fakeAPI {
	f := &fakeAPI{status: http.StatusCreated}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		f.bodies = append(f.bodies, body)
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, `{"id":"commitment-1"}`)
	}))
	return f
}

func (f *fakeAPI) close() { f.server.Close() }

func strInput(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func run(api *fakeAPI, inputs ...*core.Connection) (map[string]interface{}, error) {
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{APIURL: api.server.URL})
	return Execute(flow, nil, inputs)
}

func TestADurationIsResolvedAndStored(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	out, err := run(api,
		strInput("when", "in 72 hours"),
		strInput("description", "Pull the updated campaign numbers"),
		strInput("agent_id", "agent-1"),
		strInput("conversation_id", "conv-1"),
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["commitment_id"]).To(Equal("commitment-1"))

	Expect(api.bodies).To(HaveLen(1))
	due, parseErr := time.Parse(time.RFC3339, api.bodies[0]["due_at"].(string))
	Expect(parseErr).NotTo(HaveOccurred())
	Expect(due).To(BeTemporally("~", time.Now().Add(72*time.Hour), time.Minute))
	Expect(api.bodies[0]["source_conversation"]).To(Equal("conv-1"))
	Expect(api.bodies[0]["made_by"]).To(Equal("assistant"))
}

func TestTheResolvedDateComesBackInTheToolResult(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	out, err := run(api,
		strInput("when", "in 72 hours"),
		strInput("description", "Pull the numbers"),
		strInput("agent_id", "agent-1"),
	)
	Expect(err).NotTo(HaveOccurred())

	// The whole point of the action: the agent is handed the date it
	// should quote, rather than computing one and being wrong.
	result := out["tool_result"].(string)
	Expect(result).To(ContainSubstring(out["due_at"].(string)))
	Expect(result).To(ContainSubstring(out["due_at_friendly"].(string)))
	Expect(result).To(ContainSubstring("Pull the numbers"))
	Expect(out["due_at_friendly"]).To(ContainSubstring(time.Now().Add(72 * time.Hour).UTC().Format("Monday")))
}

func TestACalendarDateIsAccepted(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	out, err := run(api,
		strInput("when", "1 October at 9am"),
		strInput("description", "Renew the certificate"),
		strInput("agent_id", "agent-1"),
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())

	due, parseErr := time.Parse(time.RFC3339, out["due_at"].(string))
	Expect(parseErr).NotTo(HaveOccurred())
	Expect(due.Month()).To(Equal(time.October))
	Expect(due.Day()).To(Equal(1))
	Expect(due.After(time.Now())).To(BeTrue(), "a named date must never be scheduled into the past")
}

func TestAnUnreadableTimeIsRefusedWithoutStoringAnything(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	out, err := run(api,
		strInput("when", "after the sprint review"),
		strInput("description", "Follow up"),
		strInput("agent_id", "agent-1"),
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(api.bodies).To(BeEmpty(), "nothing should be stored when the time cannot be read")
	Expect(out["tool_result"]).To(ContainSubstring("No reminder has been set"))
	Expect(out["tool_result"]).To(ContainSubstring("call this again"))
}

func TestRecurrenceIsPassedThroughOnlyWhenSet(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	_, err := run(api,
		strInput("when", "tomorrow at 8am"),
		strInput("description", "Morning briefing"),
		strInput("agent_id", "agent-1"),
		strInput("recurrence", "daily"),
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(api.bodies[0]["recurrence"]).To(Equal("daily"))

	_, err = run(api,
		strInput("when", "tomorrow at 8am"),
		strInput("description", "One-off"),
		strInput("agent_id", "agent-1"),
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(api.bodies[1]).ToNot(HaveKey("recurrence"))
}

func TestAFailedWriteIsReportedRatherThanClaimed(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	api.status = http.StatusInternalServerError
	defer api.close()

	out, err := run(api,
		strInput("when", "in 1 hour"),
		strInput("description", "Follow up"),
		strInput("agent_id", "agent-1"),
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("Nothing is scheduled"))
}

func TestToolResultIsTheFirstOutput(t *testing.T) {
	RegisterTestingT(t)

	Expect(Outputs[0].Name).To(Equal("tool_result"))
	Expect(len(Description)).To(BeNumerically("<", 120))
}
