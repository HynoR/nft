package main

import (
	"flag"
	"log/slog"
	"os"
)

var (
	configPath  string
	testMode    bool
	convertMode string
	globalIp    string
)

// func useLogger(path string) {
// 	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
// 	if err != nil {
// 		slog.Error("Failed to open log file", "error", err, "path", path)
// 		os.Exit(1)
// 	}

// 	// Create a new logger that writes to both stdout and file
// 	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, logFile), nil))
// 	slog.SetDefault(logger)
// }

func main() {
	// Check command line arguments

	flag.StringVar(&configPath, "c", "nat.conf", "path to configuration file")
	flag.BoolVar(&testMode, "t", false, "run in test mode")
	flag.StringVar(&convertMode, "convert", "", "convert iptables rules to nftables rules")
	flag.StringVar(&globalIp, "ip", "", "global ip address")
	flag.Parse()

	if configPath == "nat.conf" {
		generateDefaultConfig()
		slog.Info("Using default configuration", "config", configPath)
	}

	// Create new Service
	service := NewNatService()

	if globalIp != "" {
		service.GlobalLocalIP = globalIp
	}
	// 读取/opt/nat/env是否存在，如果存在则读取里面的globalIp
	if _, err := os.Stat("/opt/nat/env"); err == nil {
		envFile, err := os.ReadFile("/opt/nat/env")
		if err != nil {
			slog.Error("Failed to read env file", "error", err)
		}
		service.GlobalLocalIP = string(envFile)
		slog.Info("Read global ip from env file", "ip", service.GlobalLocalIP)
	}

	if testMode {
		service.TestMode = true
		slog.Info("Running in test mode")
	}
	if convertMode != "" {
		slog.Info("Running in convert mode", "convert", convertMode)
		service.ConvertTask(configPath, convertMode)
		return
	}
	service.InitEnv().AddConfig(configPath).Run()
}

func generateDefaultConfig() {
	// Generate default configuration file
	if _, err := os.Stat("nat.conf"); os.IsNotExist(err) {
		f, err := os.Create("nat.conf")
		if err != nil {
			slog.Error("Failed to create default config file", "error", err)
			os.Exit(1)
		}
		defer f.Close()
		f.WriteString(`# Example NAT configuration
	# Add your configuration format here
	`)
	}
}
