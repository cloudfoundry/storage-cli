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
  "secret":          "<string> (optional - required for pre-signed URLs)"
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

# Generate a pre-signed URL (requires secret in config)
storage-cli -s dav -c dav-config.json sign remote-blob get 1h
```

## Features

### Basic Operations (Available)
- **Put** - Upload files to WebDAV server
- **Get** - Download files from WebDAV server
- **Delete** - Delete individual blobs
- **Exists** - Check if a blob exists
- **Sign** - Generate pre-signed URLs with HMAC-SHA256

### Advanced Operations (Coming Soon)
The following operations will be available in future releases:
- **List** - List all blobs or filter by prefix
- **Copy** - Server-side blob copying via WebDAV COPY method
- **DeleteRecursive** - Delete all blobs matching a prefix
- **Properties** - Retrieve blob metadata (ContentLength, ETag, LastModified)
- **EnsureStorageExists** - Initialize WebDAV directory structure

### Automatic Retry Logic
All operations automatically retry on transient errors. Default is 3 retry attempts, configurable via `retry_attempts` in config.

### TLS/HTTPS Support
Supports HTTPS connections with custom CA certificates for internal or self-signed certificates.

## Pre-signed URLs

The `sign` command generates a pre-signed URL for a specific object, action, and duration.

The request is signed using HMAC-SHA256 with a secret provided in the configuration.

The HMAC format is:
`<HTTP Verb><Object ID><Unix timestamp of the signature time><Unix timestamp of the expiration time>`

The generated URL format:
`https://blobstore.url/signed/object-id?st=HMACSignatureHash&ts=GenerationTimestamp&e=ExpirationTimestamp`

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

