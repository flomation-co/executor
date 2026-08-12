package generate_from_image

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
	. "github.com/onsi/gomega"
)

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestExecute_BuildsImageBody(t *testing.T) {
	RegisterTestingT(t)

	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"video_id":"vid_1","status":"waiting"}}`))
	}))
	defer srv.Close()
	old := heygen.BaseURL
	heygen.BaseURL = srv.URL
	defer func() { heygen.BaseURL = old }()

	inputs := []*core.Connection{
		str("api_key", "hg_key"),
		str("talking_photo_id", "tp_123"),
		str("script", "Hello there"),
		str("voice_id", "voice_9"),
		str("aspect_ratio", "9:16"),
	}
	out, err := Execute(nil, nil, inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["video_id"]).To(Equal("vid_1"))

	// Body carries the talking-photo character and the TTS script.
	Expect(gotBody["type"]).To(Equal("image"))
	Expect(gotBody["talking_photo_id"]).To(Equal("tp_123"))
	Expect(gotBody["script"]).To(Equal("Hello there"))
	Expect(gotBody["voice_id"]).To(Equal("voice_9"))
	// Portrait defaults fit=cover (shared helper).
	Expect(gotBody["fit"]).To(Equal("cover"))
}

func TestExecute_PhotoURLFallback(t *testing.T) {
	RegisterTestingT(t)

	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"data":{"video_id":"v2","status":"waiting"}}`))
	}))
	defer srv.Close()
	old := heygen.BaseURL
	heygen.BaseURL = srv.URL
	defer func() { heygen.BaseURL = old }()

	inputs := []*core.Connection{
		str("api_key", "hg_key"),
		str("photo_url", "https://cdn/face.png"),
		str("audio_url", "https://cdn/vo.mp3"),
	}
	out, err := Execute(nil, nil, inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(gotBody["photo_url"]).To(Equal("https://cdn/face.png"))
	Expect(gotBody["audio_url"]).To(Equal("https://cdn/vo.mp3"))
	Expect(gotBody).ToNot(HaveKey("talking_photo_id"))
}

func TestExecute_ValidationErrors(t *testing.T) {
	RegisterTestingT(t)

	// No photo source.
	out, err := Execute(nil, nil, []*core.Connection{str("api_key", "k"), str("script", "hi"), str("voice_id", "v")})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())

	// Photo but nothing to say.
	out, err = Execute(nil, nil, []*core.Connection{str("api_key", "k"), str("talking_photo_id", "tp")})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
}
