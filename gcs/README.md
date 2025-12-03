# GCS Client

GCS (Google Cloud Storage) client implementation for the unified storage-cli tool. This module provides Google Cloud Storage operations through the main storage-cli binary.

**Note:** This is not a standalone CLI. Use the main `storage-cli` binary with `-s gcs` flag to access GCS functionality.

For general usage and build instructions, see the [main README](../README.md).

This is **not** an official Google Product.

## GCS-Specific Configuration

The GCS client requires a JSON configuration file.

### Authentication Methods (`credentials_source`)
* `static`: A [service account](https://cloud.google.com/iam/docs/creating-managing-service-account-keys) key will be provided via the `json_key` field.
* `none`: No credentials are provided. The client is reading from a public bucket.
* &lt;empty&gt;: [Application Default Credentials](https://developers.google.com/identity/protocols/application-default-credentials)
  will be used if they exist (either through `gcloud auth application-default login` or a [service account](https://cloud.google.com/iam/docs/understanding-service-accounts)).
  If they don't exist the client will fall back to `none` behavior.

**Usage examples:**
```bash
# Upload an object
storage-cli -s gcs -c gcs-config.json put local-file.txt remote-blob

# Fetch an object
storage-cli -s gcs -c gcs-config.json get remote-blob local-file.txt

# Delete an object
storage-cli -s gcs -c gcs-config.json delete remote-blob

# Check if an object exists
storage-cli -s gcs -c gcs-config.json exists remote-blob

# Generate a signed URL (e.g., GET for 1 hour)
storage-cli -s gcs -c gcs-config.json sign remote-blob get 60s
```


## Testing

### Unit Tests
Run unit tests from the repository root:

```bash
ginkgo --skip-package=integration --cover -v -r ./gcs/...
```

### Integration Tests
1. Create a service account with the `Storage Admin` role.
1. Create a new key for your service account and download credential as JSON file.
1. Export json content with `export google_json_key_data="$(cat <path-to-json-file.json>)"`.
1. Export `export SKIP_LONG_TESTS=yes` if you want to run only the fast running tests.
1. Navigate to project's root folder.
1. Run environment setup script to create buckets `/.github/scripts/gcs/setup.sh`.
1. Run tests `/.github/scripts/gcs/run-int.sh`.
1. Run environment teardown script to delete resources `/.github/scripts/gcs/teardown.sh`.