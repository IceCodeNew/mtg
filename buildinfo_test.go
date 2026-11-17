package main

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	t.Run("user Given a built binary When reading its version Then the release and module checksum are present", func(t *testing.T) {
		got := getVersion()

		if !strings.HasPrefix(got, version+" (") {
			t.Fatalf("getVersion() = %q, want prefix %q", got, version+" (")
		}
		if !strings.Contains(got, "modules checksum ") {
			t.Fatalf("getVersion() = %q, want module checksum", got)
		}
	})
}
