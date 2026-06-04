package aws_s3_get

import (
	"context"
	"io"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/aws/s3"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Get Object"
	Description  = "Download an object from an AWS S3 bucket"
	Website      = "https://www.flomation.co"
	Icon         = "bucket+arrow-down"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "aws_access_key",
		Type:        core.ConnectionTypeString,
		Label:       "AWS Access Key",
		Placeholder: "",
	},
	core.Connection{
		Name:        "aws_secret_key",
		Type:        core.ConnectionTypeString,
		Label:       "AWS Secret Key",
		Placeholder: "",
	},
	core.Connection{
		Name:        "key",
		Type:        core.ConnectionTypeString,
		Label:       "Filename",
		Placeholder: "",
	},
	core.Connection{
		Name:        "bucket",
		Type:        core.ConnectionTypeString,
		Label:       "Bucket",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	core.Connection{
		Name:        "body",
		Type:        core.ConnectionTypeString,
		Label:       "Body",
		Placeholder: "",
	},
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
		Name:        "content_type",
		Type:        core.ConnectionTypeString,
		Label:       "Content Type",
		Placeholder: "",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	accessKey := core.FindConnection("aws_access_key", inputs)
	secretKey := core.FindConnection("aws_secret_key", inputs)
	filename := core.FindConnection("key", inputs)
	bucket := core.FindConnection("bucket", inputs)

	s, err := s3.GetService(*accessKey.String(), *secretKey.String(), "eu-west-2")
	if err != nil {
		return nil, err
	}

	result, err := s.Client.GetObject(context.Background(), &awsS3.GetObjectInput{
		Key:    aws.String(*filename.String()),
		Bucket: aws.String(*bucket.String()),
	})
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	var ct string
	if result.ContentType != nil {
		ct = *result.ContentType
	}

	return map[string]interface{}{
		"body":         string(b),
		"bucket":       *bucket.String(),
		"content_type": ct,
		"filename":     *filename.String(),
	}, nil
}
