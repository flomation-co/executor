package aws_s3_put

import (
	"bytes"
	"context"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/aws/s3"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Put"
	Description  = "AWS S3 Actions"
	Website      = "https://www.flomation.co"
	Icon         = "bucket"
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
	core.Connection{
		Name:        "contents",
		Type:        core.ConnectionTypeString,
		Label:       "Contents",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
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

	s, err := s3.GetService(*accessKey.String(), *secretKey.String(), "eu-west-2")
	if err != nil {
		return nil, err
	}

	_, err = s.Client.PutObject(context.Background(), &awsS3.PutObjectInput{
		Key:    aws.String(*filename.String()),
		Bucket: aws.String(*bucket.String()),
		Body:   bytes.NewReader([]byte(*contents.String())),
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
