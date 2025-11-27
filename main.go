package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	storage "github.com/cloudfoundry/storage-cli/storage"
)

var version string

func main() {

	configPath := flag.String("c", "", "configuration path")
	showVer := flag.Bool("v", false, "version")
	storageType := flag.String("s", "s3", "storage type: azurebs|alioss|s3|gcs")
	flag.Parse()

	if *showVer {
		fmt.Printf("version %s\n", version)
		os.Exit(0)
	}

	configFile, err := os.Open(*configPath)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := storage.NewStorageClient(*storageType, configFile)
	if err != nil {
		log.Fatalln(err)
	}
	sty := storage.NewStrategy(client)

	nonFlagArgs := flag.Args()
	if len(nonFlagArgs) < 2 {
		log.Fatalf("Expected at least two arguments got %d\n", len(nonFlagArgs))
	}

	cmd := nonFlagArgs[0]
	sty.ExecuteCommand(cmd, nonFlagArgs)

}
