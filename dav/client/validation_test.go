package client

import (
	"testing"
)

func TestValidateBlobID(t *testing.T) {
	tests := []struct {
		name      string
		blobID    string
		wantError bool
	}{
		// Valid blob IDs
		{name: "simple blob", blobID: "file.txt", wantError: false},
		{name: "hierarchical blob", blobID: "foo/bar/baz.txt", wantError: false},
		{name: "deep hierarchy", blobID: "a/b/c/d/e/f.txt", wantError: false},
		{name: "with dashes", blobID: "my-file.txt", wantError: false},
		{name: "with underscores", blobID: "my_file.txt", wantError: false},
		{name: "with dots", blobID: "file.tar.gz", wantError: false},
		{name: "double dots in filename", blobID: "my..file.txt", wantError: false},
		{name: "version with dots", blobID: "version..1", wantError: false},
		{name: "nested with double dots", blobID: "foo/my..file.txt", wantError: false},
		{name: "uuid-like", blobID: "abc-123-def-456", wantError: false},
		{name: "nested with uuid", blobID: "backups/2024/abc-123.tar.gz", wantError: false},

		// Invalid blob IDs - empty
		{name: "empty string", blobID: "", wantError: true},

		// Invalid blob IDs - leading/trailing slashes
		{name: "leading slash", blobID: "/foo/bar.txt", wantError: true},
		{name: "trailing slash", blobID: "foo/bar/", wantError: true},
		{name: "both slashes", blobID: "/foo/bar/", wantError: true},

		// Invalid blob IDs - path traversal
		{name: "dot-dot segment", blobID: "foo/../bar.txt", wantError: true},
		{name: "dot-dot at start", blobID: "../bar.txt", wantError: true},
		{name: "dot-dot at end", blobID: "foo/..", wantError: true},
		{name: "multiple dot-dots", blobID: "foo/../../bar.txt", wantError: true},
		{name: "dot segment", blobID: "foo/./bar.txt", wantError: true},
		{name: "just dot-dot", blobID: "..", wantError: true},
		{name: "just dot", blobID: ".", wantError: true},

		// Invalid blob IDs - empty segments
		{name: "double slash", blobID: "foo//bar.txt", wantError: true},
		{name: "multiple double slashes", blobID: "foo///bar.txt", wantError: true},
		{name: "double slash at start", blobID: "//foo/bar.txt", wantError: true},

		// Invalid blob IDs - control characters
		{name: "null byte", blobID: "foo\x00bar.txt", wantError: true},
		{name: "newline", blobID: "foo\nbar.txt", wantError: true},
		{name: "tab", blobID: "foo\tbar.txt", wantError: true},
		{name: "carriage return", blobID: "foo\rbar.txt", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBlobID(tt.blobID)
			if tt.wantError && err == nil {
				t.Errorf("validateBlobID(%q) expected error, got nil", tt.blobID)
			}
			if !tt.wantError && err != nil {
				t.Errorf("validateBlobID(%q) unexpected error: %v", tt.blobID, err)
			}
		})
	}
}

func TestValidatePrefix(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		wantError bool
	}{
		// Valid prefixes
		{name: "simple prefix", prefix: "foo", wantError: false},
		{name: "hierarchical prefix", prefix: "foo/bar", wantError: false},
		{name: "prefix with trailing slash", prefix: "foo/", wantError: false},
		{name: "deep prefix with trailing slash", prefix: "foo/bar/baz/", wantError: false},
		{name: "prefix with dashes", prefix: "my-prefix", wantError: false},
		{name: "prefix with dots", prefix: "backup.2024", wantError: false},
		{name: "prefix with double dots in name", prefix: "my..prefix/", wantError: false},

		// Invalid prefixes - empty
		{name: "empty string", prefix: "", wantError: true},

		// Invalid prefixes - leading slash
		{name: "leading slash", prefix: "/foo", wantError: true},
		{name: "leading slash with trailing", prefix: "/foo/", wantError: true},

		// Invalid prefixes - path traversal
		{name: "dot-dot segment", prefix: "foo/../bar", wantError: true},
		{name: "dot-dot at start", prefix: "../foo", wantError: true},
		{name: "dot segment", prefix: "foo/./bar", wantError: true},
		{name: "just dot-dot", prefix: "..", wantError: true},
		{name: "dot-dot with trailing slash", prefix: "../", wantError: true},

		// Invalid prefixes - empty segments
		{name: "double slash", prefix: "foo//bar", wantError: true},
		{name: "double slash at end", prefix: "foo//", wantError: true},

		// Invalid prefixes - control characters
		{name: "null byte", prefix: "foo\x00bar", wantError: true},
		{name: "newline", prefix: "foo\nbar", wantError: true},
		{name: "tab", prefix: "foo\tbar", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrefix(tt.prefix)
			if tt.wantError && err == nil {
				t.Errorf("validatePrefix(%q) expected error, got nil", tt.prefix)
			}
			if !tt.wantError && err != nil {
				t.Errorf("validatePrefix(%q) unexpected error: %v", tt.prefix, err)
			}
		})
	}
}
