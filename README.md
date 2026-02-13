# Storage CLI

[![Unit Tests](https://github.com/cloudfoundry/storage-cli/actions/workflows/unit-test.yml/badge.svg?branch=main)](https://github.com/cloudfoundry/storage-cli/actions/workflows/unit-test.yml)
[![Build](https://github.com/cloudfoundry/storage-cli/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/cloudfoundry/storage-cli/actions/workflows/build.yml)
[![S3 Integration Tests](https://github.com/cloudfoundry/storage-cli/actions/workflows/s3-integration.yml/badge.svg?branch=main)](https://github.com/cloudfoundry/storage-cli/actions/workflows/s3-integration.yml)
[![GCS Integration Tests](https://github.com/cloudfoundry/storage-cli/actions/workflows/gcs-integration.yml/badge.svg?branch=main)](https://github.com/cloudfoundry/storage-cli/actions/workflows/gcs-integration.yml)
[![Azure Integration Tests](https://github.com/cloudfoundry/storage-cli/actions/workflows/azurebs-integration.yml/badge.svg?branch=main)](https://github.com/cloudfoundry/storage-cli/actions/workflows/azurebs-integration.yml)
[![Alioss Integration Tests](https://github.com/cloudfoundry/storage-cli/actions/workflows/alioss-integration.yml/badge.svg?branch=main)](https://github.com/cloudfoundry/storage-cli/actions/workflows/alioss-integration.yml)

A unified command-line tool for interacting with multiple cloud storage providers through a single binary. The CLI supports five blob-storage providers (Azure Blob Storage, AWS S3, Google Cloud Storage, Alibaba Cloud OSS, and WebDAV), each with its own client implementation while sharing a common command interface.

**Note:** This CLI works with existing storage resources (buckets, containers, etc.) that are already created and configured in your cloud provider. The storage bucket/container name and credentials must be specified in the provider-specific configuration file.

Key points

- Single binary with provider selection via `-s` flag.

- Each provider has its own directory (azurebs/, s3/, gcs/, alioss/, dav/) containing client implementations and configurations.

- All providers support the same core commands (put, get, delete, exists, list, copy, etc.).

- Provider-specific configurations are passed via JSON config files.


## Providers
- [Alioss](./alioss/README.md)
- [Azurebs](./azurebs/README.md)
- [Dav](./dav/README.md)
  - additional endpoints needed by CAPI still missing
- [Gcs](./gcs/README.md)
- [S3](./s3/README.md)


## Build
Build the unified storage CLI binary:

```shell
go build -o storage-cli
```

Or with version information:
```shell
go build -ldflags "-X main.version=1.0.0" -o storage-cli
```

## Usage

The CLI uses a unified command structure across all providers:

```shell
storage-cli -s <provider> -c <config-file> <command> [arguments]
```

**Flags:**
- `-s`: Storage provider type (azurebs|s3|gcs|alioss|dav)
- `-c`: Path to provider-specific configuration file
- `-v`: Show version
- `-log-file`: Path to log file (optional, logs to stderr by default)
- `-log-level`: Logging level: debug, info, warn, error (default: warn)

**Common commands:**
- `put <path/to/file> <remote-object>` - Upload a local file to remote storage
- `get <remote-object> <path/to/file>` - Download a remote object to local file  
- `delete <remote-object>` - Delete a remote object
- `delete-recursive [prefix]` - Delete objects recursively. If prefix is omitted, deletes all objects
- `exists <remote-object>` - Check if a remote object exists (exits with code 3 if not found)
- `list [prefix]` - List remote objects. If prefix is omitted, lists all objects
- `copy <source-object> <destination-object>` - Copy object within the same storage
- `sign <object> <action> <duration_as_second>` - Generate signed URL (action: get|put, duration: e.g., 60s)
- `properties <remote-object>` - Display properties/metadata of a remote object
- `ensure-storage-exists` - Ensure the storage container/bucket exists, if not create the storage(bucket,container etc)

**Examples:**
```shell
# Upload file to S3
storage-cli -s s3 -c s3-config.json put local-file.txt remote-object.txt

# List GCS objects with prefix
storage-cli -s gcs -c gcs-config.json list my-prefix

# Check if Azure blob exists
storage-cli -s azurebs -c azure-config.json exists my-blob.txt

# Get properties of an object
storage-cli -s azurebs -c azure-config.json properties my-blob.txt

# Sign object for 'get' in alioss for 60 seconds
storage-cli -s alioss -c alioss-config.json sign object.txt get 60s

# Upload file with debug logging to file
storage-cli -s s3 -c s3-config.json -log-level debug -log-file storage.log put local-file.txt remote-object.txt

# List objects with error-level logging only
storage-cli -s gcs -c gcs-config.json -log-level error list my-prefix
```

## Contributing

Follow these steps to make a contribution to the project:

- Fork this repository
- Create a feature branch based upon the `main` branch (*pull requests must be made against this branch*)
  ``` bash
  git checkout -b feature-name origin/main
  ```
- Run tests to check your development environment setup
  ``` bash
  ginkgo --race --skip-package=integration --cover -v -r ./...
  ```
- Make your changes (*be sure to add/update tests*)
- Run tests to check your changes
  ``` bash
  ginkgo --race --skip-package=integration --cover -v -r ./...
  ```
- If you added or modified integration tests, to run them locally, follow the instructions in the provider-specific README (see [Providers](#providers) section)
- **Note:** Integration tests require access to cloud provider credentials and cannot run on PRs from forks. They will run automatically when a maintainer merges your PR to main.
- Push changes to your fork
  ``` bash
  git add .
  git commit -m "Commit message"
  git push origin feature-name
  ```
- Create a GitHub pull request, selecting `main` as the target branch

## Dependency Updates

This project uses [Dependabot](https://docs.github.com/en/code-security/dependabot) to keep dependencies up to date. Dependencies are grouped (e.g., AWS SDK, Azure SDK, Google Cloud, testing tools) to reduce PR noise.

**Integration tests on Dependabot PRs:** Integration tests are skipped for Dependabot PRs since they don't have access to secrets. If needed, maintainers can manually trigger the integration tests via `workflow_dispatch` before merging. Integration tests will also run automatically after merging to main.

## Releases

### Manual Release
Releases must be triggered manually by an approver. This can be done either via `GitHub Actions` (workflow dispatch) or through the `GitHub Releases` page using the **Draft a new release** option. The *Release Manual* workflow is responsible for creating and completing the release.

Option 1: Release via Workflow Dispatch

- Go to Actions and select the *Release Manual* workflow.

- Click **Run workflow**.

- Enter the next incremented version number with the v prefix (for example, v1.2.3).

- The workflow will create the release and upload the build artifacts once completed.

Option 2: Release via Draft Release

- Go to Releases and click **Draft a new release**.

- Create a new tag using the next incremented version with the v prefix.

- Fill in the release title and description.

- Click Publish release.

- The release will appear immediately on the Releases page. This action will also trigger the *Release Manual* workflow, which will build the artifacts and upload them to the published release once the workflow finishes.


## Notes
These commit IDs represent the last migration checkpoint from each provider's original repository, marking the final commit that was copied during the consolidation process.

- alioss   -> c303a62679ff467ba5012cc1a7ecfb7b6be47ea0
- azurebs -> 18667d2a0b5237c38d053238906b4500cfb82ce8
- dav   -> c64e57857539d0173d46e79093c2e998ec71ab63
- gcs   -> d4ab2040f37415a559942feb7e264c6b28950f77
- s3    -> 39c1e2a988e3ec0026038cba041d0c2f1fb7dc93