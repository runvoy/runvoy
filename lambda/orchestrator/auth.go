package main

import "golang.org/x/crypto/bcrypt"

func authenticate(apiKey string) bool {
	if apiKey == "" || apiKeyHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(apiKeyHash), []byte(apiKey)) == nil
}
