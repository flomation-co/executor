package s3

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Service struct {
	Client *s3.Client
}

func GetService(accessKey string, secretKey string, region string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return &Service{
		Client: s3.NewFromConfig(cfg),
	}, nil
}
