# WebDAV Client

WebDAV client implementation for the unified storage-cli tool. This module provides WebDAV blobstore operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s dav` flag to access DAV functionality.

For general usage and build instructions, see the [main README](../README.md).

## DAV-Specific Configuration

The DAV client requires a JSON configuration file with the following structure:

``` json
{
  "endpoint":               "<string> (required)",
  "user":                   "<string> (optional)",
  "password":               "<string> (optional)",
  "retry_attempts":         <uint> (optional - default: 3),
  "retry_delay":            <uint> (optional - delay in seconds between retries, default: 1),
  "tls": {
    "cert": {
      "ca":                 "<string> (optional - PEM-encoded CA certificate)"
    }
  },
  "secret":                 "<string> (optional - required for pre-signed URLs)",
  "signed_url_format":      "<string> (optional - 'hmac-sha256' (default) or 'secure-link-md5')",
  "signed_url_expiration":  <uint> (optional - signed URL lifetime in minutes, default: 15)
}
```

**Usage examples:**
```bash
# Upload a blob
storage-cli -s dav -c dav-config.json put local-file.txt remote-blob

# Fetch a blob (destination file will be overwritten if exists)
storage-cli -s dav -c dav-config.json get remote-blob local-file.txt

# Delete a blob
storage-cli -s dav -c dav-config.json delete remote-blob

# Check if blob exists
storage-cli -s dav -c dav-config.json exists remote-blob

# List all blobs
storage-cli -s dav -c dav-config.json list

# List blobs with prefix
storage-cli -s dav -c dav-config.json list my-prefix

# Copy a blob
storage-cli -s dav -c dav-config.json copy source-blob destination-blob

# Delete blobs by prefix
storage-cli -s dav -c dav-config.json delete-recursive my-prefix-

# Get blob properties (outputs JSON with ContentLength, ETag, LastModified)
storage-cli -s dav -c dav-config.json properties remote-blob

# Ensure storage exists (initialize WebDAV storage)
storage-cli -s dav -c dav-config.json ensure-storage-exists

# Generate a pre-signed URL (e.g., GET for 3600 seconds)
storage-cli -s dav -c dav-config.json sign remote-blob get 3600s
```

### Using Signed URLs with curl

```bash
# Downloading a blob:
curl -X GET <signed-url>

# Uploading a blob:
curl -X PUT -T path/to/file <signed-url>
```

## Pre-signed URLs

The `sign` command generates a pre-signed URL for a specific object, action, and duration.

The request is signed using the format selected by `signed_url_format` configuration parameter with a secret provided in the configuration.

**Supported signed URL formats:**
- **`hmac-sha256`** (default): HMAC-SHA256 signed URL format
- **`secure-link-md5`**: nginx secure_link MD5 format

The exact signature format depends on the selected format.

The generated URL format varies based on the selected format:
- **hmac-sha256**: `/signed/{blob-path}?st={hmac-sha256}&ts={timestamp}&e={expires}`
- **secure-link-md5**: `/read/{blob-path}?md5={md5-hash}&expires={timestamp}` or `/write/{blob-path}?md5={md5-hash}&expires={timestamp}`

**Note:** Pre-signed URLs require the WebDAV server to have signature verification middleware. Standard WebDAV servers don't support this - it's a Cloud Foundry extension.

## Object Path Handling

The DAV client treats object IDs as the final storage paths and uses them exactly as provided by the caller. The client does not apply any path transformations, partitioning, or prefixing - the caller is responsible for providing the complete object path including any directory structure.

For example:
- Simple paths: `my-blob-id`
- Partitioned paths: `ab/cd/my-blob-id`
- Nested paths: `folder/subfolder/my-blob-id`

All are stored exactly as specified. If your use case requires a specific directory layout (e.g., partitioning by hash prefix), implement this in the caller before invoking storage-cli.

##  BOSH Impact/Breaking Changes
  **Applies to:** storage-cli versions **v0.0.7 and later**
  
  The WebDAV client previously applied automatic path partitioning using SHA1 hash prefixes (e.g., `blob-id` → stored as `ab/blob-id` where `ab` is the first byte of SHA1). This behavior has been removed in storage-cli v0.0.7+.

  **Why:** To align with S3/GCS/Azure/AliOSS, which never had automatic partitioning. Callers now have full control over the path structure.

  **Migration:** BOSH deployments using WebDAV must now include the hash prefix in the object ID when calling storage-cli:
- **Before (≤ v0.0.6)**: Pass `blob-id` → stored as `{sha1_prefix}/blob-id`
- **After (≥ v0.0.7)**: Pass `{sha1_prefix}/blob-id` → stored as `{sha1_prefix}/blob-id`

## Features

### Automatic Retry Logic
All operations automatically retry on transient errors with 1-second delays between attempts. Default is 3 retry attempts, configurable via `retry_attempts` in config.

### TLS/HTTPS Support
Supports HTTPS connections with custom CA certificates for internal or self-signed certificates.

## Testing

### Unit Tests
Run unit tests from the repository root:

```bash
ginkgo --cover -v -r ./dav/client
```

Or using go test:
```bash
go test ./dav/client/...
```

### Integration Tests

The DAV implementation includes Go-based integration tests that run against a real WebDAV server. These tests require a WebDAV server to be available and the following environment variables to be set:

- `DAV_ENDPOINT` - WebDAV server URL
- `DAV_USER` - Username for authentication
- `DAV_PASSWORD` - Password for authentication
- `DAV_CA_CERT` - CA certificate (optional, for HTTPS with custom CA)
- `DAV_SECRET` - Secret for signed URLs (optional, for signed URL tests)

If these environment variables are not set, the integration tests will be skipped.

#### Test Server Setup

The test server uses a **multi-stage Docker build** to match production environments (CAPI/BOSH):

1. **Stage 1 (builder):** Compiles `ngx_http_dav_ext_module.so` from source with `--with-compat` flag for ABI compatibility
2. **Stage 2 (runtime):** Official `nginx:1.28-alpine` image with the compiled module loaded dynamically

**WebDAV Configuration:**
- Loads dav-ext module dynamically: `load_module /usr/lib/nginx/modules/ngx_http_dav_ext_module.so;`
- WebDAV methods: `dav_methods PUT DELETE MKCOL COPY MOVE;`
- Extended methods: `dav_ext_methods PROPFIND OPTIONS;`
- Auto-create directories: `create_full_put_path on;`
- Basic authentication with htpasswd

#### Running Integration Tests Locally

To run the full integration test suite locally:

```bash
# From the repository root
./.github/scripts/dav/setup.sh

export DAV_ENDPOINT="https://localhost:8443"
export DAV_USER="testuser"
export DAV_PASSWORD="testpass"
export DAV_CA_CERT="$(cat dav/integration/testdata/certs/server.crt)"
export DAV_SECRET="test-secret-key"

./.github/scripts/dav/run-int.sh

# Cleanup
./.github/scripts/dav/teardown.sh
```

**Test Scripts:**
- `setup.sh` - Builds and starts WebDAV test server (Docker)
- `run-int.sh` - Runs the integration tests with environment variables
- `teardown.sh` - Cleans up the test environment (stops container, removes image)

These scripts are used by the GitHub Actions workflow in `.github/workflows/dav-integration.yml`.
