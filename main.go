package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/storage-cli/common"
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

// creates path and file if not exist, othwerwise return fp
// of the existing file
func createOrUseProvided(logFile string) *os.File {
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			log.Fatalf("failed to create directory: %v", err)
		}
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}
	return f
}

// Configure slog to be json formated, set log level and
// stream to file if provided, by default it streams to os.Stderr
func configureSlog(m io.Writer, debug bool) {
	hOpt := &slog.HandlerOptions{Level: slog.LevelInfo}
	if debug {
		hOpt.Level = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(m, hOpt))
	slog.SetDefault(logger)
}

func main() {

	configPath := flag.String("c", "", "configuration path")
	showVer := flag.Bool("v", false, "version")
	storageType := flag.String("s", "s3", "storage type: azurebs|alioss|s3|gcs|dav")
	logFile := flag.String("l", "", "optional file with full path to write logs(if not specified log to os.Stderr, default behavior)")
	debug := flag.Bool("d", false, "run with debug mode")
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

	writers := []io.Writer{os.Stderr}
	if *logFile != "" {
		f := createOrUseProvided(*logFile)
		defer f.Close() //nolint:errcheck
		writers = append(writers, f)
	}
	configureSlog(io.MultiWriter(writers...), *debug)

	if *debug {
		common.InitConfig(*debug)
	}

	client, err := storage.NewStorageClient(*storageType, configFile)
	if err != nil {
		log.Fatalln(err)
	}

	cex := storage.NewCommandExecuter(client)

	nonFlagArgs := flag.Args()
	if len(nonFlagArgs) < 1 {
		log.Fatalf("Expected at least 1 argument (command) got 0")
	}

	cmd := nonFlagArgs[0]
	err = cex.Execute(cmd, nonFlagArgs[1:])
	fatalLog(cmd, err)

}
