/*
 * Copyright 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package integration

import (
	"os"

	. "github.com/onsi/gomega" //nolint:staticcheck
)

// NoLongEnv must be set in the environment
// to enable skipping long running tests
const NoLongEnv = "SKIP_LONG_TESTS"

// NoLongMsg is the template used when BucketNoLongEnv's environment variable
// has not been populated.
const NoLongMsg = "environment variable %s filled, skipping long test"

// AssertLifecycleWorks tests the main blobstore object lifecycle from
// creation to deletion.
//
// This is using gomega matchers, so it will fail if called outside an
// 'It' test.
func AssertLifecycleWorks(gcsCLIPath string, ctx AssertContext) {
	var storageType string = "gcs"
	session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", ctx.ContentFile, ctx.GCSFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "exists", ctx.GCSFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(session.Err.Contents()).To(MatchRegexp("File '.*' exists in bucket '.*'"))

	tmpLocalFileName := "gcscli-download"
	defer os.Remove(tmpLocalFileName) //nolint:errcheck

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "get", ctx.GCSFileName, tmpLocalFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	gottenBytes, err := os.ReadFile(tmpLocalFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gottenBytes)).To(Equal(ctx.ExpectedString))

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete", ctx.GCSFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "exists", ctx.GCSFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(Equal(3))
	Expect(session.Err.Contents()).To(MatchRegexp("File '.*' does not exist in bucket '.*'"))
}
