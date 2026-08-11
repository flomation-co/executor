package heygen_common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
)

func TestClient_GetDecodesDataAndSendsKey(t *testing.T) {
	RegisterTestingT(t)

	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"video_id":"v_1","status":"waiting"}}`))
	}))
	defer srv.Close()

	orig := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = orig }()

	resp, err := NewClient("secret-key").Get(nil, "/v3/videos/v_1", nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(gotKey).To(Equal("secret-key"))
	Expect(gotPath).To(Equal("/v3/videos/v_1"))
	Expect(Str(DataObj(resp), "status")).To(Equal("waiting"))
	Expect(Str(DataObj(resp), "video_id")).To(Equal("v_1"))
}

func TestClient_Non2xxYieldsAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"Too many requests"}}`))
	}))
	defer srv.Close()

	orig := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = orig }()

	_, err := NewClient("k").Get(nil, "/v3/voices", nil)
	Expect(err).To(HaveOccurred())
	ae, ok := err.(*APIError)
	Expect(ok).To(BeTrue())
	Expect(ae.Status).To(Equal(429))
	Expect(ae.Message()).To(Equal("Too many requests (rate_limited)"))
}

func TestAPIError_MessageShapes(t *testing.T) {
	RegisterTestingT(t)

	Expect((&APIError{Status: 400, Body: []byte(`{"message":"bad avatar_id"}`)}).Message()).To(Equal("bad avatar_id"))
	Expect((&APIError{Status: 401, Body: []byte(`{"error":"unauthorized"}`)}).Message()).To(Equal("unauthorized"))
	Expect((&APIError{Status: 500, Body: []byte(`oops`)}).Message()).To(ContainSubstring("HTTP 500"))
}

func TestExtractList_ToleratesShapeVariance(t *testing.T) {
	RegisterTestingT(t)

	// data is an object holding a named array.
	byKey := map[string]interface{}{"data": map[string]interface{}{
		"voices": []interface{}{map[string]interface{}{"voice_id": "a"}, map[string]interface{}{"voice_id": "b"}},
	}}
	Expect(ExtractList(byKey, "voices", "list")).To(HaveLen(2))

	// data is an array directly.
	asArray := map[string]interface{}{"data": []interface{}{map[string]interface{}{"id": "x"}}}
	Expect(ExtractList(asArray, "voices")).To(HaveLen(1))

	// nothing matches -> empty, never nil.
	Expect(ExtractList(map[string]interface{}{"data": map[string]interface{}{}}, "voices")).To(BeEmpty())
}
