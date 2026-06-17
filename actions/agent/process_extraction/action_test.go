package process_extraction

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	core "flomation.app/automate/executor"

	. "github.com/onsi/gomega"
)

// fakeAPI is a counting httptest.Server that categorises requests by
// URL path and optionally fails specific requests to simulate partial
// outages.
type fakeAPI struct {
	server *httptest.Server
	mu     sync.Mutex

	memoryCalls     []map[string]interface{}
	pendingCalls    []map[string]interface{}
	commitmentCalls []map[string]interface{}

	// When set, the Nth call of that type fails with 500. One-shot: the
	// counter is not reset after firing.
	failMemoryOnCall     int
	failPendingOnCall    int
	failCommitmentOnCall int
}

func newFakeAPI() *fakeAPI {
	f := &fakeAPI{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)

		// GETs against these paths are dedup lookups (the action fetches
		// existing memories / commitments before deciding to write new
		// ones). They're not the calls the test wants to count, so we
		// return an empty list and skip the capture. Only POST = create
		// is counted into the *Calls slices.
		isGetDedup := r.Method == http.MethodGet
		switch {
		case strings.HasSuffix(r.URL.Path, "/memory"):
			if isGetDedup {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			f.memoryCalls = append(f.memoryCalls, body)
			if f.failMemoryOnCall > 0 && len(f.memoryCalls) == f.failMemoryOnCall {
				http.Error(w, "simulated failure", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"mem-new"}`))
		case strings.HasSuffix(r.URL.Path, "/pending-action"):
			if isGetDedup {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			f.pendingCalls = append(f.pendingCalls, body)
			if f.failPendingOnCall > 0 && len(f.pendingCalls) == f.failPendingOnCall {
				http.Error(w, "simulated failure", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"pa-new"}`))
		case strings.HasSuffix(r.URL.Path, "/commitment"):
			if isGetDedup {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			f.commitmentCalls = append(f.commitmentCalls, body)
			if f.failCommitmentOnCall > 0 && len(f.commitmentCalls) == f.failCommitmentOnCall {
				http.Error(w, "simulated failure", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"com-new"}`))
		default:
			http.NotFound(w, r)
		}
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

// --- happy path across all three arms ---

func TestExecute_HappyPath_AllThreeArmsWritten(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	// Note: identity_link PAs are deliberately skipped by the action
	// (replaced by the user-declared identities flow), so the happy-path
	// PA arm uses a non-skipped type. The skip behaviour itself is
	// covered by TestExecute_IdentityLinkPendingAction_Skipped below.
	extraction := `{
		"memories": [
			{"type":"preference","title":"Name","body":"Prefers Andy","confidence":0.95},
			{"type":"fact","title":"Timezone","body":"Europe/London","confidence":0.9}
		],
		"proposed_actions": [
			{"type":"confirm_intent","evidence":"User asked to schedule a meeting","confidence":0.9,"payload":{"intent":"schedule"}}
		],
		"commitments": [
			{"kind":"followup","description":"Follow up tomorrow","trigger_type":"time_elapsed","evidence":"I'll get back to you","confidence":0.85,"made_by":"assistant"}
		]
	}`

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
		strInput("conversation_id", "conv-xyz"),
		strInput("source_message_id", "msg-42"),
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(out["memories_written"]).To(Equal(2))
	Expect(out["memories_flagged"]).To(Equal(0))
	Expect(out["memories_discarded"]).To(Equal(0))
	Expect(out["pending_actions_written"]).To(Equal(1))
	Expect(out["commitments_written"]).To(Equal(1))

	// Preference memory should be auto-pinned regardless of the pinned
	// field on the input; fact memory should not be.
	Expect(api.memoryCalls).To(HaveLen(2))
	Expect(api.memoryCalls[0]["memory_type"]).To(Equal("preference"))
	Expect(api.memoryCalls[0]["pinned"]).To(BeTrue())
	Expect(api.memoryCalls[1]["memory_type"]).To(Equal("fact"))
	Expect(api.memoryCalls[1]["pinned"]).To(BeFalse())

	// source_conversation and source_message propagated to the API call.
	Expect(api.memoryCalls[0]["source_conversation"]).To(Equal("conv-xyz"))
	Expect(api.memoryCalls[0]["source_message"]).To(Equal("msg-42"))
	Expect(api.memoryCalls[0]["agent_user_id"]).To(Equal("user-abc"))

	// Pending action carries the evidence verbatim (the plan is
	// explicit that this must not be paraphrased).
	Expect(api.pendingCalls).To(HaveLen(1))
	Expect(api.pendingCalls[0]["evidence"]).To(Equal("User asked to schedule a meeting"))
	Expect(api.pendingCalls[0]["type"]).To(Equal("confirm_intent"))

	// Commitment defaults made_by to 'assistant' when omitted, but in
	// this test it was provided explicitly.
	Expect(api.commitmentCalls[0]["made_by"]).To(Equal("assistant"))
	Expect(api.commitmentCalls[0]["kind"]).To(Equal("followup"))
}

// Regression guard: identity_link pending actions are intentionally
// skipped because the user-declared identities flow replaced AI-initiated
// linking. If this behaviour ever silently changes, this test should
// fail and force a deliberate decision.
func TestExecute_IdentityLinkPendingAction_Skipped(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	extraction := `{
		"proposed_actions": [
			{"type":"identity_link","evidence":"I'm @andyesser","confidence":0.9,"payload":{"channel":"slack"}}
		]
	}`

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["pending_actions_written"]).To(Equal(0))
	Expect(api.pendingCalls).To(BeEmpty())
}

// --- confidence threshold logic ---

func TestExecute_ConfidenceThresholds_StoreFlagDiscard(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	extraction := `{
		"memories": [
			{"type":"fact","title":"high","body":"high confidence","confidence":0.95},
			{"type":"fact","title":"mid","body":"mid confidence","confidence":0.65},
			{"type":"fact","title":"low","body":"low confidence","confidence":0.3}
		]
	}`

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())

	// >=0.8 → written, 0.5–0.8 → flagged, <0.5 → discarded silently.
	Expect(out["memories_written"]).To(Equal(1))
	Expect(out["memories_flagged"]).To(Equal(1))
	Expect(out["memories_discarded"]).To(Equal(1))

	// Only two API calls — the discarded memory was never sent.
	Expect(api.memoryCalls).To(HaveLen(2))
}

func TestExecute_ConfidenceAtExactThreshold_StoreBoundary(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	// Confidence exactly at 0.8 should be stored (the comparison is >=).
	extraction := `{
		"memories": [
			{"type":"fact","title":"t","body":"b","confidence":0.8}
		]
	}`

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["memories_written"]).To(Equal(1))
	Expect(out["memories_flagged"]).To(Equal(0))
}

// --- markdown code fence stripping ---

func TestExecute_StripsMarkdownCodeFence(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	// AI models love wrapping JSON in ```json … ```. The action must
	// handle this transparently because flow authors shouldn't have to
	// shim it themselves.
	extraction := "```json\n" +
		`{"memories":[{"type":"fact","title":"t","body":"b","confidence":0.9}]}` +
		"\n```"

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["memories_written"]).To(Equal(1))
}

func TestExecute_StripsCodeFenceWithoutLanguageTag(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	extraction := "```\n" +
		`{"memories":[{"type":"fact","title":"t","body":"b","confidence":0.9}]}` +
		"\n```"

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
}

// --- partial failure: one memory write fails ---

func TestExecute_PartialFailure_RestStillWritten(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()
	api.failMemoryOnCall = 2 // second memory write 500s

	extraction := `{
		"memories": [
			{"type":"fact","title":"first","body":"a","confidence":0.9},
			{"type":"fact","title":"second","body":"b","confidence":0.9},
			{"type":"fact","title":"third","body":"c","confidence":0.9}
		]
	}`

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	// Partial failure must NOT fail the whole action — the flow
	// continues, and the failed record shows up in the errors array.
	Expect(err).NotTo(HaveOccurred())

	// Two memories written (first and third), one in the errors array.
	Expect(out["memories_written"]).To(Equal(2))
	errs, _ := out["errors"].([]string)
	Expect(errs).To(HaveLen(1))
	Expect(errs[0]).To(ContainSubstring("memory[1]"))
	Expect(errs[0]).To(ContainSubstring("500"))
}

// --- missing agent_user_id: global scope for memories, skip pending actions ---

func TestExecute_NoAgentUserID_MemoriesGoGlobal_PendingActionsSkipped(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	extraction := `{
		"memories": [
			{"type":"fact","title":"t","body":"b","confidence":0.9}
		],
		"proposed_actions": [
			{"type":"identity_link","evidence":"ev","confidence":0.9}
		]
	}`

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		// no agent_user_id
	})
	Expect(err).NotTo(HaveOccurred())

	// Memory written with scope='global' since there's no user to attach it to.
	Expect(out["memories_written"]).To(Equal(1))
	Expect(api.memoryCalls).To(HaveLen(1))
	Expect(api.memoryCalls[0]["scope"]).To(Equal("global"))
	_, hasUser := api.memoryCalls[0]["agent_user_id"]
	Expect(hasUser).To(BeFalse())

	// Pending actions require a user_id at the schema level, so they're
	// skipped entirely. Zero API calls on that endpoint, zero errors
	// (missing upstream input is not a per-record failure).
	Expect(out["pending_actions_written"]).To(Equal(0))
	Expect(api.pendingCalls).To(HaveLen(0))
}

