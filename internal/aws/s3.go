package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/skdltmxn/taws/internal/domain"
)

type S3Client struct {
	cfg    aws.Config
	client *s3.Client
}

func NewS3Client(cfg aws.Config) *S3Client {
	return &S3Client{
		cfg:    cfg,
		client: s3.NewFromConfig(cfg),
	}
}

// getClientForBucket returns an S3 client configured for the bucket's region
func (c *S3Client) getClientForBucket(ctx context.Context, bucket string) (*s3.Client, error) {
	locOutput, err := c.client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket location: %w", err)
	}

	// LocationConstraint is empty for us-east-1, "EU" for eu-west-1
	region := string(locOutput.LocationConstraint)
	if region == "" {
		region = "us-east-1"
	} else if locOutput.LocationConstraint == types.BucketLocationConstraintEu {
		region = "eu-west-1"
	}

	// If same region, use existing client
	if region == c.cfg.Region {
		return c.client, nil
	}

	// Create a client for the bucket's region
	regionalCfg := c.cfg.Copy()
	regionalCfg.Region = region
	return s3.NewFromConfig(regionalCfg), nil
}

func (c *S3Client) ListBuckets(ctx context.Context) ([]domain.Bucket, error) {
	output, err := c.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var buckets []domain.Bucket
	for _, b := range output.Buckets {
		buckets = append(buckets, domain.Bucket{
			Name:         *b.Name,
			CreationDate: *b.CreationDate,
		})
	}

	// Sort by Name
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})

	return buckets, nil
}

func (c *S3Client) ListObjects(ctx context.Context, bucket string, prefix string) ([]domain.S3Object, error) {
	// Get a client configured for the bucket's region
	regionalClient, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return nil, err
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}

	var objects []domain.S3Object

	output, err := regionalClient.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	// Add common prefixes (folders)
	for _, p := range output.CommonPrefixes {
		objects = append(objects, domain.S3Object{
			Key:      *p.Prefix,
			IsFolder: true,
		})
	}

	// Add objects
	for _, o := range output.Contents {
		// Skip the folder itself if it appears as an object
		if *o.Key == prefix {
			continue
		}

		var size int64
		if o.Size != nil {
			size = *o.Size
		}

		objects = append(objects, domain.S3Object{
			Key:          *o.Key,
			Size:         size,
			LastModified: *o.LastModified,
			ETag:         *o.ETag,
			IsFolder:     false,
		})
	}

	return objects, nil
}

func (c *S3Client) DownloadObject(ctx context.Context, bucket, key, destPath string, onProgress func(written, total int64)) error {
	regionalClient, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return err
	}

	out, err := regionalClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer out.Body.Close()

	total := int64(-1)
	if out.ContentLength != nil {
		total = *out.ContentLength
	}
	if onProgress != nil && total > 0 {
		onProgress(0, total)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, ".taws-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	pr := &progressReader{
		r:          out.Body,
		total:      total,
		onProgress: onProgress,
	}

	if _, err := io.Copy(tmp, pr); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("failed to move temp file: %w", err)
	}
	return nil
}

func (c *S3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	regionalClient, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return err
	}

	_, err = regionalClient.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

func (c *S3Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	regionalClient, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return err
	}

	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{
			Key: aws.String(key),
		}
	}

	_, err = regionalClient.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete objects: %w", err)
	}
	return nil
}

type progressReader struct {
	r          io.Reader
	total      int64
	onProgress func(written, total int64)

	written int64
	lastAt  time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.written += int64(n)
		if p.onProgress != nil {
			now := time.Now()
			if p.lastAt.IsZero() || now.Sub(p.lastAt) > 120*time.Millisecond || (p.total > 0 && p.written >= p.total) {
				p.lastAt = now
				p.onProgress(p.written, p.total)
			}
		}
	}
	return n, err
}
