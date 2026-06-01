# WebDAV Client

WebDAV client implementation for the unified storage-cli tool. This module provides WebDAV blobstore operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s dav` flag to access DAV functionality.

For general usage and build instructions, see the [main README](../README.md).

## Path Handling Contract

**IMPORTANT:** Callers are responsible for any path layout or partitioning strategy. The DAV driver does not transform object IDs.

Object IDs are used exactly as provided:
- Simple paths: `my-blob-id` → stored as `my-blob-id`
- Nested paths: `ab/cd/my-blob-id` → stored as `ab/cd/my-blob-id`

**Breaking change in v0.0.8+:** The driver no longer applies automatic 2-character path fan-out (e.g., `abcdef...` was previously stored as `ab/cd/abcdef...` in v0.0.7 and earlier). Callers must now include any desired directory structure in the object ID itself.

This behavior aligns with other storage-cli providers (S3, Azure, GCS, AliOSS), which also pass keys through unchanged.

## DAV-Specific Configuration

The DAV client requires a JSON configuration file with the following structure:

``` json
{
  "endpoint":        "<string> (required - WebDAV server URL)",
  "user":            "<string> (optional - for Basic Auth)",
  "password":        "<string> (optional - for Basic Auth)",
  "retry_attempts":  <uint> (optional - default: 3),
  "tls": {
    "cert": {
      "ca":          "<string> (optional - PEM-encoded CA certificate)"
    }
  },
  "secret":          "<string> (optional - required for sign, sign-internal, sign-public)"
}
```

**Usage examples:**
```bash
# Upload a file to WebDAV
storage-cli -s dav -c dav-config.json put local-file.txt remote-blob

# Download a file from WebDAV
storage-cli -s dav -c dav-config.json get remote-blob local-file.txt

# Check if blob exists
storage-cli -s dav -c dav-config.json exists remote-blob

# Delete a blob
storage-cli -s dav -c dav-config.json delete remote-blob

# Generate pre-signed URLs (requires secret in config)
storage-cli -s dav -c dav-config.json sign remote-blob get 1h
storage-cli -s dav -c dav-config.json sign-internal remote-blob get 1h
storage-cli -s dav -c dav-config.json sign-public remote-blob get 1h
```

## Features

- **Put** - Upload files to WebDAV server
- **Get** - Download files from WebDAV server
- **Delete** - Delete individual blobs
- **Exists** - Check if a blob exists
- **Copy** - Server-side blob copying via WebDAV COPY method
- **List** - List all blobs or filter by prefix
- **DeleteRecursive** - Delete all blobs matching a prefix
- **Properties** - Retrieve blob metadata (ContentLength, ETag, LastModified)
- **EnsureStorageExists** - No-op (nginx auto-creates directories on PUT)
- **Sign** / **SignInternal** / **SignPublic** - Generate pre-signed URLs with HMAC-SHA256

### Automatic Retry Logic
All operations automatically retry on transient errors. Default is 3 retry attempts, configurable via `retry_attempts` in config.

### TLS/HTTPS Support
Supports HTTPS connections with custom CA certificates for internal or self-signed certificates.

## Pre-signed URLs

The `sign`, `sign-internal`, and `sign-public` commands generate a pre-signed URL for a specific object, action, and duration. Requires `secret` in config.

- `sign` and `sign-internal` use the private `endpoint` as the host — for internal consumers (e.g. stager)
- `sign-public` uses `public_endpoint` as the host (falls back to `endpoint` if not set) — for external consumers (e.g. CF API downloads via gorouter)

The directory key is always derived from the private `endpoint` path regardless of which command is used.

The URL uses nginx `secure_link_hmac` format (HMAC-SHA256):

```
https://{host}/signed/{directoryKey}/{objectID}?st={signature}&ts={timestamp}&e={duration_seconds}
```

The `directoryKey` is extracted from the endpoint path (e.g. `/admin/bbl-envs-drops-leia` → `bbl-envs-drops-leia`). nginx captures `{directoryKey}/{objectID}` as `$blob_path` and uses it for both the file alias and the HMAC message.

HMAC input: `{VERB}{directoryKey}/{objectID}{unix_timestamp}{duration_seconds}`

## Testing

### Unit Tests
Run unit tests from the repository root:

```bash
go test ./dav/client/...
```

Or using ginkgo:
```bash
ginkgo --cover -v -r ./dav/client
```

