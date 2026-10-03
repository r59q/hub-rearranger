package bootstrap

import (
	"os"
	"testing"
)

func TestStagingCannotWriteInsideASourceOrTargetCheckout(t *testing.T) {
	// Arrange.
	checkout, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(checkout, outside+"/linked-checkout"); err != nil {
		t.Fatal(err)
	}

	// Act / Assert.
	for _, path := range []string{checkout + "/package", outside + "/linked-checkout/package", outside + "/missing-parent/package"} {
		if SeparateOutput(path, checkout) {
			t.Fatalf("unsafe staging output was accepted: %s", path)
		}
	}
	if !SeparateOutput(outside+"/package", checkout) {
		t.Fatal("separate staging directory was rejected")
	}
}
