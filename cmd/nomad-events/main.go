package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"nomad-events/internal/config"
	"nomad-events/internal/nomad"
	"nomad-events/internal/outputs"
	"nomad-events/internal/routing"
)

var (
	// Version information - set by build
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// ServiceManager manages the runtime components that can be reloaded
type ServiceManager struct {
	mu            sync.RWMutex
	router        *routing.Router
	outputManager *outputs.Manager
	configPath    string
	eventStream   *nomad.EventStream
}

// NewServiceManager creates a new service manager with initial configuration
func NewServiceManager(configPath string, eventStream *nomad.EventStream) (*ServiceManager, error) {
	sm := &ServiceManager{
		configPath:  configPath,
		eventStream: eventStream,
	}

	// Load initial configuration
	if err := sm.reloadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load initial configuration: %w", err)
	}

	return sm, nil
}

// reloadConfig loads the configuration file and updates router and output manager
func (sm *ServiceManager) reloadConfig() error {
	slog.Info("Starting configuration reload", "config_path", sm.configPath)

	// Load and validate new configuration
	cfg, err := config.LoadConfig(sm.configPath)
	if err != nil {
		slog.Error("Failed to load new configuration", "error", err, "config_path", sm.configPath)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create new router
	newRouter, err := routing.NewRouter(cfg.Routes)
	if err != nil {
		slog.Error("Failed to create new router", "error", err)
		return fmt.Errorf("failed to create router: %w", err)
	}

	// Create new output manager
	newOutputManager, err := outputs.NewManager(cfg.Outputs, sm.eventStream.Client())
	if err != nil {
		slog.Error("Failed to create new output manager", "error", err)
		return fmt.Errorf("failed to create output manager: %w", err)
	}

	// Atomically replace components
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.router = newRouter
	sm.outputManager = newOutputManager

	slog.Info("Configuration reload completed successfully",
		"outputs", len(cfg.Outputs),
		"routes", len(cfg.Routes))

	return nil
}

// Route processes an event through the current router (thread-safe)
func (sm *ServiceManager) Route(event nomad.Event) ([]string, error) {
	sm.mu.RLock()
	router := sm.router
	sm.mu.RUnlock()

	return router.Route(event)
}

// Send sends an event to the specified output (thread-safe)
func (sm *ServiceManager) Send(outputName string, event nomad.Event) error {
	sm.mu.RLock()
	outputManager := sm.outputManager
	sm.mu.RUnlock()

	return outputManager.Send(outputName, event)
}

func main() {
	var (
		configPath     = flag.String("config", "config.yaml", "Path to configuration file")
		validateConfig = flag.Bool("validate-config", false, "Validate configuration and exit")
		showVersion    = flag.Bool("version", false, "Show version information and exit")
		showHelp       = flag.Bool("help", false, "Show help information and exit")
		logLevel       = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		logFormat      = flag.String("log-format", "text", "Log format (text, json)")
	)

	// Setup structured logging
	setupLogging(*logLevel, *logFormat)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `nomad-events - Nomad Event Stream Processor

USAGE:
    nomad-events [options]

DESCRIPTION:
    Connects to Nomad's event stream API and processes events through a configurable
    routing engine, forwarding them to various output destinations like Slack, HTTP
    webhooks, RabbitMQ, command execution, or stdout.

OPTIONS:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
EXAMPLES:
    # Run with default config
    nomad-events

    # Run with custom config
    nomad-events -config /path/to/config.yaml

    # Validate configuration
    nomad-events -validate-config -config config.yaml

    # Show version information
    nomad-events -version

    # Reload configuration without restart (send SIGHUP)
    kill -HUP <pid>

For more information, see: https://github.com/your-repo/nomad-events
`)
	}

	flag.Parse()

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("nomad-events %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  date: %s\n", date)
		fmt.Printf("  go: %s\n", runtime.Version())
		fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err, "config_path", *configPath)
		os.Exit(1)
	}

	if *validateConfig {
		fmt.Printf("✅ Configuration is valid\n")
		fmt.Printf("   - Config file: %s\n", *configPath)
		fmt.Printf("   - Nomad address: %s\n", cfg.Nomad.Address)
		fmt.Printf("   - Outputs defined: %d\n", len(cfg.Outputs))
		fmt.Printf("   - Routes defined: %d\n", len(cfg.Routes))

		// Test router creation
		_, err := routing.NewRouter(cfg.Routes)
		if err != nil {
			slog.Error("Failed to validate routing configuration", "error", err)
			os.Exit(1)
		}
		fmt.Printf("   - Routing configuration: valid\n")

		// Test output manager creation
		_, err = outputs.NewManager(cfg.Outputs, nil)
		if err != nil {
			slog.Error("Failed to validate output configuration", "error", err)
			os.Exit(1)
		}
		fmt.Printf("   - Output configuration: valid\n")

		fmt.Printf("✅ All configuration checks passed!\n")
		os.Exit(0)
	}

	slog.Info("Starting nomad-events",
		"version", version,
		"nomad_address", cfg.Nomad.Address,
		"config_path", *configPath)

	eventStream, err := nomad.NewEventStream(cfg.Nomad.Address, cfg.Nomad.Token)
	if err != nil {
		slog.Error("Failed to create Nomad event stream", "error", err, "nomad_address", cfg.Nomad.Address)
		os.Exit(1)
	}

	// Create service manager with reloadable components
	serviceManager, err := NewServiceManager(*configPath, eventStream)
	if err != nil {
		slog.Error("Failed to create service manager", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	eventChan := make(chan nomad.Event, 100)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := eventStream.Stream(ctx, eventChan); err != nil && err != context.Canceled {
			slog.Error("Event stream error", "error", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		processEvents(ctx, eventChan, serviceManager)
	}()

	slog.Info("Service started successfully",
		"event_buffer_size", cap(eventChan))

	// Signal handling loop
	for {
		sig := <-sigChan
		slog.Info("Received signal", "signal", sig.String())

		switch sig {
		case syscall.SIGHUP:
			slog.Info("SIGHUP received - reloading configuration...")
			if err := serviceManager.reloadConfig(); err != nil {
				slog.Error("Configuration reload failed - continuing with current config", "error", err)
			} else {
				slog.Info("Configuration reload successful")
			}
			continue

		case syscall.SIGINT, syscall.SIGTERM:
			slog.Info("Initiating graceful shutdown...")
			cancel()

			// Close event channel and wait for goroutines with timeout
			close(eventChan)

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				slog.Info("Graceful shutdown completed")
			case <-time.After(30 * time.Second):
				slog.Warn("Shutdown timeout exceeded, forcing exit")
			}

			return // Exit the main function
		}
	}
}

func processEvents(ctx context.Context, eventChan <-chan nomad.Event, serviceManager *ServiceManager) {
	eventCount := 0
	for {
		select {
		case <-ctx.Done():
			slog.Debug("Processing stopped", "events_processed", eventCount)
			return
		case event, ok := <-eventChan:
			if !ok {
				slog.Debug("Event channel closed", "events_processed", eventCount)
				return
			}

			eventCount++

			slog.Debug("Processing event",
				"topic", event.Topic,
				"type", event.Type,
				"key", event.Key,
				"index", event.Index)

			matchedOutputs, err := serviceManager.Route(event)
			if err != nil {
				slog.Error("Failed to route event",
					"error", err,
					"topic", event.Topic,
					"type", event.Type,
					"key", event.Key)
				continue
			}

			slog.Debug("Event routed",
				"topic", event.Topic,
				"type", event.Type,
				"outputs", matchedOutputs)

			for _, outputName := range matchedOutputs {
				if err := serviceManager.Send(outputName, event); err != nil {
					slog.Error("Failed to send event to output",
						"error", err,
						"output", outputName,
						"topic", event.Topic,
						"type", event.Type)
				}
			}
		}
	}
}

func setupLogging(level, format string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
