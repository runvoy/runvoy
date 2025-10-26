package main

import (
	"fmt"
	"time"
)

func generateExecutionID() string {
	return fmt.Sprintf("exec_%s_%06d",
		time.Now().UTC().Format("20060102_150405"),
		time.Now().Nanosecond()/1000)
}
