package core

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
)

// Identity inputs are a security boundary, not a convenience.
//
// agent_id and agent_user_id are what scope an agent's data to one agent and
// one person. agent_search_conversation returns everything that user has ever
// said to that agent, so a model free to choose agent_user_id is a model that
// can read somebody else's conversation — and an agent processes untrusted
// input, so it only takes a message telling it to search as another user.
//
// This was a real gap rather than a theoretical one. agent_user_id was missing
// from the list entirely, so a live tool call arrived carrying
// agent_user_id="andy" — a display name the model had inferred from the chat.

func toolInput(name string, value interface{}) *Connection {
	return &Connection{Name: name, Type: ConnectionTypeString, Value: value}
}

// schemaFor runs the tool-definition builder and returns the named tool's
// advertised input properties.
func schemaFor(t *testing.T, toolNode *Node) map[string]interface{} {
	t.Helper()

	aiNode := &Node{ID: "ai-1", Data: &NodeData{Label: "ai/anthropic", Config: NodeConfig{ID: "ai-1"}}}
	f := &Flow{context: &ExecutionContext{ChannelType: "slack"}}
	f.injectToolDefinitions(aiNode, []*Node{toolNode}, map[string]Action{})

	var raw string
	for _, inp := range aiNode.Data.Config.Inputs {
		if inp.Name == "tool_definitions" {
			if s, ok := inp.Value.(string); ok {
				raw = s
			}
		}
	}
	if raw == "" {
		t.Fatal("no tool_definitions were injected")
	}

	var tools []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &tools); err != nil {
		t.Fatalf("tool definitions are not valid JSON: %v", err)
	}
	if len(tools) == 0 {
		t.Fatal("no tools in the definitions")
	}
	schema, _ := tools[0]["input_schema"].(map[string]interface{})
	props, _ := schema["properties"].(map[string]interface{})
	return props
}

// The model must never be ASKED for an identity or credential input, even when
// the flow author has left it blank. A blank one used to be published as a
// parameter the model was expected to supply, which is precisely how it came to
// invent a value.
func TestToolSchema_NeverAdvertisesIdentityOrCredentialInputs(t *testing.T) {
	RegisterTestingT(t)

	node := &Node{
		ID: "t1",
		Data: &NodeData{Label: "agent/search_conversation", Config: NodeConfig{
			ID: "t1",
			Inputs: []*Connection{
				toolInput("agent_id", ""),      // blank, as the live flow had it
				toolInput("agent_user_id", ""), // blank — the one that leaked
				toolInput("api_key", ""),
				toolInput("query", ""), // a genuine parameter
			},
		}},
	}

	props := schemaFor(t, node)

	Expect(props).To(HaveKey("query"), "a real parameter must still be offered")
	for _, name := range []string{"agent_id", "agent_user_id", "api_key"} {
		Expect(props).ToNot(HaveKey(name), "%s must never be advertised to the model", name)
	}
}

// Hiding an input from the schema is not the same as refusing it. Models pass
// parameters they were never offered, so the list has to be enforced where
// values are applied as well as where they are advertised.
func TestNonOverridableInputs_CoversIdentityAndCredentials(t *testing.T) {
	RegisterTestingT(t)

	for _, name := range []string{
		"agent_id", "agent_user_id", "channel_type",
		"api_key", "bot_token", "signing_secret", "token",
		"password", "secret", "secret_key", "access_key", "user_token",
	} {
		Expect(nonOverridableInputs[name]).To(BeTrue(), "%s should not be settable by a tool call", name)
	}

	// Ordinary parameters must stay settable, or every tool stops working.
	for _, name := range []string{"query", "limit", "content", "file_path", "url"} {
		Expect(nonOverridableInputs[name]).To(BeFalse(), "%s is a real parameter and must remain settable", name)
	}
}

// The specific value seen on live. A display name is not a user id, and the
// action would have scoped its search by it.
func TestNonOverridableInputs_WouldHaveRefusedTheLiveCase(t *testing.T) {
	RegisterTestingT(t)

	// The model sent agent_user_id="andy" to agent_search_conversation.
	Expect(nonOverridableInputs["agent_user_id"]).To(BeTrue(),
		"this is the exact input that reached a live tool call carrying a guessed value")
}

// A blank identity input must stay blank rather than being filled by the model.
// Failing closed is the point: the action then reports that it needs wiring,
// which is recoverable, instead of silently searching the wrong person's
// conversation, which is not.
func TestToolSchema_BlankIdentityIsHiddenRatherThanRequested(t *testing.T) {
	RegisterTestingT(t)

	node := &Node{
		ID: "t1",
		Data: &NodeData{Label: "agent/search_conversation", Config: NodeConfig{
			ID:     "t1",
			Inputs: []*Connection{toolInput("agent_user_id", ""), toolInput("query", "")},
		}},
	}

	props := schemaFor(t, node)
	Expect(props).ToNot(HaveKey("agent_user_id"))
	Expect(props).To(HaveKey("query"))
}

// A wired ${...} reference must also stay out of the schema — it resolves at
// execution time, and a model's value would overwrite the reference before
// resolution could happen.
func TestToolSchema_ExcludesResolvedReferences(t *testing.T) {
	RegisterTestingT(t)

	node := &Node{
		ID: "t1",
		Data: &NodeData{Label: "agent/search_conversation", Config: NodeConfig{
			ID: "t1",
			Inputs: []*Connection{
				toolInput("agent_user_id", "${flow.agent_user_id}"),
				toolInput("limit", "20"),
				toolInput("query", ""),
			},
		}},
	}

	props := schemaFor(t, node)
	Expect(props).ToNot(HaveKey("agent_user_id"))
	Expect(props).ToNot(HaveKey("limit"), "an author-configured value is not the model's to change")
	Expect(props).To(HaveKey("query"))
}
