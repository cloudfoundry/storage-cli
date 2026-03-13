# Testing storage-cli DAV Implementation

This guide helps you test the refactored DAV storage-cli implementation against a real WebDAV server with TLS.

## Prerequisites

- Docker and docker-compose installed
- OpenSSL installed
- Go installed (for building storage-cli)

## Quick Start

### 1. Set up WebDAV Test Server

```bash
cd /Users/I546390/SAPDevelop/membrane_inline/storage-cli/dav
chmod +x setup-webdav-test.sh
./setup-webdav-test.sh
```

This will:
- Create a `webdav-test/` directory
- Generate self-signed certificates
- Start a WebDAV server on `https://localhost:8443`
- Configure authentication (user: `testuser`, password: `testpass`)

### 2. Run All Tests

```bash
chmod +x test-storage-cli.sh
./test-storage-cli.sh
```

This will test all operations:
- ✓ PUT - Upload file
- ✓ EXISTS - Check existence
- ✓ LIST - List blobs
- ✓ PROPERTIES - Get metadata
- ✓ GET - Download file
- ✓ COPY - Copy blob
- ✓ DELETE - Delete blob
- ✓ DELETE-RECURSIVE - Delete with prefix
- ✓ ENSURE-STORAGE-EXISTS - Initialize storage

## Manual Testing

If you prefer to test manually:

### 1. Build storage-cli

```bash
cd /Users/I546390/SAPDevelop/membrane_inline/storage-cli
go build -o storage-cli main.go
```

### 2. Create config.json

```bash
cd dav
cat > config.json <<EOF
{
  "endpoint": "https://localhost:8443",
  "user": "testuser",
  "password": "testpass",
  "tls": {
    "cert": {
      "ca": "$(cat webdav-test/certs/ca.crt | sed 's/$/\\n/' | tr -d '\n' | sed 's/\\n$//')"
    }
  }
}
EOF
```

### 3. Test Individual Operations

```bash
# Upload a file
echo "Test content" > test.txt
../storage-cli -s dav -c config.json put test.txt remote.txt

# Check if file exists
../storage-cli -s dav -c config.json exists remote.txt

# List all files
../storage-cli -s dav -c config.json list

# Get file properties
../storage-cli -s dav -c config.json properties remote.txt

# Download file
../storage-cli -s dav -c config.json get remote.txt downloaded.txt

# Copy file
../storage-cli -s dav -c config.json copy remote.txt remote-copy.txt

# Delete file
../storage-cli -s dav -c config.json delete remote-copy.txt

# Delete all files with prefix
../storage-cli -s dav -c config.json delete-recursive remote

# Ensure storage exists
../storage-cli -s dav -c config.json ensure-storage-exists
```

## Configuration Options

### config.json Structure

```json
{
  "endpoint": "https://localhost:8443",
  "user": "testuser",
  "password": "testpass",
  "retry_attempts": 3,
  "tls": {
    "cert": {
      "ca": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
    }
  }
}
```

### Fields

- **endpoint** (required): WebDAV server URL
- **user** (optional): Basic auth username
- **password** (optional): Basic auth password
- **retry_attempts** (optional): Number of retry attempts (default: 3)
- **tls.cert.ca** (optional): CA certificate for TLS verification

### Without TLS (HTTP only)

```json
{
  "endpoint": "http://localhost:8080",
  "user": "testuser",
  "password": "testpass"
}
```

## WebDAV Server Access

Once the server is running, you can also access it via:

### Command line (curl)

```bash
# List files
curl -k -u testuser:testpass https://localhost:8443/

# Upload file
curl -k -u testuser:testpass -T test.txt https://localhost:8443/test.txt

# Download file
curl -k -u testuser:testpass https://localhost:8443/test.txt -o downloaded.txt

# Delete file
curl -k -u testuser:testpass -X DELETE https://localhost:8443/test.txt
```

### Browser

Navigate to: https://localhost:8443
- Username: testuser
- Password: testpass
- Accept the self-signed certificate warning

### WebDAV Client

Use any WebDAV client (macOS Finder, Windows Explorer, etc.):
- URL: https://localhost:8443
- Username: testuser
- Password: testpass

## Troubleshooting

### WebDAV server not starting

```bash
cd dav/webdav-test
docker-compose logs
```

### Certificate issues

If you get certificate errors, regenerate certificates:

```bash
cd dav/webdav-test
rm -rf certs/*
cd ..
./setup-webdav-test.sh
```

### Connection refused

Check if the server is running:

```bash
docker ps | grep webdav-test
curl -k https://localhost:8443
```

### Permission denied

Check file permissions:

```bash
ls -la dav/webdav-test/data/
```

The WebDAV server runs as user `daemon`, ensure files are accessible.

## Cleanup

### Stop WebDAV server

```bash
cd dav/webdav-test
docker-compose down
```

### Remove all test files

```bash
cd dav
rm -rf webdav-test config.json
cd ..
rm -f storage-cli test-file.txt downloaded-file.txt
```

## Integration with CI/CD

The test script can be used in CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Setup WebDAV
  run: |
    cd storage-cli/dav
    ./setup-webdav-test.sh

- name: Test DAV
  run: |
    cd storage-cli/dav
    ./test-storage-cli.sh
```

## Expected Results

All operations should complete successfully with appropriate output:

```
=== Testing storage-cli DAV Implementation ===
1. Building storage-cli...
✓ Built storage-cli
✓ WebDAV server is running
2. Generating config.json with CA certificate...
✓ Generated config.json
3. Creating test file...
✓ Created test-file.txt
4. Testing PUT operation...
✓ PUT successful
5. Testing EXISTS operation...
✓ EXISTS successful (blob found)
...
=== All Tests Passed! ✓ ===
```

## Notes

- The test WebDAV server uses self-signed certificates for testing only
- For production, use proper CA-signed certificates
- The server data persists in `webdav-test/data/`
- All blob operations use SHA1-based prefix paths (e.g., `/0c/blob-id`)
- WebDAV server supports all standard DAV methods (GET, PUT, DELETE, PROPFIND, MKCOL)
