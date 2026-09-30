package config

import "testing"

func TestAddressDefaultsToLoopbackWithoutCredentials(t *testing.T) {
	// Arrange.
	t.Setenv("AGENTS_ADDR", "")
	t.Setenv("GITHUB_TOKEN", "")

	// Act.
	address := Address()

	// Assert.
	if address != "127.0.0.1:8082" {
		t.Fatalf("address = %s", address)
	}
}

func TestAddressSupportsTheComposeListenAddress(t *testing.T) {
	// Arrange.
	t.Setenv("AGENTS_ADDR", ":8082")

	// Act.
	address := Address()

	// Assert.
	if address != ":8082" {
		t.Fatalf("address = %s", address)
	}
}
