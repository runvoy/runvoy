package constants

// ExecutionsSliceInitialCapacity is the initial capacity for executions slices
const ExecutionsSliceInitialCapacity = 64

// EnvVarSplitLimit is the limit for splitting environment variable strings (KEY=VALUE)
const EnvVarSplitLimit = 2

// RegexMatchCountEnvVar is the expected number of regex matches for environment variable parsing
const RegexMatchCountEnvVar = 3

// ExpectedArgsCreateConfigFile is the expected number of arguments for create-config-file script
const ExpectedArgsCreateConfigFile = 2

// ExpectedArgsSeedAdminUser is the expected number of arguments for seed-admin-user script
const ExpectedArgsSeedAdminUser = 3

// ExpectedArgsTruncateDynamoDBTable is the expected number of arguments for truncate-dynamodb-table script
const ExpectedArgsTruncateDynamoDBTable = 2

// MinimumArgsDeleteS3Buckets is the minimum number of arguments for delete-s3-buckets script
// (script name + at least 1 bucket)
const MinimumArgsDeleteS3Buckets = 2

// MinimumArgsUpdateReadmeHelp is the minimum number of arguments for update-readme-help script
const MinimumArgsUpdateReadmeHelp = 2
