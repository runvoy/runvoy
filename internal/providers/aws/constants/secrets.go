package constants

// SecretsPrefix is the prefix for AWS secrets management.
const SecretsPrefix = "/runvoy/secrets" //nolint:gosec // G101: This is a constant, not a hardcoded credential

// SSMParameterMaxResults is the maximum number of results for SSM DescribeParameters.
const SSMParameterMaxResults = int32(50)
