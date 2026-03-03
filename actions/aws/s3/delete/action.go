package aws_s3_delete

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
	Name         = "AWS S3 Delete"
	Description  = "AWS S3 Actions"
	Website      = "https://www.flomation.co"
	Icon         = "fa-solid fa-envelope"
	Date         = "27/11/2025"
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
