package main

import (
	sharedAPI "mycli/pkg/api"
)

// Type aliases for shared API types
// Keeping these local simplifies handler signatures

type Request = sharedAPI.Request

type Response = sharedAPI.Response
