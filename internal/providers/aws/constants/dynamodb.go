package constants

import "time"

// DefaultExecutionListCapacity is the initial slice capacity used when listing
// executions from DynamoDB without an explicit limit.
const DefaultExecutionListCapacity = 16

// LogEventExpirationDelay is the duration after which buffered log events are
// marked for deletion via TTL.
const LogEventExpirationDelay = 10 * time.Minute
