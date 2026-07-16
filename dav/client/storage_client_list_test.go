package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

// fakeDavServer serves a static directory tree over PROPFIND/DELETE and
// records which paths were hit, so tests can assert how far a walk went.
type fakeDavServer struct {
	mu        sync.Mutex
	propfinds []string
	deletes   []string

	// dirs maps a directory path (relative to the endpoint, "" for the
	// root) to its child entries.
	dirs map[string][]fakeEntry
}

type fakeEntry struct {
	name  string
	isDir bool
}

func (s *fakeDavServer) handler(endpointPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.Trim(strings.TrimPrefix(r.URL.Path, endpointPath), "/")

		switch r.Method {
		case "PROPFIND":
			s.mu.Lock()
			s.propfinds = append(s.propfinds, rel)
			s.mu.Unlock()

			children, ok := s.dirs[rel]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			var b strings.Builder
			b.WriteString(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">`)
			writeResponse := func(href string, isDir bool) {
				resourceType := ""
				if isDir {
					resourceType = "<D:collection/>"
				}
				fmt.Fprintf(&b,
					`<D:response><D:href>%s</D:href><D:propstat><D:prop><D:resourcetype>%s</D:resourcetype></D:prop></D:propstat></D:response>`,
					href, resourceType)
			}

			self := endpointPath + "/"
			if rel != "" {
				self = endpointPath + "/" + rel + "/"
			}
			writeResponse(self, true)
			for _, child := range children {
				href := endpointPath + "/" + child.name
				if rel != "" {
					href = endpointPath + "/" + rel + "/" + child.name
				}
				if child.isDir {
					href += "/"
				}
				writeResponse(href, child.isDir)
			}
			b.WriteString(`</D:multistatus>`)

			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			w.Write([]byte(b.String())) //nolint:errcheck

		case "DELETE":
			s.mu.Lock()
			s.deletes = append(s.deletes, rel)
			s.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

// newPartitionedStore models a CC-style blobstore: two levels of partition
// directories with blobs at the leaves, plus sibling partitions that a
// correctly scoped walk must never touch.
func newPartitionedStore() *fakeDavServer {
	return &fakeDavServer{
		dirs: map[string][]fakeEntry{
			"": {
				{name: "aa", isDir: true},
				{name: "ab", isDir: true},
				{name: "zz", isDir: true},
			},
			"aa":                     {{name: "bb", isDir: true}},
			"aa/bb":                  {{name: "aabb-other-guid", isDir: false}},
			"ab":                     {{name: "cd", isDir: true}, {name: "zz", isDir: true}},
			"ab/zz":                  {{name: "abzz-unrelated", isDir: false}},
			"ab/cd":                  {{name: "abcd-target-guid", isDir: true}, {name: "abcd-target-guid-file", isDir: false}, {name: "abcd-unrelated", isDir: false}},
			"ab/cd/abcd-target-guid": {{name: "cflinuxfs4", isDir: false}},
			"zz":                     {{name: "yy", isDir: true}},
			"zz/yy":                  {{name: "zzyy-other", isDir: false}},
		},
	}
}

func newTestStorageClient(t *testing.T, store *fakeDavServer) (*storageClient, func()) {
	t.Helper()
	server := httptest.NewServer(store.handler("/dav"))
	c := NewStorageClient(davconf.Config{Endpoint: server.URL + "/dav"}, http.DefaultClient)
	return c.(*storageClient), server.Close
}

func sorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListScopesWalkToPrefix(t *testing.T) {
	store := newPartitionedStore()
	c, cleanup := newTestStorageClient(t, store)
	defer cleanup()

	blobs, err := c.List("ab/cd/abcd-target-guid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBlobs := []string{"ab/cd/abcd-target-guid-file", "ab/cd/abcd-target-guid/cflinuxfs4"}
	if !equalStrings(sorted(blobs), wantBlobs) {
		t.Fatalf("unexpected blobs: %v, want %v", sorted(blobs), wantBlobs)
	}

	wantPropfinds := []string{"ab/cd", "ab/cd/abcd-target-guid"}
	if !equalStrings(sorted(store.propfinds), wantPropfinds) {
		t.Fatalf("walk was not scoped to the prefix: PROPFINDs hit %v, want %v", sorted(store.propfinds), wantPropfinds)
	}
}

func TestListEmptyPrefixWalksWholeStore(t *testing.T) {
	store := newPartitionedStore()
	c, cleanup := newTestStorageClient(t, store)
	defer cleanup()

	blobs, err := c.List("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBlobs := []string{
		"aa/bb/aabb-other-guid",
		"ab/cd/abcd-target-guid-file",
		"ab/cd/abcd-target-guid/cflinuxfs4",
		"ab/cd/abcd-unrelated",
		"ab/zz/abzz-unrelated",
		"zz/yy/zzyy-other",
	}
	if !equalStrings(sorted(blobs), wantBlobs) {
		t.Fatalf("unexpected blobs: %v, want %v", sorted(blobs), wantBlobs)
	}
}

func TestListMissingPrefixDirectoryReturnsEmpty(t *testing.T) {
	store := newPartitionedStore()
	c, cleanup := newTestStorageClient(t, store)
	defer cleanup()

	blobs, err := c.List("no/such/prefix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blobs) != 0 {
		t.Fatalf("expected no blobs, got %v", blobs)
	}
}

func TestListSingleSegmentPrefixPrunesSiblings(t *testing.T) {
	store := newPartitionedStore()
	c, cleanup := newTestStorageClient(t, store)
	defer cleanup()

	blobs, err := c.List("ab")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBlobs := []string{
		"ab/cd/abcd-target-guid-file",
		"ab/cd/abcd-target-guid/cflinuxfs4",
		"ab/cd/abcd-unrelated",
		"ab/zz/abzz-unrelated",
	}
	if !equalStrings(sorted(blobs), wantBlobs) {
		t.Fatalf("unexpected blobs: %v, want %v", sorted(blobs), wantBlobs)
	}

	for _, p := range store.propfinds {
		if strings.HasPrefix(p, "aa") || strings.HasPrefix(p, "zz") {
			t.Fatalf("walk descended into sibling partition %q; PROPFINDs: %v", p, store.propfinds)
		}
	}
}

func TestDeleteRecursiveDeletesOnlyPrefixedBlobs(t *testing.T) {
	store := newPartitionedStore()
	c, cleanup := newTestStorageClient(t, store)
	defer cleanup()

	if err := c.DeleteRecursive("ab/cd/abcd-target-guid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDeletes := []string{"ab/cd/abcd-target-guid-file", "ab/cd/abcd-target-guid/cflinuxfs4"}
	if !equalStrings(sorted(store.deletes), wantDeletes) {
		t.Fatalf("unexpected deletes: %v, want %v", sorted(store.deletes), wantDeletes)
	}

	wantPropfinds := []string{"ab/cd", "ab/cd/abcd-target-guid"}
	if !equalStrings(sorted(store.propfinds), wantPropfinds) {
		t.Fatalf("walk was not scoped to the prefix: PROPFINDs hit %v, want %v", sorted(store.propfinds), wantPropfinds)
	}
}

func TestListEscapingPrefixStaysInsideEndpoint(t *testing.T) {
	store := newPartitionedStore()
	c, cleanup := newTestStorageClient(t, store)
	defer cleanup()

	for _, prefix := range []string{"../", "../../..", "../etc/passwd"} {
		store.propfinds = nil

		blobs, err := c.List(prefix)
		if err != nil {
			t.Fatalf("List(%q): unexpected error: %v", prefix, err)
		}
		if len(blobs) != 0 {
			t.Fatalf("List(%q): expected no blobs, got %v", prefix, blobs)
		}
		for _, p := range store.propfinds {
			if strings.Contains(p, "..") {
				t.Fatalf("List(%q): PROPFIND escaped the endpoint: %v", prefix, store.propfinds)
			}
		}
	}
}

func TestDeleteRecursiveMissingPrefixIsNoop(t *testing.T) {
	store := newPartitionedStore()
	c, cleanup := newTestStorageClient(t, store)
	defer cleanup()

	if err := c.DeleteRecursive("no/such/prefix"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.deletes) != 0 {
		t.Fatalf("expected no deletes, got %v", store.deletes)
	}
}

func TestPrefixDir(t *testing.T) {
	cases := []struct {
		prefix string
		want   string
	}{
		{prefix: "", want: ""},
		{prefix: "abc", want: ""},
		{prefix: "ab/", want: "ab"},
		{prefix: "ab/cd", want: "ab"},
		{prefix: "ab/cd/", want: "ab/cd"},
		{prefix: "ab/cd/guid", want: "ab/cd"},
		{prefix: "/ab/cd/guid", want: "ab/cd"},
		{prefix: "..", want: ""},
		{prefix: "../", want: ""},
		{prefix: "../etc", want: ""},
		{prefix: "../../../etc", want: ""},
		{prefix: "a/../../b", want: ""},
		{prefix: "foo/../bar/x", want: "bar"},
	}
	for _, tc := range cases {
		if got := prefixDir(tc.prefix); got != tc.want {
			t.Errorf("prefixDir(%q) = %q, want %q", tc.prefix, got, tc.want)
		}
	}
}

func TestCollectionMayContainPrefix(t *testing.T) {
	cases := []struct {
		href   string
		prefix string
		want   bool
	}{
		{href: "/dav/ab/", prefix: "", want: true},
		{href: "/dav/ab/", prefix: "ab/cd/guid", want: true},
		{href: "/dav/ab/cd/", prefix: "ab/cd/guid", want: true},
		{href: "/dav/ab/cd/guid-dir/", prefix: "ab/cd/guid", want: true},
		{href: "/dav/ab/cd/other/", prefix: "ab/cd/guid", want: false},
		{href: "/dav/aa/", prefix: "ab/cd/guid", want: false},
		{href: "/dav/ab/zz/", prefix: "ab/cd/guid", want: false},
	}
	for _, tc := range cases {
		if got := collectionMayContainPrefix(tc.href, "/dav", tc.prefix); got != tc.want {
			t.Errorf("collectionMayContainPrefix(%q, %q) = %v, want %v", tc.href, tc.prefix, got, tc.want)
		}
	}
}
