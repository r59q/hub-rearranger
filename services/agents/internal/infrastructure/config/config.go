// Package config reads process configuration without exposing it to the domain.
package config

import "os"

func Address() string {
	if address := os.Getenv("AGENTS_ADDR"); address != "" {
		return address
	}
	return "127.0.0.1:8082"
}
