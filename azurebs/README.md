# Azure Storage CLI

The Azure Storage CLI is for uploading, fetching and deleting content to and from an Azure blobstore.
It is highly inspired by the [storage-cli/s3](https://github.com/cloudfoundry/storage-cli/blob/6058f516e9b81471b64a50b01e228158a05731f0/s3)

## Usage

Given a JSON config file (`config.json`)...

``` json
{
  "account_name":           "<string> (required)",
  "account_key":            "<string> (required)",
  "container_name":         "<string> (required)",
  "environment":            "<string> (optional, default: 'AzureCloud')",
}
```

``` bash
# Command: "put"
# Upload a blob to the blobstore.
./azurebs-cli -c config.json put <path/to/file> <remote-blob> 

# Command: "get"
# Fetch a blob from the blobstore.
# Destination file will be overwritten if exists.
./azurebs-cli -c config.json get <remote-blob> <path/to/file>

# Command: "delete"
# Remove a blob from the blobstore.
./azurebs-cli -c config.json delete <remote-blob>

# Command: "exists"
# Checks if blob exists in the blobstore.
./azurebs-cli -c config.json exists <remote-blob>

# Command: "sign"
# Create a self-signed url for a blob in the blobstore.
./azurebs-cli -c config.json sign <remote-blob> <get|put> <seconds-to-expiration>
```

### Using signed urls with curl

``` bash
# Uploading a blob:
curl -X PUT -H "x-ms-blob-type: blockblob" -F 'fileX=<path/to/file>' <signed url>

# Downloading a blob:
curl -X GET <signed url>
```

## Running Tests

### Unit Tests
**Note:** Run the following commands from the repository root directory

- Using ginkgo:

  ``` bash
  go install github.com/onsi/ginkgo/v2/ginkgo

  ginkgo --skip-package=integration --randomize-all --cover -v -r ./azurebs/...
  ```

- Using go test:

  ``` bash
  go test $(go list ./azurebs/... | grep -v integration)
  ```

### Integration Tests
- To run the integration tests with your existing container
  1. Export the following variables into your environment.
  
      ```bash
      export ACCOUNT_NAME=<your Azure accounnt name>
      export ACCOUNT_KEY=<your Azure account key>
      export CONTAINER_NAME=<the target container name>
      ```
      
  1. Navigate to project's root folder and run the command below:

      ```bash
      go test ./azurebs/integration/...
      ```

- To run it from scratch; create a new container, run tests, delete the container
  1. Create a storage account in your azure subscription.
  1. Get `account name` and `access key` from you storage account.
  1. Export `account name` with command `export azure_storage_account=<account name>`.
  1. Export `access key` with command `export azure_storage_key=<access key>`.
  1. Navigate to project's root folder.
  1. Run environment setup script to create container `./.github/scripts/azurebs/setup.sh`.
  1. Run tests `./.github/scripts/azurebs/run-int.sh`.
  1. Run environment teardown script to delete test resources `./.github/scripts/azurebs/teardown.sh`.
