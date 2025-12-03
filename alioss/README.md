# Alibaba Cloud OSS Client

Alibaba Cloud OSS (Object Storage Service) client implementation for the unified storage-cli tool. This module provides Alibaba Cloud OSS operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s alioss` flag to access AliOSS functionality.

For general usage and build instructions, see the [main README](../README.md).

## AliOSS-Specific Configuration

The AliOSS client requires a JSON configuration file with the following structure:

``` json
{
  "access_key_id":             "<string> (required)",
  "access_key_secret":         "<string> (required)",
  "endpoint":                  "<string> (required)",
  "bucket_name":               "<string> (required)"
}
```

**Usage examples:**
``` bash
# Upload a blob
storage-cli -s alioss -c alioss-config.json put local-file.txt remote-blob

# Fetch a blob (destination file will be overwritten if exists)
storage-cli -s alioss -c alioss-config.json get remote-blob local-file.txt

# Delete a blob
storage-cli -s alioss -c alioss-config.json delete remote-blob

# Check if blob exists
storage-cli -s alioss -c alioss-config.json exists remote-blob

# Generate a signed URL (e.g., GET for 3600 seconds)
storage-cli -s alioss -c alioss-config.json sign remote-blob get 3600s
```

### Using Signed URLs with curl
``` bash
# Uploading a blob:
curl -X PUT -T path/to/file <signed-url>

# Downloading a blob:
curl -X GET <signed-url>
```

## Testing

### Unit Tests
Run from the repository root directory:

```bash
ginkgo --skip-package=integration --cover -v -r ./alioss/...
```
### Integration Tests

- To run the integration tests with your existing bucket:
  1. Export the following variables into your environment:
      ``` bash
      export ACCESS_KEY_ID=<your Alibaba access key id>
      export ACCESS_KEY_SECRET=<your Alibaba access key secret>
      export ENDPOINT=<your Alibaba OSS endpoint>
      export BUCKET_NAME=<your Alibaba OSS bucket>
      ```
  1. Navigate to project's root folder and run the command below:
      ``` bash
      go build -o ./alioss  ./alioss && go test ./alioss/integration/...
      ```
- To run it from scratch; create a new bucket, run tests, delete the bucket
  1. Create a user in your ali account and add policy `AliyunOSSFullAccess`, or restrict the users with more granular policies like `oss:CreateBucket`, `oss:DeleteBucket` etc.
  1. Create access key for the user.
  1. Export `AccessKeyId` with command `export access_key_id=<AccessKeyId>`.
  1. Export `AccessKeySecret` with command `export access_key_secret=<AccessKeySecret>`.
  1. Navigate to project's root folder.
  1. Run environment setup script to create bucket `./.github/scripts/alioss/setup.sh`.
  1. Run tests `./.github/scripts/alioss/run-int.sh`.
  1. Run environment teardown script to delete test resources `./.github/scripts/alioss/teardown.sh`.
