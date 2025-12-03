package storage

import (
	"time"
)

type Storager interface {
	Put(sourceFilePath string, dest string) error
	Get(source string, dest string) error
	Delete(dest string) error
	DeleteRecursive(prefix string) error
	Exists(dest string) (bool, error)
	Sign(dest string, action string, expiration time.Duration) (string, error)
	List(prefix string) ([]string, error)
	Copy(srcBlob string, dstBlob string) error
	Properties(dest string) error
	EnsureStorageExists() error
}
