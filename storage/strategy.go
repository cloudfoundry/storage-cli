package storage

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type NotExistsError struct {
	err error
}

func (e *NotExistsError) Error() string {
	return "object does not exist"
}

type Strategy struct {
	str Storager
}

func NewStrategy(s Storager) *Strategy {
	return &Strategy{str: s}
}

func (sty *Strategy) SetStorager(s Storager) {
	sty.str = s
}

func (sty *Strategy) ExecuteCommand(cmd string, nonFlagArgs []string) error {

	switch cmd {
	case "put":
		if len(nonFlagArgs) != 3 {
			return fmt.Errorf("Put method expected 3 arguments got %d\n", len(nonFlagArgs))
		}
		sourceFilePath, dst := nonFlagArgs[1], nonFlagArgs[2]

		_, err := os.Stat(sourceFilePath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		return sty.str.Put(sourceFilePath, dst)

	case "get":
		if len(nonFlagArgs) != 3 {
			return fmt.Errorf("Get method expected 3 arguments got %d\n", len(nonFlagArgs))
		}
		src, dst := nonFlagArgs[1], nonFlagArgs[2]
		return sty.str.Get(src, dst)

	case "copy":
		if len(nonFlagArgs) != 3 {
			return fmt.Errorf("Copy method expected 3 arguments got %d\n", len(nonFlagArgs))
		}

		srcBlob, dstBlob := nonFlagArgs[1], nonFlagArgs[2]
		return sty.str.Copy(srcBlob, dstBlob)

	case "delete":
		if len(nonFlagArgs) != 2 {
			return fmt.Errorf("Delete method expected 2 arguments got %d\n", len(nonFlagArgs))
		}
		return sty.str.Delete(nonFlagArgs[1])

	case "delete-recursive":
		var prefix string
		if len(nonFlagArgs) > 2 {
			return fmt.Errorf("delete-recursive takes at most one argument (prefix) got %d\n", len(nonFlagArgs)-1)
		} else if len(nonFlagArgs) == 2 {
			prefix = nonFlagArgs[1]
		} else {
			prefix = ""
		}
		return sty.str.DeleteRecursive(prefix)

	case "exists":
		if len(nonFlagArgs) != 2 {
			return fmt.Errorf("Exists method expected 2 arguments got %d\n", len(nonFlagArgs))
		}

		exists, err := sty.str.Exists(nonFlagArgs[1])
		if err == nil && !exists {
			return &NotExistsError{}
		}
		if err != nil {
			return fmt.Errorf("Failed to check exist: %w", err)
		}

	case "sign":
		if len(nonFlagArgs) != 4 {
			return fmt.Errorf("Sign method expects 3 arguments got %d\n", len(nonFlagArgs)-1)
		}

		objectID, action := nonFlagArgs[1], nonFlagArgs[2]
		action = strings.ToLower(action)
		if action != "get" && action != "put" {
			return fmt.Errorf("Action not implemented: %s. Available actions are 'get' and 'put'", action)
		}

		expiration, err := time.ParseDuration(nonFlagArgs[3])
		if err != nil {
			return fmt.Errorf("Expiration should be in the format of a duration i.e. 1h, 60m, 3600s. Got: %s", nonFlagArgs[3])
		}

		signedURL, err := sty.str.Sign(objectID, action, expiration)
		if err != nil {
			return fmt.Errorf("Failed to sign request: %w", err)
		}
		fmt.Print(signedURL)

	case "list":
		var prefix string

		if len(nonFlagArgs) == 1 {
			prefix = ""
		} else if len(nonFlagArgs) == 2 {
			prefix = nonFlagArgs[1]
		} else {
			return fmt.Errorf("List method expected 1 or 2 arguments, got %d\n", len(nonFlagArgs)-1)
		}

		var objects []string
		objects, err := sty.str.List(prefix)
		if err != nil {
			return fmt.Errorf("Failed to list objects: %w", err)
		}

		for _, object := range objects {
			fmt.Println(object)
		}

	case "properties":
		if len(nonFlagArgs) != 2 {
			return fmt.Errorf("Properties method expected 2 arguments got %d\n", len(nonFlagArgs))
		}
		return sty.str.Properties(nonFlagArgs[1])

	case "ensure-storage-exists":
		if len(nonFlagArgs) != 1 {
			return fmt.Errorf("EnsureStorageExists method expected 1 arguments got %d\n", len(nonFlagArgs))
		}
		return sty.str.EnsureStorageExists()

	default:
		return fmt.Errorf("unknown command: '%s'\n", cmd)
	}

	return nil
}
