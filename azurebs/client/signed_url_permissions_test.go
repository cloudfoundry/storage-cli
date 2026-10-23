package client_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/cloudfoundry/storage-cli/azurebs/client"
	"github.com/cloudfoundry/storage-cli/azurebs/config"
)

func TestSignedURLPermissionsFollowRequestedAction(t *testing.T) {
	storage, err := client.NewStorageClient(config.AZStorageConfig{AccountName: "account", AccountKey: "Zm9vYmFy", ContainerName: "container"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ action, permissions, timeout string }{{"GET", "r", "1800"}, {"PUT", "cw", "2700"}} {
		t.Run(tc.action, func(t *testing.T) {
			raw, err := storage.SignedUrl(tc.action, "object", time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			query := parsed.Query()
			if query.Get("sp") != tc.permissions {
				t.Fatalf("permissions=%s want%s", query.Get("sp"), tc.permissions)
			}
			if query.Get("sig") == "" || query.Get("timeout") != tc.timeout {
				t.Fatal("lost signature or timeout")
			}
		})
	}
	if raw, err := storage.SignedUrl("DELETE", "object", time.Hour); err == nil || raw != "" {
		t.Fatal("unsupported action accepted")
	}
}
