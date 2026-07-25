package integrations

import "context"

type HealthStatus struct {
	OK      bool
	Message string
}

type Integration interface {
	ID() string
	Name() string
	Kind() string
	Health(ctx context.Context) HealthStatus
}
