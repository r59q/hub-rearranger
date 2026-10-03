// Package config reads process configuration without exposing it to the domain.
package config

import "os"

func Address() string {
	if address := os.Getenv("AGENTS_ADDR"); address != "" {
		return address
	}
	return "127.0.0.1:8082"
}

// GitHubToken is optional for public repositories and stays server-side.
func GitHubToken() string { return os.Getenv("GITHUB_TOKEN") }

func BootstrapSource() string {
	if path := os.Getenv("AGENTS_BOOTSTRAP_SOURCE"); path != "" {
		return path
	}
	return "../.."
}
