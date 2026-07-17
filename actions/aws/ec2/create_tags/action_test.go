package aws_ec2_create_tags

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/gomega"
)

func TestParseTags(t *testing.T) {
	RegisterTestingT(t)

	tags := parseTags("Name=web , Env=prod ,,Team=Platform=x")
	Expect(tags).To(HaveLen(3))

	got := map[string]string{}
	for _, tg := range tags {
		got[aws.ToString(tg.Key)] = aws.ToString(tg.Value)
	}
	Expect(got["Name"]).To(Equal("web"))
	Expect(got["Env"]).To(Equal("prod"))
	// strings.Cut splits on the FIRST '=', so a value may itself contain '='.
	Expect(got["Team"]).To(Equal("Platform=x"))

	// Entries without a valid Key=Value (empty key, blank, or no '=') are skipped.
	Expect(parseTags("=novalue,  ,justkey")).To(HaveLen(0))
}
