package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudfoundry/storage-cli/ali/client"
	"github.com/cloudfoundry/storage-cli/ali/config"
)

var version string

func main() {

	configPath := flag.String("c", "", "configuration path")
	showVer := flag.Bool("v", false, "version")
	flag.Parse()

	if *showVer {
		fmt.Printf("version %s\n", version)
		os.Exit(0)
	}

	configFile, err := os.Open(*configPath)
	if err != nil {
		log.Fatalln(err)
	}

	aliConfig, err := config.NewFromReader(configFile)
	if err != nil {
		log.Fatalln(err)
	}

	storageClient, err := client.NewStorageClient(aliConfig)
	if err != nil {
		log.Fatalln(err)
	}

	blobstoreClient, err := client.New(storageClient)
	if err != nil {
		log.Fatalln(err)
	}

	nonFlagArgs := flag.Args()
	if len(nonFlagArgs) < 2 {
		log.Fatalf("Expected at least two arguments got %d\n", len(nonFlagArgs))
	}

	cmd := nonFlagArgs[0]

	switch cmd {
	case "put":
		if len(nonFlagArgs) != 3 {
			log.Fatalf("Put method expected 3 arguments got %d\n", len(nonFlagArgs))
		}
		sourceFilePath, destination := nonFlagArgs[1], nonFlagArgs[2]

		_, err := os.Stat(sourceFilePath)
		if err != nil {
			log.Fatalln(err)
		}

		err = blobstoreClient.Put(sourceFilePath, destination)
		fatalLog(cmd, err)

	case "get":
		if len(nonFlagArgs) != 3 {
			log.Fatalf("Get method expected 3 arguments got %d\n", len(nonFlagArgs))
		}
		source, destinationFilePath := nonFlagArgs[1], nonFlagArgs[2]

		err = blobstoreClient.Get(source, destinationFilePath)
		fatalLog(cmd, err)

	case "delete":
		if len(nonFlagArgs) != 2 {
			log.Fatalf("Delete method expected 2 arguments got %d\n", len(nonFlagArgs))
		}

		err = blobstoreClient.Delete(nonFlagArgs[1])
		fatalLog(cmd, err)

	case "exists":
		if len(nonFlagArgs) != 2 {
			log.Fatalf("Exists method expected 2 arguments got %d\n", len(nonFlagArgs))
		}

		var exists bool
		exists, err = blobstoreClient.Exists(nonFlagArgs[1])

		// If the object exists the exit status is 0, otherwise it is 3
		// We are using `3` since `1` and `2` have special meanings
		if err == nil && !exists {
			os.Exit(3)
		}

	case "sign":
		if len(nonFlagArgs) != 4 {
			log.Fatalf("Sign method expects 3 arguments got %d\n", len(nonFlagArgs)-1)
		}

		object, action := nonFlagArgs[1], nonFlagArgs[2]

		if action != "get" && action != "put" {
			log.Fatalf("Action not implemented: %s. Available actions are 'get' and 'put'", action)
		}

		duration, err := time.ParseDuration(nonFlagArgs[3])
		if err != nil {
			log.Fatalf("Expiration should be in the format of a duration i.e. 1h, 60m, 3600s. Got: %s", nonFlagArgs[3])
		}

		expiredInSec := int64(duration.Seconds())
		signedURL, err := blobstoreClient.Sign(object, action, expiredInSec)

		if err != nil {
			log.Fatalf("Failed to sign request: %s", err)
		}

		fmt.Println(signedURL)
		os.Exit(0)

	default:
		log.Fatalf("unknown command: '%s'\n", cmd)
	}
}

func fatalLog(cmd string, err error) {
	if err != nil {
		log.Fatalf("performing operation %s: %s\n", cmd, err)
	}
}