// --- malformed JSON ---

func TestExecute_MalformedJSON_HardError(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", "not valid json at all"),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("failed to parse extraction JSON"))
}

func TestExecute_EmptyExtraction_ZeroCounts(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	// Empty but valid extraction — model found nothing memorable.
	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", `{"memories":[],"proposed_actions":[],"commitments":[]}`),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["memories_written"]).To(Equal(0))
	Expect(out["pending_actions_written"]).To(Equal(0))
	Expect(out["commitments_written"]).To(Equal(0))
	Expect(api.memoryCalls).To(HaveLen(0))
}

// --- input validation ---

func TestExecute_MissingAgentID_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := flowWithContext("http://example.invalid")
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("extraction_json", `{}`),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("agent_id"))
}

func TestExecute_MissingExtractionJSON_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := flowWithContext("http://example.invalid")
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("extraction_json"))
}

func TestExecute_MissingContext_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{} // no context
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", `{}`),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("API URL"))
}

// --- feedback type also auto-pinned ---

func TestExecute_FeedbackTypeAutoPinned(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	extraction := `{
		"memories": [
			{"type":"feedback","title":"Terse","body":"Prefers terse replies","confidence":0.9}
		]
	}`

	flow := flowWithContext(api.server.URL)
	_, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(api.memoryCalls[0]["pinned"]).To(BeTrue(), "feedback type must be auto-pinned like preference")
}

