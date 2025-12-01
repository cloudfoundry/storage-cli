package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	storage "github.com/cloudfoundry/storage-cli/storage"
)

var version string

func fatalLog(cmd string, err error) {
	if err == nil {
		return
	}
	// If the object exists the exit status is 0, otherwise it is 3
	// We are using `3` since `1` and `2` have special meanings
	if _, ok := err.(*storage.NotExistsError); ok {
		log.Printf("performing operation %s: %s\n", cmd, err)
		os.Exit(3)
	}
	log.Fatalf("performing operation %s: %s\n", cmd, err)

}

func main() {

	configPath := flag.String("c", "", "configuration path")
	showVer := flag.Bool("v", false, "version")
	storageType := flag.String("s", "s3", "storage type: azurebs|alioss|s3|gcs|dav")
	flag.Parse()

	if *showVer {
		fmt.Printf("version %s\n", version)
		os.Exit(0)
	}

	configFile, err := os.Open(*configPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer configFile.Close() //nolint:errcheck

	client, err := storage.NewStorageClient(*storageType, configFile)
	if err != nil {
		log.Fatalln(err)
	}

	sty := storage.NewStrategy(client)

	nonFlagArgs := flag.Args()
	if len(nonFlagArgs) < 1 {
		log.Fatalf("Expected at least 1 argument got 0")
	}

	cmd := nonFlagArgs[0]
	err = sty.ExecuteCommand(cmd, nonFlagArgs)
	fatalLog(cmd, err)

}
