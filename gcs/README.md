# GCS Storage CLI
A Golang CLI for uploading, fetching and deleting content to/from [Google Cloud Storage](https://cloud.google.com/storage/). 
This tool exists to work with the [bosh-cli](https://github.com/cloudfoundry/bosh-cli) and [director](https://github.com/cloudfoundry/bosh).

This is **not** an official Google Product.


## Commands

### Usage
```bash
gcs-cli --help
```
### Upload an object
```bash
gcs-cli  -c config.json put <path/to/file> <remote-blob>
```
### Fetch an object
```bash
gcs-cli  -c config.json get <remote-blob> <path/to/file>
```
### Delete an object
```bash
gcs-cli  -c config.json delete <remote-blob>
```
### Check if an object exists
```bash
gcs-cli  -c config.json exists <remote-blob>
```

### Generate a signed url for an object
If there is an encryption key present in the config, then an additional header is sent

```bash
gcs-cli  -c config.json sign <remote-blob> <http action> <expiry>
```
Where:
 - `<http action>` is GET, PUT, or DELETE
 - `<expiry>` is a duration string less than 7 days (e.g. "6h")

## Configuration
The command line tool expects a JSON configuration file. Run `storage-cli-gcs --help` for details.

### Authentication Methods (`credentials_source`)
* `static`: A [service account](https://cloud.google.com/iam/docs/creating-managing-service-account-keys) key will be provided via the `json_key` field.
* `none`: No credentials are provided. The client is reading from a public bucket.
* &lt;empty&gt;: [Application Default Credentials](https://developers.google.com/identity/protocols/application-default-credentials)
  will be used if they exist (either through `gcloud auth application-default login` or a [service account](https://cloud.google.com/iam/docs/understanding-service-accounts)).
  If they don't exist the client will fall back to `none` behavior.

## Running Tests
## Unit Tests
1. Use the command `make -C .github/scripts/gcs test-unit`

## Integration Tests
1. Create a service account with the `Storage Admin` role.
1. Create a new key for your service account and download credential as JSON file.
1. Export json content with `export google_json_key_data="$(cat <path-to-json-file.json>)"`.
1. Export `export SKIP_LONG_TESTS=yes` if you want to run only the fast running tests.
1. Navigate to project's root folder.
1. Run environment setup script to create buckets `/.github/scripts/gcs/setup.sh`.
1. Run tests `/.github/scripts/gcs/run-int.sh`.
1. Run environment teardown script to delete resources `/.github/scripts/gcs/teardown.sh`.