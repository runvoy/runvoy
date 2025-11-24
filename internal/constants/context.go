package constants

// ConfigCtxKeyType is the type for the config context key
type ConfigCtxKeyType string

// ConfigCtxKey is the key used to store config in context
const ConfigCtxKey ConfigCtxKeyType = "config"

// StartTimeCtxKeyType is the type for start time context keys
type StartTimeCtxKeyType string

// StartTimeCtxKey is the key used to store the start time in context
const StartTimeCtxKey StartTimeCtxKeyType = "startTime"

// RequestIDLogField is the field name used for request ID in log entries
const RequestIDLogField = "request_id"
