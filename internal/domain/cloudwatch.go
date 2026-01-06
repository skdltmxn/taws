package domain

import (
	"context"
	"time"
)

type CloudWatchLogGroup struct {
	Name          string
	RetentionDays int32
	StoredBytes   int64
}

type CloudWatchLogEvent struct {
	EventID       string
	Timestamp     time.Time
	Message       string
	LogStreamName string
}

type CloudWatchClient interface {
	ListLogGroups(ctx context.Context) ([]CloudWatchLogGroup, error)
	FilterLogEvents(ctx context.Context, logGroupName string, startTime time.Time) ([]CloudWatchLogEvent, error)
}
