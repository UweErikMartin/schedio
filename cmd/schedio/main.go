package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"schedio/internal/caldav"
	"schedio/internal/config"
	"schedio/internal/middleware"
	"schedio/internal/server"
	calstore "schedio/internal/store"

	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	args := config.ParseCommandLineArgs(os.Args)
	_ = flag.Set("v", strconv.Itoa(args.Verbose))

	var caldavStore calstore.CalendarStore
	if args.Dummy {
		klog.Info("using dummy CalDAV store")
		caldavStore = calstore.NewDummyStore()
	} else {
		caldavStore = calstore.NewMemoryStore()
	}
	caldavHandler := caldav.NewHandler(caldavStore, args.RootPath)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", args.BindAddress, args.Port),
		Handler:           middleware.LoggingMiddleware(server.NewRouter(&args, caldavHandler)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if args.RootPath != "" {
			klog.Infof("schedio listening on %s (root path: %s)", httpServer.Addr, args.RootPath)
		} else {
			klog.Infof("schedio listening on %s", httpServer.Addr)
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		klog.Fatalf("shutdown failed: %v", err)
	}

	klog.Info("schedio stopped")
}
