# WebDAV Client

WebDAV client implementation for the unified storage-cli tool. This module provides WebDAV blobstore operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s dav` flag to access DAV functionality.

For general usage and build instructions, see the [main README](../README.md).

## DAV-Specific Configuration

The DAV client requires a JSON configuration file with WebDAV endpoint details and credentials.

**Usage examples:**
```bash
# Upload an object
storage-cli -s dav -c dav-config.json put local-file.txt remote-object

# Fetch an object
storage-cli -s dav -c dav-config.json get remote-object local-file.txt

# Delete an object
storage-cli -s dav -c dav-config.json delete remote-object

# Check if an object exists
storage-cli -s dav -c dav-config.json exists remote-object

# Generate a signed URL (e.g., GET for 1 hour)
storage-cli -s dav -c dav-config.json sign remote-object get 60s
```

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
ginkgo --cover -v -r ./dav/...
```
