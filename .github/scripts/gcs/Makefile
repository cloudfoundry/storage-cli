# Copyright 2017 Google Inc.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#    http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
SHELL = bash
TMP_DIR := /tmp/storage-cli-gcs-$(or $(GITHUB_RUN_ID),$(USER))
default: test-int

# Ensure tmp directory exists
create-tmp-dir:
	@mkdir -p "$(TMP_DIR)"

# Generate a $StorageClass.lock which contains our bucket name
# used for testing. Buckets must be unique among all in GCS,
# we cannot simply hardcode a bucket.
.PHONY: FORCE
regional.lock: create-tmp-dir
	@test -s "$(TMP_DIR)/regional.lock" || \
	echo "gcs-$$(openssl rand -hex 20)" > "$(TMP_DIR)/regional.lock"

# Create a bucket using the name located in $StorageClass.lock with
# a sane location.
regional-bucket: regional.lock
	@gsutil ls | grep -q "$$(cat $(TMP_DIR)/regional.lock)"; if [ $$? -ne 0 ]; then \
		gsutil mb -c REGIONAL -l us-east1 "gs://$$(cat $(TMP_DIR)/regional.lock)"; \
	fi

.PHONY: FORCE
multiregional.lock: create-tmp-dir
	@test -s "$(TMP_DIR)/multiregional.lock" || \
	echo "gcs-$$(openssl rand -hex 20)" > "$(TMP_DIR)/multiregional.lock"

multiregional-bucket: multiregional.lock
	@gsutil ls | grep -q "$$(cat $(TMP_DIR)/multiregional.lock)"; if [ $$? -ne 0 ]; then \
		gsutil mb -c MULTI_REGIONAL -l us "gs://$$(cat $(TMP_DIR)/multiregional.lock)"; \
	fi

.PHONY: FORCE
public.lock: create-tmp-dir
	@test -s "$(TMP_DIR)/public.lock" || \
	echo "gcs-$$(openssl rand -hex 20)" > "$(TMP_DIR)/public.lock"


public-bucket: public.lock
	@gsutil ls | grep -q "$$(cat $(TMP_DIR)/public.lock)"; if [ $$? -ne 0 ]; then \
		gsutil mb -c MULTI_REGIONAL -l us "gs://$$(cat $(TMP_DIR)/public.lock)" && \
		gsutil iam ch allUsers:legacyObjectReader "gs://$$(cat $(TMP_DIR)/public.lock)" && \
		gsutil iam ch allUsers:legacyBucketReader "gs://$$(cat $(TMP_DIR)/public.lock)" && \
		echo "waiting for IAM to propagate" && \
		until curl -s \
			"https://storage.googleapis.com/$$(cat $(TMP_DIR)/public.lock)/non-existent" \
			| grep -q "NoSuchKey"; do sleep 1; done; \
	fi

# Create all buckets necessary for the test.
prep-gcs: regional-bucket multiregional-bucket public-bucket

# Remove all buckets listed in $StorageClass.lock files.
clean-gcs:
	@test -s "$(TMP_DIR)/multiregional.lock" && test -s "$(TMP_DIR)/regional.lock" && test -s "$(TMP_DIR)/public.lock"
	@gsutil rm "gs://$$(cat $(TMP_DIR)/regional.lock)/*" || true
	@gsutil rb "gs://$$(cat $(TMP_DIR)/regional.lock)"
	@rm "$(TMP_DIR)/regional.lock"
	@gsutil rm "gs://$$(cat $(TMP_DIR)/multiregional.lock)/*" || true
	@gsutil rb "gs://$$(cat $(TMP_DIR)/multiregional.lock)"
	@rm "$(TMP_DIR)/multiregional.lock"
	@gsutil rm "gs://$$(cat $(TMP_DIR)/public.lock)/*" || true
	@gsutil rb "gs://$$(cat $(TMP_DIR)/public.lock)"
	@rm "$(TMP_DIR)/public.lock"
	@rmdir "$(TMP_DIR)" 2>/dev/null || true

# Perform only unit tests
test-unit:
	cd ../../.. && go run github.com/onsi/ginkgo/v2/ginkgo --skip-package=integration ./gcs/...

# Perform all tests, including integration tests.
test-int:
	 export MULTIREGIONAL_BUCKET_NAME="$$(cat $(TMP_DIR)/multiregional.lock)" && \
	 export REGIONAL_BUCKET_NAME="$$(cat $(TMP_DIR)/regional.lock)" && \
	 export PUBLIC_BUCKET_NAME="$$(cat $(TMP_DIR)/public.lock)" && \
	 export SKIP_LONG_TESTS="" && \
	 cd ../../.. && go run github.com/onsi/ginkgo/v2/ginkgo gcs/integration/

# Perform all non-long tests, including integration tests.
test-fast-int:
	 export MULTIREGIONAL_BUCKET_NAME="$$(cat $(TMP_DIR)/multiregional.lock)" && \
	 export REGIONAL_BUCKET_NAME="$$(cat $(TMP_DIR)/regional.lock)" && \
	 export PUBLIC_BUCKET_NAME="$$(cat $(TMP_DIR)/public.lock)" && \
	 export SKIP_LONG_TESTS="yes" && \
	 cd ../../.. && go run github.com/onsi/ginkgo/v2/ginkgo gcs/integration/

help:
	 @echo "  prep-gcs: create external GCS buckets needed for integration testing"
	 @echo "  clean-gcs: remove external GCS buckets"
	 @echo "  test-fast-int: run an reduced integration test suite (presubmit)"
	 @echo "  test-int: run the full integration test (CI only)"
	 @echo "  test-unit: run unit tests"
	 @echo ""
	 @echo "expected environment variables:"
	 @echo "  GOOGLE_SERVICE_ACCOUNT=contents of a JSON service account key"
