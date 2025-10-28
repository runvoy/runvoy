package app

import (
	"fmt"
	"time"
)

type Storage interface {
	WriteRecord(record string, table string) error
	ReadRecord(record string, table string) (string, error)
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Greet(name string) string {
	if name == "" {
		return "Hello, world!"
	}

	return name
}

func (s *Service) AddNewUser(email *string, storage Storage) error {
	record := fmt.Sprintf(`{"email": "%s", "created_at": "%s"}`, *email, time.Now().Format(time.RFC3339))

	return storage.WriteRecord(record, "api_keys")
}
