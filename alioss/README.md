# Ali Storage CLI

The Ali Storage CLI is for uploading, fetching and deleting content to and from an Ali OSS.
It is highly inspired by the [storage-cli/s3](https://github.com/cloudfoundry/storage-cli/blob/6058f516e9b81471b64a50b01e228158a05731f0/s3)

## Usage

Given a JSON config file (`config.json`)...

``` json
{
  "access_key_id":             "<string> (required)",
  "access_key_secret":         "<string> (required)",
  "endpoint":                  "<string> (required)",
  "bucket_name":               "<string> (required)"
}
```

``` bash
# Command: "put"
# Upload a blob to the blobstore.
./alioss-cli -c config.json put <path/to/file> <remote-blob>

# Command: "get"
# Fetch a blob from the blobstore.
# Destination file will be overwritten if exists.
./alioss-cli -c config.json get <remote-blob> <path/to/file>

# Command: "delete"
# Remove a blob from the blobstore.
./alioss-cli -c config.json delete <remote-blob>

# Command: "exists"
# Checks if blob exists in the blobstore.
./alioss-cli -c config.json exists <remote-blob>

# Command: "sign"
# Create a self-signed url for a blob in the blobstore.
./alioss-cli -c config.json sign <remote-blob> <get|put> <seconds-to-expiration>
```

### Using signed urls with curl
``` bash
# Uploading a blob:
curl -X PUT -T path/to/file <signed url>

# Downloading a blob:
curl -X GET <signed url>
```
## Running Tests

### Unit Tests
```bash
go install github.com/onsi/ginkgo/v2/ginkgo
ginkgo --skip-package=integration --randomize-all --cover -v -r ./alioss/...
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
  1. go build -o ./alioss  ./alioss && go test ./alioss/integration/...

- To run it from scratch; create a new bucket, run tests, delete the bucket
  1. Create a user in your ali account and add policy `AliyunOSSFullAccess` or restrict the users with more granular policies like `oss:CreateBucket, oss:DeleteBucket` etc.
  1. Create access key for the user.
  1. Export `AccessKeyId` with command `export access_key_id=<AccessKeyId>`.
  1. Export `AccessKeySecret` with command `export access_key_secret=<AccessKeyId>`.
  1. Navigate to project's root folder.
  1. Run environment setup script to create container `/.github/scripts/alioss/setup.sh`.
  1. Run tests `/.github/scripts/alioss/run-int.sh`.
  1. Run environment teardown script to delete test resources `/.github/scripts/alioss/teardown.sh`.
