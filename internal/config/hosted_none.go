//go:build !hosted

package config

type hosted struct{}

func loadHosted(*Config) {}

// Hosted is false in the self-hosted build.
func (Config) Hosted() bool { return false }
