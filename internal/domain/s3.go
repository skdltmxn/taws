package domain

import (
	"context"
	"time"
)

type Bucket struct {
	Name         string
	CreationDate time.Time
	Region       string
}

type S3Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
	IsFolder     bool
}

type S3Client interface {
	ListBuckets(ctx context.Context) ([]Bucket, error)
	ListObjects(ctx context.Context, bucket string, prefix string) ([]S3Object, error)
	DownloadObject(ctx context.Context, bucket, key, destPath string, onProgress func(written, total int64)) error
}
