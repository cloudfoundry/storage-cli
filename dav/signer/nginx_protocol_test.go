package signer_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry/storage-cli/dav/signer"
)

// Run with a real Nginx binary containing ngx_http_hmac_secure_link_module and
// the matching BOSH blobstore template. Only the verifier's message directive is
// extracted; a local fixture server exposes its verification result in a header.
func TestNGINXVerifierBindsSignedURLFields(t *testing.T) {
	binary := os.Getenv("DAV_NGINX_BINARY")
	templatePath := os.Getenv("DAV_VERIFIER_TEMPLATE")
	if binary == "" || templatePath == "" {
		t.Skip("set DAV_NGINX_BINARY and DAV_VERIFIER_TEMPLATE for the real verifier check")
	}
	template, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	directive := regexp.MustCompile(`(?m)^\s*secure_link_hmac_message\s+([^;]+);`).FindSubmatch(template)
	if directive == nil {
		t.Fatal("verifier template has no HMAC message directive")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"foo", "foo1"} {
		path := filepath.Join(dir, "html", "signed", "09", name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture-blob"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	config := fmt.Sprintf(`daemon off;
master_process off;
pid %s/nginx.pid;
error_log %s/error.log notice;
events { worker_connections 64; }
http {
 access_log off;
 map $uri $object_id { /signed/09/foo 09/foo; /signed/09/foo1 09/foo1; default other; }
 server {
  listen %s;
  root %s/html;
  location /signed/ {
   secure_link_hmac $arg_st,$arg_ts,$arg_e;
   secure_link_hmac_secret fixture-signing-secret;
   secure_link_hmac_message %s;
   secure_link_hmac_algorithm sha256;
   add_header X-Fixture-Verified $secure_link_hmac always;
  }
 }
}
`, dir, dir, address, dir, directive[1])
	configPath := filepath.Join(dir, "nginx.conf")
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	command := exec.Command(binary, "-p", dir+"/", "-c", configPath)
	command.Stderr = &diagnostics
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		if err := command.Wait(); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Error(err)
			}
		}
	}()
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			if err := connection.Close(); err != nil {
				t.Fatal(err)
			}
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("fixture Nginx did not start; inspect its configuration and build options")
	}
	httpClient := &http.Client{Timeout: time.Second}
	verify := func(t *testing.T, raw, method string, wantValid bool) {
		t.Helper()
		request, err := http.NewRequest(method, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := httpClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		valid := response.Header.Get("X-Fixture-Verified") == "1"
		if valid != wantValid {
			t.Fatalf("module accepted=%v, want%v (HTTP%d)", valid, wantValid, response.StatusCode)
		}
	}
	now := time.Now().Unix()
	base := "http://" + address
	signedURL, err := signer.NewSigner("fixture-signing-secret").GenerateSignedURL(base, "09", "foo", "GET", time.Unix(now, 0), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("valid", func(t *testing.T) { verify(t, signedURL, "GET", true) })
	mutate := func(raw string, changes map[string]string) string {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		for key, value := range changes {
			query.Set(key, value)
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	t.Run("timestamp-duration digit shift", func(t *testing.T) {
		verify(t, mutate(signedURL, map[string]string{"ts": strconv.FormatInt(now*10+3, 10), "e": "600"}), "GET", false)
	})
	t.Run("duration", func(t *testing.T) { verify(t, mutate(signedURL, map[string]string{"e": "36000"}), "GET", false) })
	t.Run("method", func(t *testing.T) { verify(t, signedURL, "PUT", false) })
	t.Run("path-timestamp digit shift", func(t *testing.T) {
		raw, err := signer.NewSigner("fixture-signing-secret").GenerateSignedURL(base, "09", "foo1", "GET", time.Unix(now, 0), time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		verify(t, raw, "GET", true)
		raw = strings.Replace(raw, "/foo1?", "/foo?", 1)
		verify(t, mutate(raw, map[string]string{"ts": "1" + strconv.FormatInt(now, 10)}), "GET", false)
	})
	t.Run("expired", func(t *testing.T) {
		raw, err := signer.NewSigner("fixture-signing-secret").GenerateSignedURL(base, "09", "foo", "GET", time.Unix(now-7200, 0), time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		verify(t, raw, "GET", false)
	})
	t.Run("legacy format rejected", func(t *testing.T) {
		mac := hmac.New(sha256.New, []byte("fixture-signing-secret"))
		if _, err := fmt.Fprintf(mac, "GET09/foo%d3600", now); err != nil {
			t.Fatal(err)
		}
		verify(t, mutate(signedURL, map[string]string{"st": base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}), "GET", false)
	})
}
