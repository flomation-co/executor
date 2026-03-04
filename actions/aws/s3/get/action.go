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
	Description  = "AWS S3 Actions"
	Website      = "https://www.flomation.co"
	Icon         = "envelope"
	Date         = "03/01/2026"
)

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

	return map[string]interface{}{
		"body":     b,
		"bucket":   *bucket.String(),
		"filename": *filename.String(),
		"result":   0,
	}, nil
}
