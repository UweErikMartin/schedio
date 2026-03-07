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

	"schedio/internal/config"
	"schedio/internal/middleware"
	"schedio/internal/server"
	calstore "schedio/internal/store"
	"schedio/internal/token"

	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	args := config.ParseCommandLineArgs(os.Args)
	_ = flag.Set("v", strconv.Itoa(args.Verbose))

	klog.Infof("server timezone: %s", time.Now().Location())

	// MemoryStore implements both CalendarStore and DomainStore; share one
	// instance so that settings changes (e.g. DefaultCalendarName) are
	// immediately reflected in CalDAV responses without a restart.
	sharedStore := calstore.NewMemoryStore()
	var caldavStore calstore.CalendarStore
	if args.Dummy {
		klog.Info("using dummy CalDAV store")
		caldavStore = calstore.NewDummyStore()
	} else {
		caldavStore = sharedStore
	}
	domainStore := sharedStore
	if len(args.Users) > 0 {
		klog.Infof("syncing %d users into domain store", len(args.Users))
		if err := syncUsersFromConfig(args.Users, domainStore); err != nil {
			klog.Fatalf("error syncing users: %v", err)
		}
	}
	if len(args.Services) > 0 {
		klog.Infof("syncing %d services into domain store", len(args.Services))
		if err := syncServicesFromConfig(args.Services, domainStore); err != nil {
			klog.Fatalf("error syncing services: %v", err)
		}
	}
	if len(args.Timeslots) > 0 {
		klog.Infof("syncing %d timeslots into domain store", len(args.Timeslots))
		if err := syncAvailabilityFromConfig(args.Timeslots, domainStore); err != nil {
			klog.Fatalf("error syncing timeslots: %v", err)
		}
	}

	signer, err := token.NewSigner(context.Background(), domainStore)
	if err != nil {
		klog.Fatalf("error creating token signer: %v", err)
	}

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", args.BindAddress, args.Port),
		Handler:           middleware.LoggingMiddleware(server.NewRouter(&args, caldavStore, domainStore, signer)),
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

// syncUsersFromConfig converts the config.UserEntry list (read from -usersFile)
// into store.User values and syncs them into the domain store. When a UserEntry
// carries an explicit ID it is used as-is so that cross-references from the
// availability file remain stable across restarts.
func syncUsersFromConfig(entries []config.UserEntry, st calstore.DomainStore) error {
	users := make([]*calstore.User, len(entries))
	for i, e := range entries {
		id := e.ID
		if id == "" {
			id = calstore.NewID()
		}
		users[i] = &calstore.User{
			ID:                id,
			Email:             e.Email,
			PasswordHash:      e.PasswordHash,
			Role:              calstore.UserRole(e.Role),
			AppleOAuthEnabled: e.AppleOAuthEnabled,
			AppleSubject:      e.AppleSubject,
			Name:              e.Name,
		}
	}
	return st.SyncUsers(context.Background(), users)
}

// syncAvailabilityFromConfig converts the config.TimeslotEntry list (read from
// -availabilityFile) into store.Availability values and upserts them into the
// domain store. Each entry is idempotent: re-running with the same file
// produces the same in-memory state.
func syncAvailabilityFromConfig(entries []config.TimeslotEntry, st calstore.DomainStore) error {
	for _, e := range entries {
		start, err := time.Parse(time.RFC3339, e.StartAt)
		if err != nil {
			return fmt.Errorf("availability %q: invalid start_at %q: %w", e.UID, e.StartAt, err)
		}
		end, err := time.Parse(time.RFC3339, e.EndAt)
		if err != nil {
			return fmt.Errorf("availability %q: invalid end_at %q: %w", e.UID, e.EndAt, err)
		}
		ts := &calstore.Availability{
			ID:        calstore.NewID(),
			UserID:    e.UserID,
			CalDAVUID: e.UID,
			StartAt:   start,
			EndAt:     end,
			RRule:     e.RRule,
		}
		if err := st.UpsertAvailability(context.Background(), ts); err != nil {
			return fmt.Errorf("upsert availability %q: %w", e.UID, err)
		}
	}
	return nil
}

// syncServicesFromConfig converts the config.ServiceEntry list (read from
// -servicesFile or the built-in defaults) into store.Service values and
// creates them in the domain store. The domain store is always empty at
// startup so CreateService will never encounter a conflict here.
func syncServicesFromConfig(entries []config.ServiceEntry, st calstore.DomainStore) error {
	for _, e := range entries {
		svc := &calstore.Service{
			ID:              e.ID,
			Name:            e.Name,
			Summary:         e.Summary,
			Description:     e.Description,
			Price:           e.Price,
			DurationMinutes: e.DurationMinutes,
			DailyLimit:      e.DailyLimit,
		}
		if err := st.CreateService(context.Background(), svc); err != nil {
			return fmt.Errorf("create service %q: %w", e.ID, err)
		}
	}
	return nil
}
