package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/skdltmxn/taws/internal/domain"
)

type ECRClient struct {
	client *ecr.Client
}

func NewECRClient(cfg aws.Config) *ECRClient {
	return &ECRClient{
		client: ecr.NewFromConfig(cfg),
	}
}

func (c *ECRClient) ListRepositories(ctx context.Context) ([]domain.Repository, error) {
	input := &ecr.DescribeRepositoriesInput{}
	var repos []domain.Repository

	paginator := ecr.NewDescribeRepositoriesPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		for _, r := range output.Repositories {
			encryption := "AES256"
			if r.EncryptionConfiguration != nil && r.EncryptionConfiguration.EncryptionType != "" {
				encryption = string(r.EncryptionConfiguration.EncryptionType)
			}

			mutability := "MUTABLE"
			if r.ImageTagMutability != "" {
				mutability = string(r.ImageTagMutability)
			}

			repos = append(repos, domain.Repository{
				Name:       derefString(r.RepositoryName),
				URI:        derefString(r.RepositoryUri),
				Encryption: encryption,
				Mutability: mutability,
				CreatedAt:  derefTime(r.CreatedAt),
			})
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})

	return repos, nil
}

func (c *ECRClient) ListImageTags(ctx context.Context, repositoryName string) ([]domain.ECRImageTag, error) {
	if strings.TrimSpace(repositoryName) == "" {
		return nil, fmt.Errorf("repository name is required")
	}

	var tags []domain.ECRImageTag
	paginator := ecr.NewDescribeImagesPaginator(c.client, &ecr.DescribeImagesInput{
		RepositoryName: aws.String(repositoryName),
	})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list images: %w", err)
		}

		for _, img := range output.ImageDetails {
			digest := derefString(img.ImageDigest)
			pushedAt := derefTime(img.ImagePushedAt)
			sizeBytes := derefInt64(img.ImageSizeInBytes)

			if len(img.ImageTags) == 0 {
				id := digest
				if id == "" {
					id = fmt.Sprintf("%s|untagged|%d", repositoryName, pushedAt.Unix())
				}
				tags = append(tags, domain.ECRImageTag{
					ID:        id,
					Tag:       "<untagged>",
					Digest:    digest,
					PushedAt:  pushedAt,
					SizeBytes: sizeBytes,
				})
				continue
			}

			for _, t := range img.ImageTags {
				tag := strings.TrimSpace(t)
				if tag == "" {
					tag = "<none>"
				}
				id := repositoryName + "|" + tag
				if digest != "" {
					id = id + "|" + digest
				}
				tags = append(tags, domain.ECRImageTag{
					ID:        id,
					Tag:       tag,
					Digest:    digest,
					PushedAt:  pushedAt,
					SizeBytes: sizeBytes,
				})
			}
		}
	}

	sort.Slice(tags, func(i, j int) bool {
		pi := tags[i].PushedAt
		pj := tags[j].PushedAt
		if !pi.Equal(pj) {
			return pi.After(pj)
		}
		return tags[i].Tag < tags[j].Tag
	})

	return tags, nil
}
