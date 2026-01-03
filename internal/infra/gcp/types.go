package gcp

type ResourceFile struct {
	Project       *ProjectResource
	Firestore     *FirestoreResource
	CloudRun      *CloudRunResource
	SecretManager *SecretManagerResource
}

type ProjectResource struct {
	ProjectID      string `yaml:"id"`
	Name           string `yaml:"name"`
	BillingAccount  string `yaml:"billing_account"`
	OrgID          string `yaml:"org_id"`
}

type FirestoreResource struct {
	Database *DatabaseConfig `yaml:"database"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`
	Location string `yaml:"location"`
}

type CloudRunResource struct {
	Orchestrator *CloudRunService `yaml:"orchestrator"`
	Processor    *CloudRunService `yaml:"processor"`
}

type CloudRunService struct {
	Name    string `yaml:"name"`
	Region   string `yaml:"region"`
	Runtime  string `yaml:"runtime"`
	Source   string `yaml:"source"`
}

type SecretManagerResource struct {
	EncryptionKey *KMSKeyConfig `yaml:"encryption_key"`
}

type KMSKeyConfig struct {
	KeyRing  string `yaml:"key_ring"`
	Key      string `yaml:"key"`
	Location  string `yaml:"location"`
}

type ResourceResult struct {
	Type       string
	ResourceID string
	State      string
}
