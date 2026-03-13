# WebDAV Client

WebDAV client implementation for the unified storage-cli tool. This module provides WebDAV blobstore operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s dav` flag to access DAV functionality.

For general usage and build instructions, see the [main README](../README.md).

## DAV-Specific Configuration

The DAV client requires a JSON configuration file with the following structure:

``` json
{
  "endpoint":        "<string> (required)",
  "user":            "<string> (optional)",
  "password":        "<string> (optional)",
  "retry_attempts":  <uint> (optional - default: 3),
  "tls": {
    "cert": {
      "ca":          "<string> (optional - PEM-encoded CA certificate)"
    }
  },
  "secret":          "<string> (optional - required for pre-signed URLs)"
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

The request is signed using HMAC-SHA256 with a secret provided in the configuration.

The HMAC format is:
`<HTTP Verb><Object ID><Unix timestamp of the signature time><Unix timestamp of the expiration time>`

The generated URL format:
`https://blobstore.url/signed/8c/object-id?st=HMACSignatureHash&ts=GenerationTimestamp&e=ExpirationTime`

**Note:** The `/8c/` represents the SHA1 prefix directory where the blob is stored. Pre-signed URLs require the WebDAV server to have signature verification middleware. Standard WebDAV servers don't support this - it's a Cloud Foundry extension.

## Features

### SHA1-Based Prefix Directories
All blobs are stored in subdirectories based on the first 2 hex characters of their SHA1 hash (e.g., blob `my-file.txt` → path `/8c/my-file.txt`). This distributes files across 256 directories (00-ff) to prevent performance issues with large flat directories.

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

### End-to-End Tests

The DAV implementation includes Docker-based end-to-end testing with an Apache WebDAV server.

**Quick start:**
```bash
cd dav
./setup-webdav-test.sh   # Sets up Apache WebDAV with HTTPS
./test-storage-cli.sh     # Runs complete test suite
```

This tests all operations: PUT, GET, DELETE, DELETE-RECURSIVE, EXISTS, LIST, COPY, PROPERTIES, and ENSURE-STORAGE-EXISTS.

**For detailed testing instructions, see [TESTING.md](TESTING.md).**
