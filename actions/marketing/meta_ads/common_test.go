package meta_ads_common

import (
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func ins(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

// Ads Manager shows a bare account number while the API needs the act_ prefix,
// so both must work. Getting this wrong surfaces as "Unsupported get request",
// which reads as a permissions problem and sends you looking in the wrong place.
func TestAccountPath_AcceptsEitherForm(t *testing.T) {
	RegisterTestingT(t)

	Expect(AccountPath("1234567890")).To(Equal("/act_1234567890"))
	Expect(AccountPath("act_1234567890")).To(Equal("/act_1234567890"))
	Expect(AccountPath("  act_1234567890  ")).To(Equal("/act_1234567890"))
	Expect(AccountPath("")).To(Equal(""))
	// Must not double-prefix.
	Expect(AccountPath(AccountPath("1234567890")[1:])).To(Equal("/act_1234567890"))
}

// Graph takes structured values as JSON-encoded STRINGS inside a form body, and
// numbers must not arrive in scientific notation — a Meta object id rendered as
// 1.234567890123e+12 is rejected.
func TestMergeJSONFields(t *testing.T) {
	RegisterTestingT(t)

	p := url.Values{}
	err := MergeJSONFields(p, "fields", ins([2]string{"fields",
		`{"bid_strategy":"LOWEST_COST_WITHOUT_CAP","page_id":1234567890123,"is_test":true,"rate":1.5,"targeting":{"countries":["GB"]},"skipme":null}`}))

	Expect(err).To(BeNil())
	Expect(p.Get("bid_strategy")).To(Equal("LOWEST_COST_WITHOUT_CAP"))
	Expect(p.Get("page_id")).To(Equal("1234567890123"))
	Expect(p.Get("is_test")).To(Equal("true"))
	Expect(p.Get("rate")).To(Equal("1.5"))
	Expect(p.Get("targeting")).To(Equal(`{"countries":["GB"]}`))
	Expect(p).ToNot(HaveKey("skipme"))
}

func TestMergeJSONFields_RejectsNonObject(t *testing.T) {
	RegisterTestingT(t)

	Expect(MergeJSONFields(url.Values{}, "fields", ins([2]string{"fields", "not json"}))).ToNot(BeNil())
	Expect(MergeJSONFields(url.Values{}, "fields", ins([2]string{"fields", `["an","array"]`}))).ToNot(BeNil())
	// Absent is fine.
	Expect(MergeJSONFields(url.Values{}, "fields", ins())).To(BeNil())
}

// An override must beat the curated inputs, since that is the only useful
// precedence for an escape hatch.
func TestMergeJSONFields_OverridesEarlierValues(t *testing.T) {
	RegisterTestingT(t)

	p := url.Values{"status": {"PAUSED"}}
	Expect(MergeJSONFields(p, "fields", ins([2]string{"fields", `{"status":"ACTIVE"}`}))).To(BeNil())
	Expect(p.Get("status")).To(Equal("ACTIVE"))
}

func TestSetJSONParam_ValidatesBeforeSending(t *testing.T) {
	RegisterTestingT(t)

	p := url.Values{}
	Expect(SetJSONParam(p, "targeting", "targeting", ins([2]string{"targeting", `{"geo_locations":{"countries":["GB"]}}`}))).To(BeNil())
	Expect(p.Get("targeting")).To(Equal(`{"geo_locations":{"countries":["GB"]}}`))

	// Invalid JSON must be caught here rather than becoming a generic
	// "Invalid parameter" from Graph with no indication of which field.
	Expect(SetJSONParam(url.Values{}, "targeting", "targeting", ins([2]string{"targeting", `{broken`}))).ToNot(BeNil())
}

// The absence of a `next` URL is the end-of-list signal — cursors.after can
// still be populated on the final page, so keying off it alone would loop
// forever re-fetching the last page.
func TestNextCursor(t *testing.T) {
	RegisterTestingT(t)

	more := map[string]interface{}{"paging": map[string]interface{}{
		"next":    "https://graph.facebook.com/v25.0/act_1/campaigns?after=ABC",
		"cursors": map[string]interface{}{"after": "ABC"},
	}}
	Expect(NextCursor(more)).To(Equal("ABC"))

	last := map[string]interface{}{"paging": map[string]interface{}{
		"cursors": map[string]interface{}{"after": "ABC"},
	}}
	Expect(NextCursor(last)).To(Equal(""))

	Expect(NextCursor(map[string]interface{}{})).To(Equal(""))
}

func TestData(t *testing.T) {
	RegisterTestingT(t)

	got := Data(map[string]interface{}{"data": []interface{}{
		map[string]interface{}{"id": "1"},
		"not an object",
		map[string]interface{}{"id": "2"},
	}})
	Expect(got).To(HaveLen(2))
	Expect(got[0]["id"]).To(Equal("1"))

	Expect(Data(map[string]interface{}{})).To(BeNil())
}

// An unresolved ${...} must be refused rather than sent as a literal token —
// Meta answers that with an opaque code 190, which reads as "your token
// expired" rather than "your secret reference is wrong".
func TestGetAuth_RefusesUnresolvedSecret(t *testing.T) {
	RegisterTestingT(t)

	_, _, err := GetAuth(ins([2]string{"access_token", "${secrets.MetaAdsToken}"}))
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("did not resolve"))

	_, _, err = GetAuth(ins())
	Expect(err).ToNot(BeNil())

	token, secret, err := GetAuth(ins([2]string{"access_token", "EAAG..."}, [2]string{"app_secret", "s3cr3t"}))
	Expect(err).To(BeNil())
	Expect(token).To(Equal("EAAG..."))
	Expect(secret).To(Equal("s3cr3t"))
}

// Error mapping is the difference between a flow author retrying (rate limit)
// and reconfiguring (permissions), so the families must stay distinguishable.
func TestAPIError_ClassifiesTheCommonFailures(t *testing.T) {
	RegisterTestingT(t)

	body := func(code, sub int64, msg string) map[string]interface{} {
		e := map[string]interface{}{"code": float64(code), "message": msg}
		if sub != 0 {
			e["error_subcode"] = float64(sub)
		}
		return map[string]interface{}{"error": e}
	}

	Expect(apiError(400, body(17, 0, "User request limit reached"), nil).Error()).
		To(ContainSubstring("rate limit"))
	Expect(apiError(400, body(17, 0, "User request limit reached"), nil).Error()).
		To(ContainSubstring("Limited Access"))

	Expect(apiError(400, body(190, 0, "Session expired"), nil).Error()).
		To(ContainSubstring("System User token"))

	Expect(apiError(403, body(200, 1234, "Permissions error"), nil).Error()).
		To(ContainSubstring("ads_management"))

	// error_user_msg is Meta's human-facing text and beats the terse `message`.
	withUser := map[string]interface{}{"error": map[string]interface{}{
		"code": float64(100), "message": "Invalid parameter",
		"error_user_msg": "Your daily budget is below the minimum for this currency",
	}}
	Expect(apiError(400, withUser, nil).Error()).
		To(ContainSubstring("below the minimum"))

	// A non-JSON body must still produce something readable.
	Expect(apiError(500, nil, []byte("upstream exploded")).Error()).
		To(ContainSubstring("upstream exploded"))
}

// Meta routinely answers a malformed request with a sentence that names nothing
// — "The ad creative is invalid" — while separately reporting exactly which
// field it objected to. Dropping that turned a precise answer into a guessing
// game between permissions, an unapproved image and a provisioning delay.
func TestAPIError_SurfacesBlameFieldSpecs(t *testing.T) {
	RegisterTestingT(t)

	body := map[string]interface{}{"error": map[string]interface{}{
		"code": float64(100), "error_subcode": float64(1487015),
		"message":        "Invalid parameter",
		"error_user_msg": "The ad creative is invalid.",
		"error_data": map[string]interface{}{
			"blame_field_specs": []interface{}{
				[]interface{}{"creative", "object_story_spec", "link_data", "image_hash"},
			},
		},
	}}

	err := apiError(400, body, nil)
	Expect(err.Error()).To(ContainSubstring("The ad creative is invalid"))
	Expect(err.Error()).To(ContainSubstring("1487015"))
	// The whole point: the field path must reach the reader.
	Expect(err.Error()).To(ContainSubstring("creative.object_story_spec.link_data.image_hash"))
}

func TestAPIError_BlameFieldSpecs_Shapes(t *testing.T) {
	RegisterTestingT(t)

	// Multiple paths.
	multi := map[string]interface{}{"error": map[string]interface{}{
		"code": float64(100), "message": "Invalid parameter",
		"error_data": map[string]interface{}{"blame_field_specs": []interface{}{
			[]interface{}{"targeting", "geo_locations"},
			[]interface{}{"bid_amount"},
		}},
	}}
	Expect(apiError(400, multi, nil).Error()).To(ContainSubstring("targeting.geo_locations, bid_amount"))

	// A bare string rather than a path array.
	bare := map[string]interface{}{"error": map[string]interface{}{
		"code": float64(100), "message": "Invalid parameter",
		"error_data": map[string]interface{}{"blame_field_specs": []interface{}{"adset_id"}},
	}}
	Expect(apiError(400, bare, nil).Error()).To(ContainSubstring("adset_id"))

	// No error_data at all — must not add a dangling suffix.
	plain := map[string]interface{}{"error": map[string]interface{}{
		"code": float64(100), "message": "Invalid parameter",
	}}
	Expect(apiError(400, plain, nil).Error()).ToNot(ContainSubstring("offending field"))

	// error_data present but empty.
	empty := map[string]interface{}{"error": map[string]interface{}{
		"code": float64(100), "message": "Invalid parameter",
		"error_data": map[string]interface{}{},
	}}
	Expect(apiError(400, empty, nil).Error()).ToNot(ContainSubstring("offending field"))
}
