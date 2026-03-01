// Package email provides the SMTP-based email subsystem for schedio.
//
// Transport: implicit TLS via tls.Dial (port 465). No third-party mailer
// library is used.
//
// Templates are Go text/template files embedded with //go:embed. One pair of
// subject.txt + body.txt files per email type lives under templates/.
package email

import (
	"bytes"
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"text/template"

	"k8s.io/klog/v2"
)

//go:embed templates/reserved/subject.txt
var reservedSubjectTmpl string

//go:embed templates/reserved/body.txt
var reservedBodyTmpl string

//go:embed templates/session-result/subject.txt
var sessionResultSubjectTmpl string

//go:embed templates/session-result/body.txt
var sessionResultBodyTmpl string

//go:embed templates/change-summary/subject.txt
var changeSummarySubjectTmpl string

//go:embed templates/change-summary/body.txt
var changeSummaryBodyTmpl string

//go:embed templates/cancellation/subject.txt
var cancellationSubjectTmpl string

//go:embed templates/cancellation/body.txt
var cancellationBodyTmpl string

//go:embed templates/admin-notify/subject.txt
var adminNotifySubjectTmpl string

//go:embed templates/admin-notify/body.txt
var adminNotifyBodyTmpl string

//go:embed templates/admin-conflict/subject.txt
var adminConflictSubjectTmpl string

//go:embed templates/admin-conflict/body.txt
var adminConflictBodyTmpl string

//go:embed templates/retention-notify/subject.txt
var retentionNotifySubjectTmpl string

//go:embed templates/retention-notify/body.txt
var retentionNotifyBodyTmpl string

//go:embed templates/billing-invoice/subject.txt
var billingInvoiceSubjectTmpl string

//go:embed templates/billing-invoice/body.txt
var billingInvoiceBodyTmpl string

// Config holds the SMTP connection parameters.
type Config struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

// Sender is the central email dispatcher. Create one via NewSender and pass it
// to all subsystems that need to send email.
type Sender struct {
	cfg              Config
	templates        map[string]*templatePair
	fromNameMu       sync.RWMutex
	fromNameOverride string // set at runtime by the admin settings handler
}

// SetFromName updates the display name used in the From: header of all
// outgoing e-mails. It is safe to call concurrently and takes effect
// immediately for all subsequent sends. An empty name is ignored.
func (s *Sender) SetFromName(name string) {
	if name == "" {
		return
	}
	s.fromNameMu.Lock()
	s.fromNameOverride = name
	s.fromNameMu.Unlock()
}

// resolveFromName returns the current From display name: the runtime override
// if one has been set, otherwise the value from the initial Config.
func (s *Sender) resolveFromName() string {
	s.fromNameMu.RLock()
	override := s.fromNameOverride
	s.fromNameMu.RUnlock()
	if override != "" {
		return override
	}
	return s.cfg.FromName
}

// templatePair holds a compiled subject and body template.
type templatePair struct {
	subject *template.Template
	body    *template.Template
}

// NewSender constructs a Sender and compiles all embedded templates.
// It does not open an SMTP connection at construction time.
func NewSender(cfg Config) (*Sender, error) {
	pairs := map[string][2]string{
		"reserved":         {reservedSubjectTmpl, reservedBodyTmpl},
		"session-result":   {sessionResultSubjectTmpl, sessionResultBodyTmpl},
		"change-summary":   {changeSummarySubjectTmpl, changeSummaryBodyTmpl},
		"cancellation":     {cancellationSubjectTmpl, cancellationBodyTmpl},
		"admin-notify":     {adminNotifySubjectTmpl, adminNotifyBodyTmpl},
		"admin-conflict":   {adminConflictSubjectTmpl, adminConflictBodyTmpl},
		"retention-notify": {retentionNotifySubjectTmpl, retentionNotifyBodyTmpl},
		"billing-invoice":  {billingInvoiceSubjectTmpl, billingInvoiceBodyTmpl},
	}
	compiled := make(map[string]*templatePair, len(pairs))
	for name, raw := range pairs {
		subj, err := template.New(name + "/subject").Parse(raw[0])
		if err != nil {
			return nil, fmt.Errorf("email: parse %s subject template: %w", name, err)
		}
		body, err := template.New(name + "/body").Parse(raw[1])
		if err != nil {
			return nil, fmt.Errorf("email: parse %s body template: %w", name, err)
		}
		compiled[name] = &templatePair{subject: subj, body: body}
	}
	return &Sender{cfg: cfg, templates: compiled}, nil
}

// send is the low-level mail dispatch using implicit TLS (tls.Dial).
// to is a list of recipient addresses; subject and body are rendered strings.
func (s *Sender) send(_ context.Context, to []string, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	from := fmt.Sprintf("%s <%s>", s.resolveFromName(), s.cfg.FromAddress)

	header := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n",
		from, strings.Join(to, ", "), subject,
	)
	msg := []byte(header + body)

	tlsCfg := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         s.cfg.Host,
	}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("email: tls dial %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: smtp client: %w", err)
	}
	defer client.Quit() //nolint:errcheck

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("email: smtp auth: %w", err)
	}

	if err = client.Mail(s.cfg.FromAddress); err != nil {
		return fmt.Errorf("email: smtp MAIL FROM: %w", err)
	}
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("email: smtp RCPT TO %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email: smtp DATA: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("email: smtp write body: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("email: smtp close writer: %w", err)
	}
	klog.V(3).Infof("email: sent %q to %v", subject, to)
	return nil
}

// render executes the named template pair with data and returns (subject, body, err).
func (s *Sender) render(name string, data any) (string, string, error) {
	pair, ok := s.templates[name]
	if !ok {
		return "", "", fmt.Errorf("email: unknown template %q", name)
	}
	var subj, body bytes.Buffer
	if err := pair.subject.Execute(&subj, data); err != nil {
		return "", "", fmt.Errorf("email: render %s subject: %w", name, err)
	}
	if err := pair.body.Execute(&body, data); err != nil {
		return "", "", fmt.Errorf("email: render %s body: %w", name, err)
	}
	return strings.TrimSpace(subj.String()), body.String(), nil
}

// ValidateSMTP opens a TLS connection to the SMTP host to verify reachability.
func (s *Sender) ValidateSMTP() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	tlsCfg := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         s.cfg.Host,
	}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("email: cannot reach SMTP server at %s: %w", addr, err)
	}
	_ = conn.Close()
	return nil
}
