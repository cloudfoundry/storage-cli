package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const testOpTimeout = 30 * time.Second

// testRunConfig returns a RunConfig wired for tests.
func testRunConfig(name string, profile Profile) RunConfig {
	return RunConfig{
		Provider:     "s3",
		BackendLabel: "s3",
		Profile:      profile,
		OpTimeout:    testOpTimeout,
		Namespace:    "test",
		RunID:        name,
	}
}

type mockStore struct {
	dir string
}

func newMockStore(dir string) *mockStore {
	return &mockStore{dir: dir}
}

func (s *mockStore) storePath(key string) string {
	parts := strings.Split(key, "/")
	return filepath.Join(append([]string{s.dir}, parts...)...)
}

// runner returns a CLIRunner that stores and retrieves objects locally.
func (s *mockStore) runner() CLIRunner {
	return func(_ context.Context, op OperationType, key, localFile string) (time.Duration, bool, error) {
		p := s.storePath(key)
		switch op {
		case Put:
			if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
				return 0, false, err
			}
			return time.Millisecond, false, copyFile(localFile, p)
		case Get:
			return time.Millisecond, false, copyFile(p, localFile)
		case Delete:
			err := os.Remove(p)
			if os.IsNotExist(err) {
				return time.Millisecond, false, nil
			}
			return time.Millisecond, false, err
		}
		return time.Millisecond, false, nil
	}
}

// failRunner returns a CLIRunner that fails for the specified operation
// and delegates all others to the store runner.
func failRunner(store *mockStore, failOp OperationType) CLIRunner {
	base := store.runner()
	return func(ctx context.Context, op OperationType, key, localFile string) (time.Duration, bool, error) {
		if op == failOp {
			return 0, false, fmt.Errorf("injected failure for %s", failOp)
		}
		return base(ctx, op, key, localFile)
	}
}

// deleteFailRunner returns a CLIRunner that succeeds on put/get but fails on delete.
func deleteFailRunner(store *mockStore) CLIRunner {
	return failRunner(store, Delete)
}

// timeoutRunner returns a CLIRunner that reports a timeout for the specified
// operation without blocking, and delegates others to the store.
func timeoutRunner(store *mockStore, timeoutOp OperationType) CLIRunner {
	base := store.runner()
	return func(ctx context.Context, op OperationType, key, localFile string) (time.Duration, bool, error) {
		if op == timeoutOp {
			return 200 * time.Millisecond, true, fmt.Errorf("operation timed out")
		}
		return base(ctx, op, key, localFile)
	}
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close() //nolint:errcheck

	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer df.Close() //nolint:errcheck

	_, err = io.Copy(df, sf)
	return err
}
