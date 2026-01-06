package app

import (
	"context"

	"github.com/skdltmxn/taws/internal/aws"
	"github.com/skdltmxn/taws/internal/config"
	"github.com/skdltmxn/taws/internal/domain"
)

type App struct {
	Config    *config.Config
	AWSClient domain.AWSClient
	Identity  *domain.AWSIdentity
	AuthError error
}

func New() *App {
	return &App{}
}

func (a *App) Init(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	a.Config = cfg

	client, err := aws.NewClient(ctx, cfg.Region, cfg.Profile)
	if err != nil {
		return err
	}
	a.AWSClient = client

	identity, err := client.VerifyCredentials(ctx)
	if err != nil {
		a.AuthError = err
	} else {
		a.Identity = identity
	}

	return nil
}

// SwitchProfile changes the active AWS profile and re-initializes the client
func (a *App) SwitchProfile(ctx context.Context, profile, region string) error {
	// Update config
	a.Config.Profile = profile
	if region != "" {
		a.Config.Region = region
	}

	// Create new client with new profile
	client, err := aws.NewClient(ctx, a.Config.Region, profile)
	if err != nil {
		return err
	}
	a.AWSClient = client

	// Verify credentials
	identity, err := client.VerifyCredentials(ctx)
	if err != nil {
		a.AuthError = err
		a.Identity = nil
		return err
	}

	a.Identity = identity
	a.AuthError = nil
	return nil
}

// SwitchRegion changes the active AWS region and re-initializes the client
func (a *App) SwitchRegion(ctx context.Context, region string) error {
	a.Config.Region = region

	// Create new client with new region
	client, err := aws.NewClient(ctx, region, a.Config.Profile)
	if err != nil {
		return err
	}
	a.AWSClient = client

	// Verify credentials (region change shouldn't affect auth, but verify anyway)
	identity, err := client.VerifyCredentials(ctx)
	if err != nil {
		a.AuthError = err
		a.Identity = nil
		return err
	}

	a.Identity = identity
	a.AuthError = nil
	return nil
}
