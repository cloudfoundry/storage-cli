# Benchmarker
Standalone benchmark harness for storage-cli. Measures wall-clock transfer performance of the CLI binary against real provider buckets.

## How it works
The harness invokes the real `storage-cli` binary as a subprocess for every `put` and `get`, measures elapsed wall-clock time, and verifies downloaded content against a SHA-256 computed before the timer starts. This exercises flags, config parsing, client construction, SDK behaviour, and exit status at exactly the CLI boundary used by Cloud Controller.

Failed or timed-out samples are retained in the output and mark the run as failed; they never appear as successful timing samples.

## Test Profiles
| Profile | Object sizes | Samples | When to use |
|---------|-------------|---------|-------------|
| `quick` | 1 KiB, 1 MiB, 10 MiB, 128 MiB | 3 measured round trips per size | Fast sanity check on relevant changes — covers latency-dominated (1 KiB) through large-object (128 MiB) transfers without a warm-up |
| `full`  | 1 KiB, 1 MiB, 10 MiB, 128 MiB, 1 GiB | 5 measured round trips per size | Weekly runs, pre-release validation, and investigations — the extra 1 GiB size exercises transfers well beyond provider multipart/chunk thresholds (Azure 32 MiB, GCS 100 MiB, S3 5 MiB parts) |

## Usage
```sh
# Build the harness
go build -tags benchmark -o benchmarker ./benchmark

# Quick run against AWS S3
./benchmarker \
  -candidate ./storage-cli-linux-amd64 \
  -provider  s3 \
  -config    /path/to/config-s3.json \
  -profile   quick \
  -output    ./results/s3

# Full run with baseline comparison against AWS ceph
./benchmarker \
  -candidate ./storage-cli-linux-amd64 \
  -baseline  ./storage-cli-1.0.12-linux-amd64 \
  -provider  s3 \
  -config    /path/to/config-s3.json \
  -backend-label ceph \
  -profile   full \
  -timeout   20m \
  -namespace benchmark/my-run \
  -output    ./results/ceph
```

## Flags
| Flag | Default | Description |
|------|---------|-------------|
| `-candidate` | *(required)* | Path to the CLI binary under test |
| `-baseline` | *(none)* | Path to a previous release binary for comparison |
| `-provider` | *(required)* | CLI storage selector: `s3`, `gcs`, `azurebs`, `alioss` |
| `-config` | *(required)* | Path to provider JSON configuration file |
| `-backend-label` | *(same as -provider)* | Label used in reports to identify the actual storage backend. Defaults to the provider name. Set explicitly when the backend differs, e.g. `-provider s3 -backend-label ceph` |
| `-profile` | `quick` | `quick` or `full` |
| `-timeout` | `10m` | Per-operation timeout applied to each put and get. Must include a unit, e.g. `30s`, `10m`, `1h` |
| `-namespace` | `benchmark` | Top-level prefix for every object key written to the bucket, e.g. `benchmark/1731.../s3/candidate/1KiB/sample-0`. Used to group benchmark objects so bucket lifecycle rules and cleanup stay scoped away from other data in the same bucket. |
| `-run-id` | *(timestamp)* | Unique identifier embedded in all object keys |
| `-output` | `.` | Directory for output files |

## Output Files
| File | Contents |
|------|----------|
| `results.json` | Raw per-sample timings, statuses, and metadata |
| `summary.md` | Markdown table with median latency and throughput; suitable for GitHub job summary |

## Configuration Files
Provider configuration is read from the JSON file specified by `-config`. For the different providers, check the provider's specific README.md, e.g. s3/README.md

Example structure for S3:

```json
{
  "access_key_id": "...",
  "secret_access_key": "...",
  "bucket_name": "my-benchmark-bucket",
  "region": "eu-central-1"
}
```

## Infrastructure Requirements
- Use a dedicated benchmark bucket or container, not a production or shared integration bucket. The harness deletes objects it creates, but lifecycle expiration should be configured as a backstop for interrupted runs.
- The IAM/service-account used for benchmarking needs only object-level read/write/delete on the dedicated bucket; no bucket-management permissions are required.
- Object keys are namespaced as `<namespace>/<run-id>/<provider>/<label>/<size>/<sample>`. The harness never calls `delete-recursive` against shared prefixes.

## Running Unit Tests
```sh
go test -tags benchmark ./benchmarks/
```
The tests build a fake CLI binary that stores objects locally and exercise invocation, integrity verification, failure modes, cleanup isolation, and all report formats without contacting any cloud provider.

## Editor Setup
The `//go:build benchmark` tag excludes these files from normal builds, which
means gopls will show a "no packages found" warning in your editor. Fix it by
adding the tag to your gopls build flags in VS Code settings:

```json
"gopls": {
    "buildFlags": ["-tags=benchmark"]
}
```
