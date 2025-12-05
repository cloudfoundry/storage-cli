# Azure Blob Storage Client

Azure Blob Storage client implementation for the unified storage-cli tool. This module provides Azure Blob Storage operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s azurebs` flag to access Azure Blob Storage functionality.

For general usage and build instructions, see the [main README](../README.md).

## Azure-Specific Configuration

The Azure client requires a JSON configuration file with the following structure:

``` json
{
  "account_name":           "<string> (required)",
  "account_key":            "<string> (required)",
  "container_name":         "<string> (required)",
  "environment":            "<string> (optional, default: 'AzureCloud')"
}
```

**Usage examples:**
``` bash
# Upload a blob
storage-cli -s azurebs -c azure-config.json put local-file.txt remote-blob

# Fetch a blob (destination file will be overwritten if exists)
storage-cli -s azurebs -c azure-config.json get remote-blob local-file.txt

# Delete a blob
storage-cli -s azurebs -c azure-config.json delete remote-blob

# Check if blob exists
storage-cli -s azurebs -c azure-config.json exists remote-blob

# Generate a signed URL (e.g., GET for 3600 seconds)
storage-cli -s azurebs -c azure-config.json sign remote-blob get 3600s
```

### Using Signed URLs with curl

``` bash
# Uploading a blob:
curl -X PUT -H "x-ms-blob-type: blockblob" -F 'fileX=<path/to/file>' <signed-url>

# Downloading a blob:
curl -X GET <signed-url>
```

## Testing

### Unit Tests
Run from the repository root directory:

```bash
ginkgo --skip-package=integration --randomize-all --cover -v -r ./azurebs/...
```

Or using go test:
```bash
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
