# S3 Integration Tests

This folder contains shell scripts used by `.github/workflows/s3-integration.yml` and the composite actions in `.github/actions/`.

## Test Scenarios

### Test with Static Credentials

- Script: `run-integration-aws.sh`
- Triggered by: `s3-integration-run` action with `test_type: aws`
- Input shape: CloudFormation stack (`stack_name`: `s3cli-iam`, `s3cli-private-bucket`, or `s3cli-public-bucket`) + credentials + `region_name`
- Behavior: verifies core storage-cli S3 operations (create/check bucket, upload/download, list, delete) for a user authenticating with standard AWS access keys in the selected region and endpoint.

### Test IAM Roles

- Script: `run-integration-aws-iam.sh`
- Triggered by: `s3-integration-run` action with `test_type: aws-iam`
- Input shape: CloudFormation stack (`stack_name`: `s3cli-iam`) outputs + IAM role ARN from the stack
- Behavior: verifies that storage-cli works without static keys when permissions are provided by an IAM role at runtime, matching role-based production usage. Note that the policy grants access permission to the "Lambda" service, because this is used as test execution environment. In a productive setup, the principal is typically the "EC2" service. See [Storage CLI with AWS IAM Instance Profiles](https://docs.cloudfoundry.org/deploying/common/cc-blobstore-config.html#storage-cli-aws-iam) for more details.

### Test Assume Roles

- Script: `run-integration-aws-assume.sh`
- Triggered by: `s3-integration-run` action with `test_type: aws-assume`
- Input shape: CloudFormation stack (`stack_name`: `s3cli-iam`) + base credentials
- Behavior: verifies that storage-cli can access S3 through cross-account or delegated-role credentials (STS assume role), rather than direct long-lived credentials.

### Test S3 Compatible

- Script: `run-integration-s3-compat.sh`
- Triggered by: `s3-compatible-integration` job in workflow
- Input shape: no CloudFormation stack (pre-existing bucket) + HMAC key pair + bucket + endpoint host/port
- Behavior: verifies that storage-cli S3 commands work against non-AWS S3-compatible providers (for example GCS interoperability mode), not only native AWS S3.

## CloudFormation assets

The CloudFormation templates stored in `assets/` are used to create AWS infrastructure resources for the integration tests.

### `cloudformation-s3cli-iam.template.json` (`stack_name: s3cli-iam`)

- Used by workflow job(s): `aws-s3-us-integration` (steps `Setup AWS infrastructure`, `Teardown AWS infrastructure`).
- Creates one private S3 bucket (`AWS::S3::Bucket`).
- Creates one IAM role (`AWS::IAM::Role`) assumable by Lambda (so the integration test that is run in the Lambda context has permissions to access the S3 bucket).
- Attaches inline policy permissions for CloudWatch Logs and S3 bucket/object operations used by IAM-role integration tests.

### `cloudformation-s3cli-private-bucket.template.json` (`stack_name: s3cli-private-bucket`)

- Used by workflow job(s): `aws-s3-regional-integration` matrix entries `Frankfurt` and `European Sovereign Cloud`.
- Creates one private S3 bucket (`AWS::S3::Bucket`).

### `cloudformation-s3cli-public-bucket.template.json` (`stack_name: s3cli-public-bucket`)

- Used by workflow job(s): `aws-s3-regional-integration` matrix entry `Public Read`.
- Creates one S3 bucket (`AWS::S3::Bucket`) with public-access blocks disabled.
