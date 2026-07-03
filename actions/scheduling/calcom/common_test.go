package calcom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestGetAuth(t *testing.T) {
	RegisterTestingT(t)
	_, err := GetAuth([]*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("authentication required"))

	token, err := GetAuth([]*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "cal_live_x"},
	})
	Expect(err).To(BeNil())
	Expect(token).To(Equal("cal_live_x"))
}

func TestCheckResponseErrorEnvelope(t *testing.T) {
	RegisterTestingT(t)
	// Matches the live shape observed from api.cal.com.
	err := CheckResponse(&APIResponse{
		StatusCode: 404,
		Body:       []byte(`{"status":"error","error":{"code":"NotFoundException","message":"Event type with id 99999999 not found"}}`),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("NotFoundException: Event type with id 99999999 not found"))

	Expect(CheckResponse(&APIResponse{StatusCode: 204})).To(BeNil())

	err = CheckResponse(&APIResponse{StatusCode: 429})
	Expect(err.Error()).To(ContainSubstring("rate limit"))
}

func TestExecuteAPISendsBearerAndVersion(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer cal_live_x"))
		Expect(r.Header.Get("cal-api-version")).To(Equal("2026-02-25"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"uid":"bk_1"}}`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	obj, err := GetResource("cal_live_x", "/bookings/bk_1", VersionBookings, nil)
	Expect(err).To(BeNil())
	Expect(obj["uid"]).To(Equal("bk_1"))
}

func TestExecuteAPIOmitsVersionWhenBlank(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present := r.Header["Cal-Api-Version"]
		Expect(present).To(BeFalse())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"id":7}}`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	obj, err := GetResource("cal_live_x", "/me", VersionNone, nil)
	Expect(err).To(BeNil())
	Expect(idOf(obj)).To(Equal("7"))
}

// TestListResourcesPaginates walks two take/skip pages, stopping when
// pagination.hasNextPage is false.
func TestListResourcesPaginates(t *testing.T) {
	RegisterTestingT(t)
	var seenSkips []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		skip := r.URL.Query().Get("skip")
		seenSkips = append(seenSkips, skip)
		w.Header().Set("Content-Type", "application/json")
		if skip == "0" {
			_, _ = w.Write([]byte(`{"status":"success","data":[{"id":1},{"id":2}],"pagination":{"hasNextPage":true,"remainingItems":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":[{"id":3}],"pagination":{"hasNextPage":false,"remainingItems":0}}`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	items, next, pages, err := ListResources("cal_live_x", "/bookings", VersionBookings, nil, 2, 0, true)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(3))
	Expect(next).To(Equal(0))
	Expect(pages).To(Equal(2))
	Expect(seenSkips).To(Equal([]string{"0", "2"}))
}

// TestListResourcesSinglePageNoPagination handles endpoints (schedules, teams,
// event-types) that return a bare array with no pagination block.
func TestListResourcesSinglePageNoPagination(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":[{"id":1},{"id":2}]}`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	items, next, pages, err := ListResources("cal_live_x", "/event-types", VersionEventTypes, nil, 0, 0, true)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(2))
	Expect(next).To(Equal(0))
	Expect(pages).To(Equal(1))
}

func TestOptionalStringSliceForms(t *testing.T) {
	RegisterTestingT(t)
	// []interface{} (parsed multi-select)
	got := OptionalStringSlice("triggers", []*core.Connection{
		{Name: "triggers", Type: core.ConnectionTypeMultiSelect, Value: []interface{}{"BOOKING_CREATED", "MEETING_ENDED"}},
	})
	Expect(got).To(Equal([]string{"BOOKING_CREATED", "MEETING_ENDED"}))

	// comma-separated string
	got = OptionalStringSlice("triggers", []*core.Connection{
		{Name: "triggers", Type: core.ConnectionTypeMultiSelect, Value: "BOOKING_CREATED, BOOKING_CANCELLED"},
	})
	Expect(got).To(Equal([]string{"BOOKING_CREATED", "BOOKING_CANCELLED"}))

	// JSON-array string
	got = OptionalStringSlice("triggers", []*core.Connection{
		{Name: "triggers", Type: core.ConnectionTypeMultiSelect, Value: `["A","B"]`},
	})
	Expect(got).To(Equal([]string{"A", "B"}))
}

func TestParseJSONObjectAndArray(t *testing.T) {
	RegisterTestingT(t)
	obj, err := ParseJSONObject("metadata_json", []*core.Connection{
		{Name: "metadata_json", Type: core.ConnectionTypeObject, Value: `{"source":"flomation"}`},
	})
	Expect(err).To(BeNil())
	Expect(obj["source"]).To(Equal("flomation"))

	arr, err := ParseJSONArray("availability_json", []*core.Connection{
		{Name: "availability_json", Type: core.ConnectionTypeObject, Value: `[{"days":["Monday"]}]`},
	})
	Expect(err).To(BeNil())
	Expect(arr).To(HaveLen(1))

	_, err = ParseJSONObject("bad", []*core.Connection{
		{Name: "bad", Type: core.ConnectionTypeObject, Value: `{not json`},
	})
	Expect(err).To(HaveOccurred())
}

func TestResultShapers(t *testing.T) {
	RegisterTestingT(t)
	r := ResourceResult(map[string]interface{}{"uid": "bk_1"}, "done")
	Expect(r["id"]).To(Equal("bk_1"))
	Expect(r["success"]).To(Equal(true))

	// numeric id path
	var raw map[string]interface{}
	_ = json.Unmarshal([]byte(`{"id":42}`), &raw)
	r = ResourceResult(raw, "done")
	Expect(r["id"]).To(Equal("42"))

	l := ListResult([]interface{}{1, 2}, 4, "listed")
	Expect(l["count"]).To(Equal(2))
	Expect(l["next_skip"]).To(Equal(4))
}
