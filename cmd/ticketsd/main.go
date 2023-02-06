package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	firebase "firebase.google.com/go/v4"
	"github.com/mroobert/tixer-tickets/gcfirestore"
	"github.com/mroobert/tixer-tickets/http"
	"golang.org/x/exp/slog"
)

var (
	ErrFirebaseProjectIdNotProvided = errors.New("firebase-project-id not provided")
	ErrInitFirebaseApp              = errors.New("could not initialize firebase app")
	ErrInitFireStoreClient          = errors.New("could not initialize firestore client")
)

func main() {
	ctx := context.Background()

	app, err := BuildApplication(ctx)
	if err != nil {
		fmt.Println("error building application: ", err)
		os.Exit(1)
	}

	shutdown := make(chan error)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		app.Logger.Info("service interrupt received", "signal", s.String())
		err := app.Shutdown()
		if err != nil {
			shutdown <- err
		}
		app.Logger.Info("shutdown complete")
		shutdown <- nil
	}()

	if err := app.Run(ctx); err != nil {
		app.Shutdown()
		app.Logger.Error("run error", err)
		os.Exit(1)
	}

	err = <-shutdown
	if err != nil {
		app.Logger.Error("shutdown error", err)
	}

	app.Logger.Info("service stopped")
}

// Config holds all the configuration settings for the application.
// We will read in these configuration settings from command-line
// flags when the application starts.
type Config struct {
	Env string
	Web struct {
		IdleTimeout     time.Duration
		WriteTimeout    time.Duration
		ReadTimeout     time.Duration
		ShutdownTimeout time.Duration
		APIHost         string
		DebugHost       string
	}
	Firebase struct {
		ProjectID string
		Firestore struct {
			CollectionName string
			CounterDocID   string
		}
	}
}

// Application holds the dependencies for this app.
type Application struct {
	Config     Config
	Logger     *slog.Logger
	HTTPServer *http.Server
}

// BuildApplication creates a new configured Application.
func BuildApplication(ctx context.Context) (*Application, error) {
	var (
		app Application
		cfg Config
	)
	app = Application{}

	flag.StringVar(&cfg.Env, "env", "development", "Environment (development|staging|production)")

	// Web
	flag.StringVar(&cfg.Web.APIHost, "api-host", "0.0.0.0:8080", "API Host")
	flag.StringVar(&cfg.Web.DebugHost, "debug-host", "0.0.0.0:3000", "Debug Host")
	flag.DurationVar(&cfg.Web.IdleTimeout, "idle-timeout", 120*time.Second, "Idle Timeout")
	flag.DurationVar(&cfg.Web.WriteTimeout, "write-timeout", 10*time.Second, "Write Timeout")
	flag.DurationVar(&cfg.Web.ReadTimeout, "read-timeout", 5*time.Second, "Read Timeout")
	flag.DurationVar(&cfg.Web.ShutdownTimeout, "shutdown-timeout", 20*time.Second, "Shutdown Timeout")

	// Firebase
	flag.StringVar(&cfg.Firebase.ProjectID, "firebase-project-id", "", "Firebase project ID")
	flag.StringVar(&cfg.Firebase.Firestore.CollectionName, "firestore-collection-name", "tickets", "Tickets collection name")
	flag.StringVar(&cfg.Firebase.Firestore.CounterDocID, "firestore-stats-doc-ID", "--counter--", "Document ID which stores tickets counter")

	flag.Parse()
	app.Config = cfg

	// Init Store client.
	if app.Config.Firebase.ProjectID == "" {
		return nil, ErrFirebaseProjectIdNotProvided
	}

	fbTicketsApp, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: app.Config.Firebase.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("%q: %w", err.Error(), ErrInitFirebaseApp)
	}

	storeClient, err := fbTicketsApp.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", err.Error(), ErrInitFireStoreClient)
	}

	storer := gcfirestore.NewStorer(
		storeClient,
		app.Config.Firebase.Firestore.CollectionName,
		app.Config.Firebase.Firestore.CounterDocID,
	)

	// Instantiate HTTP Server.
	app.SetLogger()
	app.HTTPServer = http.NewServer(
		http.WithAddr(app.Config.Web.APIHost),
		http.WithIdleTimeout(app.Config.Web.IdleTimeout),
		http.WithLogger(app.Logger),
		http.WithReadTimeout(app.Config.Web.ReadTimeout),
		http.WithWriteTimeout(app.Config.Web.WriteTimeout),
		http.WithShutdownTimeout(app.Config.Web.ShutdownTimeout),
	)
	app.HTTPServer.TicketService = storer
	app.HTTPServer.AttachRoutesV1()

	return &app, nil
}

// Run performs the startup sequence.
func (a *Application) Run(ctx context.Context) error {
	a.Logger.Info("starting the server", "addr", a.HTTPServer.Addr, "env", a.Config.Env)
	if err := a.HTTPServer.Open(); err != nil {
		return err
	}

	return nil
}

// Shutdown performs the gracefull shutdown sequence.
func (a *Application) Shutdown() error {
	if a.HTTPServer != nil {
		if err := a.HTTPServer.Shutdown(); err != nil {
			a.HTTPServer.Close()
			return err
		}
	}

	return nil
}

func (a *Application) SetLogger() {
	switch a.Config.Env {
	case "development":
		a.Logger = slog.New(slog.NewTextHandler(os.Stdout))
	case "staging", "production":
		a.Logger = slog.New(slog.NewJSONHandler(os.Stdout))
	}
}
