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

func TestNormalizeEngine(t *testing.T) {
	RegisterTestingT(t)

	// string -> object (the exact failure agents hit via raw_json)
	b := map[string]interface{}{"engine": "avatar_v"}
	NormalizeEngine(b)
	Expect(b["engine"]).To(Equal(map[string]interface{}{"type": "avatar_v"}))

	// already an object -> untouched
	obj := map[string]interface{}{"type": "avatar_iv", "reference_look_id": "look_1"}
	b = map[string]interface{}{"engine": obj}
	NormalizeEngine(b)
	Expect(b["engine"]).To(Equal(obj))

	// empty string / nil -> dropped
	b = map[string]interface{}{"engine": ""}
	NormalizeEngine(b)
	Expect(b).ToNot(HaveKey("engine"))
	b = map[string]interface{}{"engine": nil}
	NormalizeEngine(b)
	Expect(b).ToNot(HaveKey("engine"))
}

func TestNormalizeBackground(t *testing.T) {
	RegisterTestingT(t)

	// hex with # -> colour
	b := map[string]interface{}{"background": "#150e14"}
	NormalizeBackground(b)
	Expect(b["background"]).To(Equal(map[string]interface{}{"type": "color", "value": "#150e14"}))

	// bare hex -> colour with # prepended
	b = map[string]interface{}{"background": "150e14"}
	NormalizeBackground(b)
	Expect(b["background"]).To(Equal(map[string]interface{}{"type": "color", "value": "#150e14"}))

	// http(s) URL -> image
	b = map[string]interface{}{"background": "https://cdn/room.jpg"}
	NormalizeBackground(b)
	Expect(b["background"]).To(Equal(map[string]interface{}{"type": "image", "url": "https://cdn/room.jpg"}))

	// object left untouched; empty dropped
	obj := map[string]interface{}{"type": "image", "asset_id": "a1"}
	b = map[string]interface{}{"background": obj}
	NormalizeBackground(b)
	Expect(b["background"]).To(Equal(obj))
	b = map[string]interface{}{"background": ""}
	NormalizeBackground(b)
	Expect(b).ToNot(HaveKey("background"))
}

func TestDefaultPortraitFit(t *testing.T) {
	RegisterTestingT(t)

	// portrait + no fit -> cover
	b := map[string]interface{}{"aspect_ratio": "9:16"}
	DefaultPortraitFit(b)
	Expect(b["fit"]).To(Equal("cover"))
	b = map[string]interface{}{"aspect_ratio": "4:5"}
	DefaultPortraitFit(b)
	Expect(b["fit"]).To(Equal("cover"))

	// explicit fit is respected
	b = map[string]interface{}{"aspect_ratio": "9:16", "fit": "contain"}
	DefaultPortraitFit(b)
	Expect(b["fit"]).To(Equal("contain"))

	// landscape/square/none -> untouched
	for _, ar := range []string{"16:9", "1:1", "auto", ""} {
		b = map[string]interface{}{"aspect_ratio": ar}
		DefaultPortraitFit(b)
		Expect(b).ToNot(HaveKey("fit"), "ar=%s should not get a default fit", ar)
	}
}
