package aws

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/skdltmxn/taws/internal/domain"
)

type IAMClient struct {
	client *iam.Client
}

func NewIAMClient(cfg aws.Config) *IAMClient {
	return &IAMClient{
		client: iam.NewFromConfig(cfg),
	}
}

func (c *IAMClient) ListUsers(ctx context.Context) ([]domain.IAMUser, error) {
	input := &iam.ListUsersInput{}
	var users []domain.IAMUser

	paginator := iam.NewListUsersPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}

		for _, u := range output.Users {
			users = append(users, domain.IAMUser{
				UserName:   derefString(u.UserName),
				UserID:     derefString(u.UserId),
				CreateDate: derefTime(u.CreateDate),
				Arn:        derefString(u.Arn),
			})
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].UserName < users[j].UserName
	})

	return users, nil
}

func (c *IAMClient) ListRoles(ctx context.Context) ([]domain.IAMRole, error) {
	input := &iam.ListRolesInput{}
	var roles []domain.IAMRole

	paginator := iam.NewListRolesPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list roles: %w", err)
		}

		for _, r := range output.Roles {
			roles = append(roles, domain.IAMRole{
				RoleName:   derefString(r.RoleName),
				RoleID:     derefString(r.RoleId),
				CreateDate: derefTime(r.CreateDate),
				Arn:        derefString(r.Arn),
			})
		}
	}

	sort.Slice(roles, func(i, j int) bool {
		return roles[i].RoleName < roles[j].RoleName
	})

	return roles, nil
}