// --- resolveDueIn (Phase 3 due-at resolution) ---

func TestResolveDueIn_SimpleMinutes(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("30 minutes")
	Expect(result).NotTo(BeEmpty())
	// Parse the result and check it's roughly 30 minutes from now.
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Minutes()).To(BeNumerically("~", 30, 1))
}

func TestResolveDueIn_Hours(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("2 hours")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Hours()).To(BeNumerically("~", 2, 0.1))
}

func TestResolveDueIn_SingularUnit(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("1 hour")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Hours()).To(BeNumerically("~", 1, 0.1))
}

func TestResolveDueIn_GoDurationFormat(t *testing.T) {
	RegisterTestingT(t)
	// Go-style "30m" should also work.
	result := resolveDueIn("30m")
	Expect(result).NotTo(BeEmpty())
}

func TestResolveDueIn_UnparseableReturnsEmpty(t *testing.T) {
	RegisterTestingT(t)
	Expect(resolveDueIn("soon")).To(BeEmpty())
	Expect(resolveDueIn("")).To(BeEmpty())
	Expect(resolveDueIn("whenever you get a chance")).To(BeEmpty())
	Expect(resolveDueIn("asap")).To(BeEmpty())
}

func TestResolveDueIn_Days(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("1 day")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Hours()).To(BeNumerically("~", 24, 1))
}

