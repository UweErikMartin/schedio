// Package retention implements the GDPR data-retention + billing background
// job. It runs once at startup and then every 24 hours.
//
// Pass 1 — Billing: generate invoices for contacts whose last appointment has
// ended and billing_generated is false.
//
// Pass 2 — Retention notification: send retention-notify emails to all Staff
// for contacts whose last_appointment_end_at + retention_period_days ≤ now
// and retention_state = 'active'.
//
// Pass 3 — Confirmation expiry: move contacts from 'notified' to
// 'pending_deletion' when retention_notified_at + 7 days ≤ now.
package retention

import (
	"context"
	"time"

	"schedio/internal/billing"
	"schedio/internal/email"
	"schedio/internal/store"

	"k8s.io/klog/v2"
)

const (
	jobInterval        = 24 * time.Hour
	confirmationWindow = 7 * 24 * time.Hour
)

// StartJob launches the background retention + billing goroutine. It runs
// immediately on start and then on a 24-hour ticker. The goroutine exits when
// ctx is cancelled.
func StartJob(ctx context.Context, st store.DomainStore, sender *email.Sender, billingSvc *billing.Service) {
	go func() {
		klog.Info("retention: starting background job")
		runAll(ctx, st, sender, billingSvc)

		ticker := time.NewTicker(jobInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				klog.Info("retention: background job stopped")
				return
			case <-ticker.C:
				runAll(ctx, st, sender, billingSvc)
			}
		}
	}()
}

// runAll executes all three passes sequentially.
func runAll(ctx context.Context, st store.DomainStore, sender *email.Sender, billingSvc *billing.Service) {
	runBillingPass(ctx, st, billingSvc)
	runRetentionNotifyPass(ctx, st, sender)
	runConfirmationExpiryPass(ctx, st)
}

// runBillingPass — Pass 1.
func runBillingPass(ctx context.Context, st store.DomainStore, billingSvc *billing.Service) {
	contacts, err := st.ListBillingDue(ctx)
	if err != nil {
		klog.Errorf("retention: billing pass: list billing due: %v", err)
		return
	}
	klog.V(2).Infof("retention: billing pass: %d contacts due", len(contacts))
	for _, c := range contacts {
		if err := billingSvc.GenerateAndSend(ctx, c); err != nil {
			klog.Errorf("retention: billing pass: contact %s: %v", c.ID, err)
			continue
		}
		if err := st.MarkBillingGenerated(ctx, c.ID); err != nil {
			klog.Errorf("retention: billing pass: mark billing generated %s: %v", c.ID, err)
		}
	}
}

// runRetentionNotifyPass — Pass 2.
func runRetentionNotifyPass(ctx context.Context, st store.DomainStore, sender *email.Sender) {
	settings, err := st.GetSettings(ctx)
	if err != nil {
		klog.Errorf("retention: notify pass: get settings: %v", err)
		return
	}
	period := time.Duration(settings.RetentionPeriodDays) * 24 * time.Hour

	contacts, err := st.ListRetentionDue(ctx, period)
	if err != nil {
		klog.Errorf("retention: notify pass: list retention due: %v", err)
		return
	}
	klog.V(2).Infof("retention: notify pass: %d contacts due", len(contacts))

	staff, err := st.ListStaff(ctx)
	if err != nil {
		klog.Errorf("retention: notify pass: list staff: %v", err)
		return
	}
	staffEmails := make([]string, 0, len(staff))
	for _, s := range staff {
		staffEmails = append(staffEmails, s.Email)
	}

	for _, c := range contacts {
		expires := time.Now().UTC().Add(confirmationWindow)
		// TODO: generate signed deletion-confirmation URL via token.Signer.
		confirmURL := "/admin/api/v1/retention/confirm?token=TODO"

		if err := sender.SendRetentionNotify(ctx, staffEmails, email.RetentionNotifyData{
			Contact:          c,
			ConfirmDeleteURL: confirmURL,
			ExpiresAt:        expires,
			SentAt:           time.Now().UTC(),
		}); err != nil {
			klog.Errorf("retention: notify pass: send email for contact %s: %v", c.ID, err)
			continue
		}
		if err := st.MarkRetentionNotified(ctx, c.ID); err != nil {
			klog.Errorf("retention: notify pass: mark notified %s: %v", c.ID, err)
		}
	}
}

// runConfirmationExpiryPass — Pass 3.
func runConfirmationExpiryPass(ctx context.Context, st store.DomainStore) {
	contacts, err := st.ListConfirmationExpired(ctx)
	if err != nil {
		klog.Errorf("retention: expiry pass: list confirmation expired: %v", err)
		return
	}
	klog.V(2).Infof("retention: expiry pass: %d contacts expired", len(contacts))
	for _, c := range contacts {
		if err := st.AddToPendingDeletion(ctx, c.ID); err != nil {
			klog.Errorf("retention: expiry pass: add to pending deletion %s: %v", c.ID, err)
		}
	}
}
