package storage

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type NotExistsError struct{}

func (e *NotExistsError) Error() string {
	return "object does not exist"
}

type CommandExecuter struct {
	str Storager
}

func NewCommandExecuter(s Storager) *CommandExecuter {
	return &CommandExecuter{str: s}
}

func (sty *CommandExecuter) SetStorager(s Storager) {
	sty.str = s
}

func (sty *CommandExecuter) Execute(cmd string, nonFlagArgs []string) error {

	switch cmd {
	case "put":
		if len(nonFlagArgs) != 2 {
			return fmt.Errorf("put method expected 2 arguments got %d", len(nonFlagArgs))
		}
		sourceFilePath, dst := nonFlagArgs[0], nonFlagArgs[1]

		_, err := os.Stat(sourceFilePath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		return sty.str.Put(sourceFilePath, dst)

	case "get":
		if len(nonFlagArgs) != 2 {
			return fmt.Errorf("get method expected 2 arguments got %d", len(nonFlagArgs))
		}
		src, dst := nonFlagArgs[0], nonFlagArgs[1]
		return sty.str.Get(src, dst)

	case "copy":
		if len(nonFlagArgs) != 2 {
			return fmt.Errorf("copy method expected 2 arguments got %d", len(nonFlagArgs))
		}

		srcBlob, dstBlob := nonFlagArgs[0], nonFlagArgs[1]
		return sty.str.Copy(srcBlob, dstBlob)

	case "delete":
		if len(nonFlagArgs) != 1 {
			return fmt.Errorf("delete method expected 1 argument got %d", len(nonFlagArgs))
		}
		return sty.str.Delete(nonFlagArgs[0])

	case "delete-recursive":
		var prefix string
		if len(nonFlagArgs) > 1 {
			return fmt.Errorf("delete-recursive takes at most 1 argument (prefix) got %d", len(nonFlagArgs))
		}
		if len(nonFlagArgs) == 1 {
			prefix = nonFlagArgs[0]
		}

		return sty.str.DeleteRecursive(prefix)

	case "exists":
		if len(nonFlagArgs) != 1 {
			return fmt.Errorf("exists method expected 1 argument got %d", len(nonFlagArgs))
		}

		exists, err := sty.str.Exists(nonFlagArgs[0])
		if err == nil && !exists {
			return &NotExistsError{}
		}
		if err != nil {
			return fmt.Errorf("failed to check exist: %w", err)
		}

	case "sign":
		if len(nonFlagArgs) != 3 {
			return fmt.Errorf("sign method expects 3 arguments got %d", len(nonFlagArgs))
		}

		objectID, action := nonFlagArgs[0], nonFlagArgs[1]
		action = strings.ToLower(action)
		if action != "get" && action != "put" {
			return fmt.Errorf("action not implemented: %s. Available actions are 'get' and 'put'", action)
		}

		expiration, err := time.ParseDuration(nonFlagArgs[2])
		if err != nil {
			return fmt.Errorf("expiration should be in the format of a duration i.e. 1h, 60m, 3600s. Got: %s", nonFlagArgs[2])
		}

		signedURL, err := sty.str.Sign(objectID, action, expiration)
		if err != nil {
			return fmt.Errorf("failed to sign request: %w", err)
		}
		fmt.Print(signedURL)

	case "list":
		var prefix string
		if len(nonFlagArgs) > 1 {
			return fmt.Errorf("list method takes at most 1 argument (prefix) got %d", len(nonFlagArgs))
		}
		if len(nonFlagArgs) == 1 {
			prefix = nonFlagArgs[0]
		}

		var objects []string
		objects, err := sty.str.List(prefix)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		for _, object := range objects {
			fmt.Println(object)
		}

	case "properties":
		if len(nonFlagArgs) != 1 {
			return fmt.Errorf("properties method expected 1 argument got %d", len(nonFlagArgs))
		}
		return sty.str.Properties(nonFlagArgs[0])

	case "ensure-storage-exists":
		if len(nonFlagArgs) != 0 {
			return fmt.Errorf("ensureStorageExists method expected 0 argument got %d", len(nonFlagArgs))
		}
		return sty.str.EnsureStorageExists()

	default:
		return fmt.Errorf("unknown command: '%s'", cmd)
	}

	return nil
}
