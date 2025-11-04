# Storage CLI
This repository consolidates five independent blob-storage CLIs, one per provider, into a single codebase. Each provider lives in its own directory (e.g., azure/, aws/, gcs/, ali/, dav/) with its own main package and code base. The each clis expose similar high-level operations (e.g., put, get, delete) but are intentionally separate binaries, preserving each provider’s native SDK, flags, and behaviors.

Key points

- Each provider builds independently.

- Client setup, config, and options are contained within the provider’s folder.

- All tools support the same core commands (such as put, get, and delete) for a familiar workflow, while each provider defines its own flags, parameters, and execution flow that align with its native SDK and terminology.

- Central issue tracking, shared CI, and aligned release process without merging implementations.


## Providers
- [Ali](./ali/README.md)
- [Azure](./azure/README.md)
- [Dav](./dav/README.md)
- [GCS](./gcs/README.md)
- [S3](./s3/README.md)


## Build
Use following command to build it locally

```shell
go build -o <provider-folder-name>/<build-name> <provider-folder-name>/main.go  
```
e.g. `go build -o ali/storage-cli-ali ali/main.go`


## Notes
These commit IDs represent the last migration checkpoint from each provider's original repository, marking the final commit that was copied during the consolidation process.

- ali   -> c303a62679ff467ba5012cc1a7ecfb7b6be47ea0
- azure -> 18667d2a0b5237c38d053238906b4500cfb82ce8
- dav   -> c64e57857539d0173d46e79093c2e998ec71ab63
- gcs   -> d4ab2040f37415a559942feb7e264c6b28950f77
- s3    -> 7ac9468ba8567eaf79828f30007c5a44066ef50f