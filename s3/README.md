# S3 Client

S3 client implementation for the unified storage-cli tool. This module provides S3-compatible blobstore operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s s3` flag to access S3 functionality.

For general usage and build instructions, see the [main README](../README.md).

## S3-Specific Configuration

The S3 client requires a JSON configuration file with the following structure:

``` json
{
  "bucket_name":            "<string> (required)",

  "credentials_source":     "<string> [static|env_or_profile|none]",
  "access_key_id":          "<string> (required if credentials_source = 'static')",
  "secret_access_key":      "<string> (required if credentials_source = 'static')",

  "region":                 "<string> (optional - default: 'us-east-1')",
  "host":                   "<string> (optional)",
  "port":                   <int> (optional),

  "ssl_verify_peer":        <bool> (optional),
  "use_ssl":                <bool> (optional),
  "signature_version":      "<string> (optional)",
  "server_side_encryption": "<string> (optional)",
  "sse_kms_key_id":         "<string> (optional)",
  "multipart_upload":       <bool> (optional - default: true)
}
```

**Usage examples:**
```shell
# Upload a file to S3
storage-cli -s s3 -c s3-config.json put local-file.txt remote-object.txt

# Download a file from S3
storage-cli -s s3 -c s3-config.json get remote-object.txt downloaded-file.txt

# Check if an object exists
storage-cli -s s3 -c s3-config.json exists remote-object.txt

# List all objects
storage-cli -s s3 -c s3-config.json list

# Delete an object
storage-cli -s s3 -c s3-config.json delete remote-object.txt
```

## Testing

### Unit Tests
Run unit tests from the repository root:
```bash
ginkgo --skip-package=integration --cover -v -r ./s3/...
```

### Integration Tests

To run the integration tests, export the following variables into your environment:

```
export access_key_id=YOUR_AWS_ACCESS_KEY
export focus_regex="GENERAL AWS|AWS V2 REGION|AWS V4 REGION|AWS US-EAST-1"
export region_name=us-east-1
export s3_endpoint_host=https://s3.amazonaws.com
export secret_access_key=YOUR_SECRET_ACCESS_KEY
export stack_name=s3cli-iam
export bucket_name=s3cli-pipeline
```

Run `./.github/scripts/s3/setup-aws-infrastructure.sh` and `./.github/scripts/s3/teardown-infrastructure.sh` before and after the `./.github/scripts/s3/run-integration-*` in repo's root folder.
