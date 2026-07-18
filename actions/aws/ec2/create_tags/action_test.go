package aws_ec2_create_tags

import (
	"testing"

	core "flomation.app/automate/executor"
	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/gomega"
)

func TestBuildTags(t *testing.T) {
	RegisterTestingT(t)

	// Value shape as the editor stores a key_value_array (JSON string of
	// {key,value} objects). A blank-key row is skipped.
	inputs := []*core.Connection{
		{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Value: `[{"key":"Name","value":"web"},{"key":"Env","value":"prod"},{"key":"","value":"skip"}]`},
	}

	tags := buildTags(inputs)
	Expect(tags).To(HaveLen(2))

	got := map[string]string{}
	for _, tg := range tags {
		got[aws.ToString(tg.Key)] = aws.ToString(tg.Value)
	}
	Expect(got["Name"]).To(Equal("web"))
	Expect(got["Env"]).To(Equal("prod"))
}

func TestBuildTagsEmpty(t *testing.T) {
	RegisterTestingT(t)
	Expect(buildTags(nil)).To(BeEmpty())
}
