package create

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

func TestExecute_BuildsTranslateBody(t *testing.T) {
	RegisterTestingT(t)

	var gotBody map[string]interface{}
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"data":{"video_translate_id":"tr_1","status":"pending"}}`))
	}))
	defer srv.Close()
	old := heygen.BaseURL
	heygen.BaseURL = srv.URL
	defer func() { heygen.BaseURL = old }()

	inputs := []*core.Connection{
		str("api_key", "k"),
		str("video_url", "https://cdn/in.mp4"),
		str("output_language", "Spanish"),
		str("title", "Dubbed"),
		{Name: "translate_audio_only", Type: core.ConnectionTypeBoolean, Value: true},
		{Name: "speaker_num", Type: core.ConnectionTypeInteger, Value: int64(2)},
	}
	out, err := Execute(nil, nil, inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["video_translate_id"]).To(Equal("tr_1"))
	Expect(out["status"]).To(Equal("pending"))

	Expect(gotPath).To(Equal("/v2/video_translate"))
	Expect(gotBody["video_url"]).To(Equal("https://cdn/in.mp4"))
	Expect(gotBody["output_language"]).To(Equal("Spanish"))
	Expect(gotBody["title"]).To(Equal("Dubbed"))
	Expect(gotBody["translate_audio_only"]).To(Equal(true))
	Expect(gotBody["speaker_num"]).To(BeNumerically("==", 2))
}

func TestExecute_RequiresVideoAndLanguage(t *testing.T) {
	RegisterTestingT(t)

	out, _ := Execute(nil, nil, []*core.Connection{str("api_key", "k"), str("output_language", "Spanish")})
	Expect(out["success"]).To(BeFalse())

	out, _ = Execute(nil, nil, []*core.Connection{str("api_key", "k"), str("video_url", "https://x/in.mp4")})
	Expect(out["success"]).To(BeFalse())
}
