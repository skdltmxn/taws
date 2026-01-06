package domain

import (
	"context"
	"time"
)

type Repository struct {
	Name       string
	URI        string
	Encryption string
	Mutability string
	CreatedAt  time.Time
}

type ECRImageTag struct {
	ID        string
	Tag       string
	Digest    string
	PushedAt  time.Time
	SizeBytes int64
}

type ECRClient interface {
	ListRepositories(ctx context.Context) ([]Repository, error)
	ListImageTags(ctx context.Context, repositoryName string) ([]ECRImageTag, error)
}