// --- Phase 3 enhanced patterns ---

func TestResolveDueIn_Tomorrow(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("tomorrow")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	// Should be roughly 24 hours from now (same time tomorrow).
	diff := time.Until(parsed)
	Expect(diff.Hours()).To(BeNumerically("~", 24, 1))
}

func TestResolveDueIn_TomorrowAt9am(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("tomorrow at 9am")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	// Should be tomorrow, and the hour should be 9 in local time.
	tomorrow := time.Now().AddDate(0, 0, 1)
	Expect(parsed.Year()).To(Equal(tomorrow.Year()))
	Expect(parsed.YearDay()).To(Equal(tomorrow.YearDay()))
	// Convert back to local for the hour check since we format as UTC.
	local := parsed.In(time.Now().Location())
	Expect(local.Hour()).To(Equal(9))
}

func TestResolveDueIn_TomorrowAt1430(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("tomorrow at 14:30")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	local := parsed.In(time.Now().Location())
	Expect(local.Hour()).To(Equal(14))
	Expect(local.Minute()).To(Equal(30))
}

func TestResolveDueIn_NextMonday(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("next monday")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	local := parsed.In(time.Now().Location())
	Expect(local.Weekday()).To(Equal(time.Monday))
	// Default time for bare weekday is 9am.
	Expect(local.Hour()).To(Equal(9))
	// Must be in the future (1-7 days out).
	Expect(parsed.After(time.Now())).To(BeTrue())
}

func TestResolveDueIn_NextTuesdayAt10am(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("next tuesday at 10am")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	local := parsed.In(time.Now().Location())
	Expect(local.Weekday()).To(Equal(time.Tuesday))
	Expect(local.Hour()).To(Equal(10))
}

func TestResolveDueIn_InAnHour(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("in an hour")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Minutes()).To(BeNumerically("~", 60, 2))
}

func TestResolveDueIn_InADay(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("in a day")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Hours()).To(BeNumerically("~", 24, 1))
}

func TestResolveDueIn_In30Minutes(t *testing.T) {
	RegisterTestingT(t)
	// "in 30 minutes" — the "in" prefix + numeric pattern.
	result := resolveDueIn("in 30 minutes")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Minutes()).To(BeNumerically("~", 30, 1))
}

func TestResolveDueIn_Weeks(t *testing.T) {
	RegisterTestingT(t)
	result := resolveDueIn("2 weeks")
	Expect(result).NotTo(BeEmpty())
	parsed, err := time.Parse(time.RFC3339, result)
	Expect(err).NotTo(HaveOccurred())
	diff := time.Until(parsed)
	Expect(diff.Hours()).To(BeNumerically("~", 14*24, 1))
}

func TestExecute_CommitmentWithDueIn_ResolvedToDueAt(t *testing.T) {
	RegisterTestingT(t)

	api := newFakeAPI()
	defer api.close()

	extraction := `{
		"memories": [],
		"proposed_actions": [],
		"commitments": [
			{"kind":"followup","description":"Follow up","trigger_type":"time_elapsed","due_in":"30 minutes","evidence":"I'll get back to you in 30 minutes","confidence":0.9,"made_by":"assistant"}
		]
	}`

	flow := flowWithContext(api.server.URL)
	out, err := Execute(flow, nil, []*core.Connection{
		strInput("agent_id", "agent-1"),
		strInput("extraction_json", extraction),
		strInput("agent_user_id", "user-abc"),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out["commitments_written"]).To(Equal(1))

	// The commitment API call should have a due_at field (resolved from due_in).
	Expect(api.commitmentCalls).To(HaveLen(1))
	dueAt, hasDueAt := api.commitmentCalls[0]["due_at"]
	Expect(hasDueAt).To(BeTrue(), "due_in should have been resolved to due_at")
	Expect(dueAt).NotTo(BeEmpty())
}
