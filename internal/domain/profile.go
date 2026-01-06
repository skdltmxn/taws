package domain

// AWSProfile represents an AWS profile from config/credentials files
type AWSProfile struct {
	Name      string
	Region    string
	SSOStart  string // SSO start URL if SSO profile
	IsSSO     bool
	IsDefault bool
}
