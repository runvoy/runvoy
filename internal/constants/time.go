package constants

import "time"

// ClaimURLExpirationMinutes is the number of minutes after which a claim URL expires.
const ClaimURLExpirationMinutes = 15

// DefaultContextTimeout is the default timeout for context operations.
const DefaultContextTimeout = 10 * time.Second

// ScriptContextTimeout is the timeout for script context operations.
const ScriptContextTimeout = 10 * time.Second

// LongScriptContextTimeout is the timeout for longer script context operations.
const LongScriptContextTimeout = 30 * time.Second

// TestContextTimeout is the timeout for test contexts.
const TestContextTimeout = 5 * time.Second

// SpinnerTickerInterval is the interval between spinner frame updates.
const SpinnerTickerInterval = 80 * time.Millisecond
