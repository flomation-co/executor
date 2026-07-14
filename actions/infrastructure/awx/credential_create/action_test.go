package infrastructure_awx_credential_create

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	. "github.com/onsi/gomega"
)

// machineType is AWX's built-in Machine credential type (id 1): username is
// optional, password and ssh_key_data are secret, and `required` lists username.
const machineType = `{
  "id": 1, "name": "Machine", "kind": "ssh",
  "inputs": {
    "fields": [
      {"id":"username","label":"Username","type":"string"},
      {"id":"password","label":"Password","type":"string","secret":true},
      {"id":"ssh_key_data","label":"SSH Private Key","type":"string","secret":true,"multiline":true}
    ],
    "required": ["username","password"]
  }
}`

func awxServer(h http.HandlerFunc) *httptest.Server {
	awx.ResetAPIRootCacheForTest()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`))
		case "/api/v2/me/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin"}]}`))
		default:
			h(w, r)
		}
	}))
}

func inputs(url string, extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: url},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "token"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
	return append(base, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func object(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: value}
}

// typeOnlyServer answers the credential-type lookup and 404s everything else, so
// a test that expects a client-side refusal fails loudly if the POST is made.
func typeOnlyServer(t *testing.T) *httptest.Server {
	return awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/credential_types/1/" {
			_, _ = w.Write([]byte(machineType))
			return
		}
		t.Errorf("posted the credential despite an invalid Credential Fields: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
}

func TestCreateCredentialPostsTheOwnershipAndInputs(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/credential_types/1/":
			_, _ = w.Write([]byte(machineType))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/credentials/":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":9,"name":"Deploy key","credential_type":1,"organization":1,"inputs":{"username":"deploy","password":"$encrypted$"}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("name", "Deploy key"),
		str("credential_type_id", "1"),
		str("organization_id", "1"),
		object("inputs", `{"username":"deploy","password":"hunter2"}`),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("9"))

	// AWX wants true JSON integers for the FKs, and exactly one owner.
	Expect(body["name"]).To(Equal("Deploy key"))
	Expect(body["credential_type"]).To(BeEquivalentTo(1))
	Expect(body["organization"]).To(BeEquivalentTo(1))
	Expect(body).ToNot(HaveKey("user"))
	Expect(body).ToNot(HaveKey("team"))
	Expect(body["inputs"]).To(Equal(map[string]interface{}{"username": "deploy", "password": "hunter2"}))
}

// ★ THE TRAP. AWX does NOT enforce the credential type's inputs.required at
// create time: it saves the credential and the playbook blows up hours later. We
// refuse up-front, naming the field.
func TestCreateCredentialRefusesAMissingRequiredField(t *testing.T) {
	RegisterTestingT(t)

	srv := typeOnlyServer(t)
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("name", "Deploy key"),
		str("credential_type_id", "1"),
		str("organization_id", "1"),
		object("inputs", `{"username":"deploy"}`), // no password
	))

	Expect(err).To(BeNil()) // SOFT
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("password is required by this credential type"))
	Expect(out["error"]).To(ContainSubstring("only fail later"))
}

// "$encrypted$" is AWX's "keep the existing value" keyword — meaningless on a
// create, and exactly what someone gets if they pipe Get Credential's output
// straight back in.
func TestCreateCredentialRefusesTheEncryptedSentinel(t *testing.T) {
	RegisterTestingT(t)

	srv := typeOnlyServer(t)
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("name", "Copy of Demo Credential"),
		str("credential_type_id", "1"),
		str("organization_id", "1"),
		object("inputs", `{"username":"deploy","password":"$encrypted$"}`),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("$encrypted$"))
	Expect(out["error"]).To(ContainSubstring("password"))
	Expect(out["error"]).To(ContainSubstring("not a value"))
}

func TestCreateCredentialRefusesAnUnknownField(t *testing.T) {
	RegisterTestingT(t)

	srv := typeOnlyServer(t)
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("name", "Deploy key"),
		str("credential_type_id", "1"),
		str("organization_id", "1"),
		object("inputs", `{"username":"deploy","password":"hunter2","secret_key":"nope"}`),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("secret_key is not a field of this credential type"))
	Expect(out["error"]).To(ContainSubstring("ssh_key_data")) // the valid names are listed
}

func TestCreateCredentialRequiresTheFields(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, inputs("https://awx.example.com",
		str("name", "Deploy key"),
		str("credential_type_id", "1"),
		str("organization_id", "1"),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Credential Fields is required"))
}
