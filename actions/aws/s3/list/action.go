package aws_s3_list_bucket

import (
	"context"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/aws/s3"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 List Buckets"
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
		Name:        "aws_region",
		Type:        core.ConnectionTypeString,
		Label:       "AWS Region",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	core.Connection{
		Name:        "buckets",
		Type:        core.ConnectionTypeObject,
		Label:       "Buckets",
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
	region := core.FindConnection("aws_region", inputs)

	s, err := s3.GetService(*accessKey.String(), *secretKey.String(), *region.String())
	if err != nil {
		return nil, err
	}

	result, err := s.Client.ListBuckets(context.Background(), &awsS3.ListBucketsInput{
		BucketRegion: aws.String(*region.String()),
	})
	if err != nil {
		return nil, err
	}

	var buckets []string
	for _, b := range result.Buckets {
		buckets = append(buckets, *b.Name)
	}

	return map[string]interface{}{
		"buckets": buckets,
		"result":  0,
	}, nil
}
