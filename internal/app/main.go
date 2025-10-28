package app

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
