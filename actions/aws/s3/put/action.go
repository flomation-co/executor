package aws_s3_put

import (
	"bytes"
	"context"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"flomation.app/automate/executor/actions/aws/s3"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Put"
	Description  = "Upload an object to an AWS S3 bucket"
	Website      = "https://www.flomation.co"
	Icon         = "bucket+arrow-up"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "aws_access_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "AWS Access Key",
		Placeholder: "",
		Required:    true,
	},
	core.Connection{
		Name:        "aws_secret_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "AWS Secret Key",
		Placeholder: "",
		Required:    true,
	},
	core.Connection{
		Name:        "aws_region",
		Type:        core.ConnectionTypeString,
		Label:       "Region",
		Placeholder: "eu-west-2",
	},
	core.Connection{
		Name:        "key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Filename",
		Placeholder: "",
		Required:    true,
	},
	core.Connection{
		Name:        "bucket",
		Type:        core.ConnectionTypeString,
		Label:       "Bucket",
		Placeholder: "",
		Required:    true,
	},
	core.Connection{
		Name:        "contents",
		Type:        core.ConnectionTypeString,
		Label:       "Contents",
		Placeholder: "",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	core.Connection{
		Name:        "bucket",
		Type:        core.ConnectionTypeString,
		Label:       "Bucket",
		Placeholder: "",
	},
	core.Connection{
		Name:        "filename",
		Type:        core.ConnectionTypeString,
		Label:       "Filename",
		Placeholder: "",
	},
	core.Connection{
		Name:        "result",
		Type:        core.ConnectionTypeInteger,
		Label:       "Filename",
		Placeholder: "",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	accessKey := core.FindConnection("aws_access_key", inputs)
	secretKey := core.FindConnection("aws_secret_key", inputs)
	filename := core.FindConnection("key", inputs)
	bucket := core.FindConnection("bucket", inputs)
	contents := core.FindConnection("contents", inputs)

	// The object body accepts a flo:file:/flo:blob: reference (e.g. a large media
	// action output) as well as inline text.
	cs := *contents.String()
	bodyBytes := []byte(cs)
	mimeType := ""
	if core.IsFileRef(cs) || core.IsBlobToken(cs) {
		resolved, resolvedMime, rerr := flow.ResolveToBytes(cs)
		if rerr != nil {
			return nil, rerr
		}
		bodyBytes, mimeType = resolved, resolvedMime
	}

	// A blank key, or one ending in "/", means "put it in there under its own
	// name" rather than writing an object literally called "" or "prefix/".
	key := core.UploadDestination(strFrom(filename), cs, mimeType, "upload")

	region := awscommon.InputString("aws_region", inputs)
	if region == "" {
		region = "eu-west-2"
	}
	s, err := s3.GetService(*accessKey.String(), *secretKey.String(), region)
	if err != nil {
		return nil, err
	}

	_, err = s.Client.PutObject(context.Background(), &awsS3.PutObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(*bucket.String()),
		Body:   bytes.NewReader(bodyBytes),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"bucket":   *bucket.String(),
		"filename": key,
		"result":   0,
	}, nil
}

// strFrom reads a connection that may be absent or unset without panicking —
// the key is optional now, so it can legitimately be neither.
func strFrom(c *core.Connection) string {
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
