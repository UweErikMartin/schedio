// Package billing generates invoices for customers whose last appointment has
// ended and whose billing_generated flag is false. Each invoice is written as
// a plain-text file to DATA_DIR/invoices/ and a billing-invoice email is sent
// to all Staff users.
package billing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"schedio/internal/email"
	"schedio/internal/store"

	"k8s.io/klog/v2"
)

// Service handles invoice generation and file storage.
type Service struct {
	store   store.DomainStore
	email   *email.Sender
	dataDir string // root directory; invoices go to dataDir/invoices/
}

// NewService constructs a billing Service.
func NewService(st store.DomainStore, sender *email.Sender, dataDir string) *Service {
	return &Service{store: st, email: sender, dataDir: dataDir}
}

// GenerateAndSend generates an invoice for the contact and sends it to all
// Staff users. It is idempotent: calling it twice for the same contact is
// safe because the caller checks billing_generated before invoking.
func (svc *Service) GenerateAndSend(ctx context.Context, contact *store.Contact) error {
	bookings, err := svc.store.ListBookingsForContact(ctx, contact.ID)
	if err != nil {
		return fmt.Errorf("billing: list bookings for %s: %w", contact.ID, err)
	}

	invoice := renderInvoice(contact, bookings)
	filename := fmt.Sprintf("%s-%s-%s.txt",
		time.Now().UTC().Format("2006-01-02"),
		contact.LastName,
		contact.FirstName,
	)
	invoiceDir := filepath.Join(svc.dataDir, "invoices")
	if err := os.MkdirAll(invoiceDir, 0o750); err != nil {
		return fmt.Errorf("billing: create invoice dir: %w", err)
	}
	path := filepath.Join(invoiceDir, filename)
	if err := os.WriteFile(path, []byte(invoice), 0o640); err != nil {
		return fmt.Errorf("billing: write invoice %s: %w", path, err)
	}
	klog.V(2).Infof("billing: wrote invoice %s for contact %s", path, contact.ID)

	staff, err := svc.store.ListStaff(ctx)
	if err != nil {
		return fmt.Errorf("billing: list staff: %w", err)
	}
	emails := make([]string, 0, len(staff))
	for _, s := range staff {
		emails = append(emails, s.Email)
	}
	if err := svc.email.SendBillingInvoice(ctx, emails, email.BillingInvoiceData{
		Contact:     contact,
		Bookings:    bookings,
		InvoicePath: path,
		SentAt:      time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("billing: send invoice email: %w", err)
	}
	return nil
}

// renderInvoice produces a plain-text invoice for the contact.
func renderInvoice(contact *store.Contact, bookings []*store.Booking) string {
	var out string
	out += "Invoice\n"
	out += fmt.Sprintf("Customer: %s %s <%s>\n", contact.FirstName, contact.LastName, contact.Email)
	out += fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	for i, b := range bookings {
		out += fmt.Sprintf("%d. Booking %s\n   %s – %s\n   Service: %s\n",
			i+1, b.ID,
			b.StartAt.Format("2006-01-02 15:04"),
			b.EndAt.Format("15:04 MST"),
			b.ServiceID,
		)
	}
	return out
}
