package client

import (
	"testing"
)

func TestExtractSignEndpoint(t *testing.T) {
	t.Run("returns endpoint host and scheme", func(t *testing.T) {
		got, err := extractSignEndpoint("https://blobstore.service.cf.internal:4443/admin/bucket")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "https://blobstore.service.cf.internal:4443" {
			t.Fatalf("unexpected endpoint: %q", got)
		}
	})

	t.Run("returns parse error for invalid URL", func(t *testing.T) {
		_, err := extractSignEndpoint("http://[::1")
		if err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}

func TestExtractDirectoryKey(t *testing.T) {
	t.Run("extracts admin directory key", func(t *testing.T) {
		got, err := extractDirectoryKey("https://blobstore.service.cf.internal:4443/admin/bbl-envs-drops-vader")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "bbl-envs-drops-vader" {
			t.Fatalf("unexpected key: %q", got)
		}
	})

	t.Run("returns fallback last path segment when no admin segment", func(t *testing.T) {
		got, err := extractDirectoryKey("https://example.com/root/dir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "dir" {
			t.Fatalf("unexpected key: %q", got)
		}
	})

	t.Run("returns parse error for invalid URL", func(t *testing.T) {
		_, err := extractDirectoryKey("http://[::1")
		if err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}

