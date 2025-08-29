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
