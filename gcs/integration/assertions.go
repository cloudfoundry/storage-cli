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
	"fmt"
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
	storageType := "gcs"
	session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", ctx.ContentFile, ctx.GCSFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "exists", ctx.GCSFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(session.Err.Contents()).To(MatchRegexp("File '.*' exists in bucket '.*'"))

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "properties", ctx.GCSFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	output := string(session.Out.Contents())
	Expect(output).To(MatchRegexp(`"etag":\s*".+?"`))
	Expect(output).To(MatchRegexp(`"last_modified":\s*".+?"`))
	Expect(output).To(MatchRegexp(`"content_length":\s*\d+`))

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

func AssertDeleteRecursiveWithPrefixLifecycle(gcsCLIPath string, ctx AssertContext) {
	storageType := "gcs"

	fileName1 := MakeContentFile(GenerateRandomString())
	fileName2 := MakeContentFile(GenerateRandomString())
	fileName3 := MakeContentFile(GenerateRandomString())
	prefix := fmt.Sprintf("%s-%s/", "test-prefix-delete-recursive", GenerateRandomString(10))
	dstObject1 := fmt.Sprintf("%s%s", prefix, GenerateRandomString())
	dstObject2 := fmt.Sprintf("%s%s", prefix, GenerateRandomString())
	dstObject3 := GenerateRandomString()
	defer os.Remove(fileName1) //nolint:errcheck
	defer os.Remove(fileName2) //nolint:errcheck
	defer os.Remove(fileName3) //nolint:errcheck

	session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", fileName1, dstObject1)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", fileName2, dstObject2)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", fileName3, dstObject3)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete-recursive", prefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "exists", dstObject3)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "exists", dstObject1)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(Equal(3))
	Expect(session.Err.Contents()).To(MatchRegexp("File '.*' does not exist in bucket '.*'"))

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "exists", dstObject1)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(Equal(3))
	Expect(session.Err.Contents()).To(MatchRegexp("File '.*' does not exist in bucket '.*'"))

	//cleanup artifact
	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete", dstObject3)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

}

func AssertCopyLifecycle(gcsCLIPath string, ctx AssertContext) {
	storageType := "gcs"

	dstNameToPut := GenerateRandomString()
	dstNameToCopy := GenerateRandomString()

	session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", ctx.ContentFile, dstNameToPut)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "copy", dstNameToPut, dstNameToCopy)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	tmpFileName := "copy-lifecycle"
	defer os.Remove(tmpFileName) //nolint:errcheck
	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "get", dstNameToCopy, tmpFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	contentGet, err := os.ReadFile(tmpFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(contentGet)).To(Equal(ctx.ExpectedString))

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete", dstNameToPut)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete", dstNameToCopy)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
}

func AssertListMultipleWithPrefixLifecycle(gcsCLIPath string, ctx AssertContext) {
	storageType := "gcs"
	fileName1 := MakeContentFile(GenerateRandomString())
	fileName2 := MakeContentFile(GenerateRandomString())
	fileName3 := MakeContentFile(GenerateRandomString())
	prefix := fmt.Sprintf("%s-%s/", "test-prefix-list", GenerateRandomString(10))
	dstObject1 := fmt.Sprintf("%s%s", prefix, GenerateRandomString())
	dstObject2 := fmt.Sprintf("%s%s", prefix, GenerateRandomString())
	dstObject3 := GenerateRandomString()
	defer os.Remove(fileName1) //nolint:errcheck
	defer os.Remove(fileName2) //nolint:errcheck
	defer os.Remove(fileName3) //nolint:errcheck

	session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", fileName1, dstObject1)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", fileName2, dstObject2)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "put", fileName3, dstObject3)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "list", prefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	objs := string(session.Out.Contents())
	Expect(objs).To(ContainSubstring(dstObject1))
	Expect(objs).To(ContainSubstring(dstObject2))
	Expect(objs).ToNot(ContainSubstring(dstObject3))

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete", dstObject1)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete", dstObject2)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "delete", dstObject3)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
}
