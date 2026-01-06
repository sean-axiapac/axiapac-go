package filesystem

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func ReadFile(bucket string, key string, ctx context.Context, outStream io.Writer) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create an S3 client
	client := s3.NewFromConfig(cfg)

	// Get the object
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object %s from bucket %s: %w", key, bucket, err)
	}
	defer resp.Body.Close()

	// Write the S3 object data to the provided stream
	_, err = io.Copy(outStream, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy object %s from bucket %s: %w", key, bucket, err)
	}

	return nil
}

func ListFiles(bucket string, ctx context.Context) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucket, err)
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}
