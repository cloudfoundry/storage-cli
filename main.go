package main

import (
	"errors"
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
		os.Exit(3)
	}
	slog.Error("performing operation", "command", cmd, "error", err)
	os.Exit(1)

}

// first, create path if not exist,
// then open/create file and return file pointer
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

func parseLogLevel(logLevel string) slog.Level {
	switch logLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}

}

// Configure slog to be json formated, set log level and
// stream to file if provided, by default it streams to os.Stderr
func configureSlog(m io.Writer, logLevel string) {
	level := parseLogLevel(logLevel)

	hOpt := &slog.HandlerOptions{Level: level}

	logger := slog.New(slog.NewJSONHandler(m, hOpt))
	slog.SetDefault(logger)
}

func main() {

	configPath := flag.String("c", "", "configuration path")
	showVer := flag.Bool("v", false, "version")
	storageType := flag.String("s", "", "storage type: azurebs|alioss|s3|gcs|dav")
	logFile := flag.String("log-file", "", "optional file with full path to write logs(if not specified log to os.Stderr, default behavior)")
	logLevel := flag.String("log-level", "warn", "log level: debug|info|warn|error")
	flag.Parse()

	if *showVer {
		fmt.Printf("version %s\n", version)
		os.Exit(0)
	}

	// configure slog
	writers := []io.Writer{os.Stderr}
	if *logFile != "" {
		f := createOrUseProvided(*logFile)
		defer f.Close() //nolint:errcheck
		writers = append(writers, f)
	}
	configureSlog(io.MultiWriter(writers...), *logLevel)

	// configure storage-cli config
	common.InitConfig(parseLogLevel(*logLevel))

	// check client config file exists
	configFile, err := os.Open(*configPath)
	if err != nil {
		fatalLog("", err)
	}
	defer configFile.Close() //nolint:errcheck

	// create client
	client, err := storage.NewStorageClient(*storageType, configFile)
	if err != nil {
		fatalLog("", err)
	}

	// inject client into executor
	cex := storage.NewCommandExecuter(client)

	// simple check for any command
	nonFlagArgs := flag.Args()
	if len(nonFlagArgs) < 1 {
		fatalLog("", errors.New("expected at least 1 argument (command) got 0"))
	}

	// execute command
	cmd := nonFlagArgs[0]
	err = cex.Execute(cmd, nonFlagArgs[1:])
	fatalLog(cmd, err)

}
