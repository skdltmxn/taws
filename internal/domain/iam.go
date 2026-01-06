package domain

import (
	"context"
	"time"
)

type IAMUser struct {
	UserName   string
	UserID     string
	CreateDate time.Time
	Arn        string
}

type IAMRole struct {
	RoleName   string
	RoleID     string
	CreateDate time.Time
	Arn        string
}

type IAMClient interface {
	ListUsers(ctx context.Context) ([]IAMUser, error)
	ListRoles(ctx context.Context) ([]IAMRole, error)
}
