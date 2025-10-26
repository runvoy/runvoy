package main

import "golang.org/x/crypto/bcrypt"

func authenticate(cfg *Config, apiKey string) bool {
	if apiKey == "" || cfg.APIKeyHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(cfg.APIKeyHash), []byte(apiKey)) == nil
}
