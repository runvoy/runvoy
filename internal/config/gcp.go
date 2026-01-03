package config

// GCPConfig represents GCP-specific configuration.
type GCPConfig struct {
	ProjectID      string `mapstructure:"project_id" yaml:"project_id"`
	Region         string `mapstructure:"region" yaml:"region"`
	BillingAccount string `mapstructure:"billing_account" yaml:"billing_account"`
	OrgID          string `mapstructure:"org_id" yaml:"org_id"`
	ReleasesBucket string `mapstructure:"releases_bucket" yaml:"releases_bucket"`
}
