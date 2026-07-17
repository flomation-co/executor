package aws_s3_delete

import (
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
	Name         = "AWS S3 Delete"
	Description  = "Delete an object from an AWS S3 bucket"
	Website      = "https://www.flomation.co"
	Icon         = "bucket+trash"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "aws_access_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "AWS Access Key",
		Placeholder: "",
	},
	core.Connection{
		Name:        "aws_secret_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "AWS Secret Key",
		Placeholder: "",
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

	region := awscommon.InputString("aws_region", inputs)
	if region == "" {
		region = "eu-west-2"
	}
	s, err := s3.GetService(*accessKey.String(), *secretKey.String(), region)
	if err != nil {
		return nil, err
	}

	_, err = s.Client.DeleteObject(context.Background(), &awsS3.DeleteObjectInput{
		Key:    aws.String(*filename.String()),
		Bucket: aws.String(*bucket.String()),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"bucket":   *bucket.String(),
		"filename": *filename.String(),
		"result":   0,
	}, nil
}
