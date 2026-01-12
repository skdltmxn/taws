package domain

import (
	"context"
	"time"
)

type FunctionState string

const (
	FunctionStateActive   FunctionState = "Active"
	FunctionStatePending  FunctionState = "Pending"
	FunctionStateInactive FunctionState = "Inactive"
	FunctionStateFailed   FunctionState = "Failed"
)

type LambdaFunction struct {
	Name         string
	ARN          string
	Runtime      string
	Handler      string
	Description  string
	Timeout      int32
	MemorySize   int32
	CodeSize     int64
	State        FunctionState
	LastModified time.Time
	Role         string
}

type LambdaCodeFile struct {
	Path    string
	Content string
}

type LambdaClient interface {
	ListFunctions(ctx context.Context) ([]LambdaFunction, error)
	DeleteFunction(ctx context.Context, functionName string) error
	GetFunctionCode(ctx context.Context, functionName string) ([]LambdaCodeFile, error)
}
