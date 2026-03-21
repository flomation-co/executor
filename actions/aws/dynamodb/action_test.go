package aws_dynamodb

import (
	"testing"

	core "flomation.app/automate/executor"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	. "github.com/onsi/gomega"
)

func makeInputs(vals map[string]interface{}) []*core.Connection {
	var inputs []*core.Connection
	for _, inp := range Inputs {
		c := &core.Connection{
			Name: inp.Name,
			Type: inp.Type,
		}
		if v, ok := vals[inp.Name]; ok {
			c.Value = v
		}
		inputs = append(inputs, c)
	}
	return inputs
}

func TestMetadata(t *testing.T) {
	RegisterTestingT(t)

	Expect(Name).To(Equal("DynamoDB Query"))
	Expect(Type).To(Equal(core.ActionTypeAction))
	Expect(Author).To(Equal("Andy Esser"))
	Expect(Icon).To(Equal("table"))
}

func TestInputsConfiguration(t *testing.T) {
	RegisterTestingT(t)

	Expect(len(Inputs)).To(Equal(8))

	accessKeyInput := Inputs[0]
	Expect(accessKeyInput.Name).To(Equal("access_key"))
	Expect(accessKeyInput.Required).To(BeTrue())

	secretKeyInput := Inputs[1]
	Expect(secretKeyInput.Name).To(Equal("secret_key"))
	Expect(secretKeyInput.Required).To(BeTrue())

	regionInput := Inputs[2]
	Expect(regionInput.Name).To(Equal("region"))
	Expect(regionInput.Required).To(BeTrue())

	tableInput := Inputs[3]
	Expect(tableInput.Name).To(Equal("table_name"))
	Expect(tableInput.Required).To(BeTrue())

	opInput := Inputs[4]
	Expect(opInput.Name).To(Equal("operation"))
	Expect(opInput.Required).To(BeTrue())
	Expect(len(opInput.Options)).To(Equal(5))
}

func TestOutputsConfiguration(t *testing.T) {
	RegisterTestingT(t)

	Expect(len(Outputs)).To(Equal(2))
	Expect(Outputs[0].Name).To(Equal("results"))
	Expect(Outputs[0].Type).To(Equal(core.ConnectionTypeObject))
	Expect(Outputs[1].Name).To(Equal("count"))
	Expect(Outputs[1].Type).To(Equal(core.ConnectionTypeInteger))
}

func TestParseDynamoDBKey(t *testing.T) {
	RegisterTestingT(t)

	key, err := parseDynamoDBKey(`{"pk": {"S": "user123"}, "sk": {"N": "42"}}`)
	Expect(err).To(BeNil())
	Expect(key).To(HaveLen(2))

	pkVal, ok := key["pk"].(*types.AttributeValueMemberS)
	Expect(ok).To(BeTrue())
	Expect(pkVal.Value).To(Equal("user123"))

	skVal, ok := key["sk"].(*types.AttributeValueMemberN)
	Expect(ok).To(BeTrue())
	Expect(skVal.Value).To(Equal("42"))
}

func TestParseDynamoDBKeyWithBool(t *testing.T) {
	RegisterTestingT(t)

	key, err := parseDynamoDBKey(`{"active": {"BOOL": "true"}}`)
	Expect(err).To(BeNil())

	boolVal, ok := key["active"].(*types.AttributeValueMemberBOOL)
	Expect(ok).To(BeTrue())
	Expect(boolVal.Value).To(BeTrue())
}

func TestParseDynamoDBKeyInvalid(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseDynamoDBKey(`{invalid}`)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("invalid key JSON"))
}

func TestUnmarshalItem(t *testing.T) {
	RegisterTestingT(t)

	item := map[string]types.AttributeValue{
		"name":   &types.AttributeValueMemberS{Value: "test"},
		"count":  &types.AttributeValueMemberN{Value: "42"},
		"active": &types.AttributeValueMemberBOOL{Value: true},
	}

	result, err := unmarshalItem(item)
	Expect(err).To(BeNil())
	Expect(result["name"]).To(Equal("test"))
	Expect(result["active"]).To(BeTrue())
}

func TestUnmarshalItems(t *testing.T) {
	RegisterTestingT(t)

	items := []map[string]types.AttributeValue{
		{"id": &types.AttributeValueMemberS{Value: "a"}},
		{"id": &types.AttributeValueMemberS{Value: "b"}},
	}

	results, err := unmarshalItems(items)
	Expect(err).To(BeNil())
	Expect(results).To(HaveLen(2))
	Expect(results[0]["id"]).To(Equal("a"))
	Expect(results[1]["id"]).To(Equal("b"))
}

func TestUnsupportedOperation(t *testing.T) {
	RegisterTestingT(t)

	node := &core.Node{ID: "ddb-1", Type: "aws/dynamodb", Data: &core.NodeData{ID: "ddb-1"}}
	flow := &core.Flow{Nodes: []*core.Node{node}}

	inputs := makeInputs(map[string]interface{}{
		"access_key": "AKIATEST",
		"secret_key": "secret",
		"region":     "eu-west-1",
		"table_name": "test-table",
		"operation":  "invalid_op",
		"key":        "{}",
	})

	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("unsupported operation"))
}

func TestPutItemRequiresData(t *testing.T) {
	RegisterTestingT(t)

	node := &core.Node{ID: "ddb-1", Type: "aws/dynamodb", Data: &core.NodeData{ID: "ddb-1"}}
	flow := &core.Flow{Nodes: []*core.Node{node}}

	inputs := makeInputs(map[string]interface{}{
		"access_key": "AKIATEST",
		"secret_key": "secret",
		"region":     "eu-west-1",
		"table_name": "test-table",
		"operation":  "put_item",
		"key":        "{}",
		"data":       "",
	})

	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("data is required"))
}
