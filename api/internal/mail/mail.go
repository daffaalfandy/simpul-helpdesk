// Package mail renders and sends email.
package mail

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	texttemplate "text/template"
	"time"

	gomail "github.com/wneessen/go-mail"
)

const (
	// smtpsPort uses implicit TLS; other ports use STARTTLS (see TLSPolicy).
	smtpsPort   = 465
	sendTimeout = 15 * time.Second
	// localhostName is the loopback host name; loopback addresses are recognised by value.
	localhostName = "localhost"

	textTemplateGlob = "templates/*.txt.tmpl"
	htmlTemplateGlob = "templates/*.html.tmpl"
	textSuffix       = ".txt.tmpl"
	htmlSuffix       = ".html.tmpl"
	subjectSuffix    = ".subject" // each text template defines "<name>.subject"
)

// Template names (files in templates/).
const (
	TemplatePasswordReset = "password_reset"
	TemplateTicketUpdate  = "ticket_update"
	TemplateDigest        = "digest"
	TemplatePicBreach     = "sla_breach_pic"
	TemplateInvite        = "invite"
)

// Headers for automatic email: auto-responders stay quiet (RFC 3834, and Exchange's own
// header), and mail clients can offer a link to the notification settings (RFC 2369).
const (
	HeaderAutoSubmitted        = "Auto-Submitted"
	HeaderAutoResponseSuppress = "X-Auto-Response-Suppress"
	HeaderListUnsubscribe      = "List-Unsubscribe"
	AutoSubmittedGenerated     = "auto-generated"
	AutoResponseSuppressAll    = "All"
	listUnsubscribeFormat      = "<%s>"
)

// Template helpers shared by the text and HTML sets.
const (
	textFuncName        = "text"
	statusFuncName      = "status"
	colorFuncName       = "color"
	statusLabelTemplate = "label.status"
)

// AutoReplyHeaders keep auto-responders quiet on any automatic email.
func AutoReplyHeaders() map[string]string {
	return map[string]string{
		HeaderAutoSubmitted:        AutoSubmittedGenerated,
		HeaderAutoResponseSuppress: AutoResponseSuppressAll,
	}
}

// AutomaticHeaders are the headers of a notification email, with the settings page for
// List-Unsubscribe.
func AutomaticHeaders(settingsURL string) map[string]string {
	h := AutoReplyHeaders()
	h[HeaderListUnsubscribe] = fmt.Sprintf(listUnsubscribeFormat, settingsURL)
	return h
}

// PasswordResetData fills the password_reset template.
type PasswordResetData struct {
	Name      string
	OrgName   string
	Link      string
	ExpiresIn string
}

// InviteData fills the invite template: a new account's set-password link.
type InviteData struct {
	Name        string
	OrgName     string
	LogoURL     string // the organisation logo, absolute; empty without one
	InviterName string // empty when the system sent it
	Role        string // admin | supervisor | agent | requester
	ClientName  string // requesters' client
	Link        string
	ExpiresIn   string
}

type Message struct {
	To      string
	ToName  string
	Subject string
	Text    string
	HTML    string
	Headers map[string]string
}

type Sender interface {
	Send(ctx context.Context, m Message) error
}

type SMTPConfig struct {
	Addr     string // host:port
	User     string
	Pass     string
	From     string
	FromName string
	TLS      TLSPolicy
}

// TLSPolicy says whether STARTTLS is required on ports other than smtpsPort. Reset and invite
// links travel by email, so a relay reached over the network must not fall back to plain text
// when an attacker strips STARTTLS from its greeting.
type TLSPolicy string

const (
	// TLSAuto requires STARTTLS, except for a relay on this machine (a local Postfix, Mailpit).
	TLSAuto TLSPolicy = "auto"
	// TLSMandatory refuses to send without STARTTLS.
	TLSMandatory TLSPolicy = "mandatory"
	// TLSOpportunistic uses STARTTLS when the relay offers it.
	TLSOpportunistic TLSPolicy = "opportunistic"
	// TLSOff never uses STARTTLS (a relay on a private network without TLS).
	TLSOff TLSPolicy = "off"
)

// TLSPolicies lists the accepted values.
var TLSPolicies = []TLSPolicy{TLSAuto, TLSMandatory, TLSOpportunistic, TLSOff}

func (p TLSPolicy) Valid() bool { return slices.Contains(TLSPolicies, p) }

type SMTP struct {
	cfg  SMTPConfig
	host string
	port int
}

// NewSMTP validates the config and returns a sender.
func NewSMTP(cfg SMTPConfig) (*SMTP, error) {
	host, portStr, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("SMTP address must be host:port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("SMTP port: %w", err)
	}
	if cfg.TLS == "" {
		cfg.TLS = TLSAuto
	}
	if !cfg.TLS.Valid() {
		return nil, fmt.Errorf("SMTP TLS policy %q: use one of %v", cfg.TLS, TLSPolicies)
	}
	return &SMTP{cfg: cfg, host: host, port: port}, nil
}

func (s *SMTP) Send(ctx context.Context, m Message) error {
	msg := gomail.NewMsg()
	if err := msg.FromFormat(s.cfg.FromName, s.cfg.From); err != nil {
		return fmt.Errorf("from address: %w", err)
	}
	if err := msg.AddToFormat(m.ToName, m.To); err != nil {
		return fmt.Errorf("to address: %w", err)
	}
	msg.Subject(m.Subject)
	msg.SetMessageID()
	msg.SetDate()
	for name, value := range m.Headers {
		msg.SetGenHeader(gomail.Header(name), value)
	}
	msg.SetBodyString(gomail.TypeTextPlain, m.Text)
	if m.HTML != "" {
		msg.AddAlternativeString(gomail.TypeTextHTML, m.HTML)
	}

	opts := []gomail.Option{gomail.WithPort(s.port), gomail.WithTimeout(sendTimeout)}
	switch {
	case s.port == smtpsPort:
		opts = append(opts, gomail.WithSSL())
	case s.cfg.TLS == TLSOff:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	case s.cfg.TLS == TLSOpportunistic || (s.cfg.TLS == TLSAuto && isLoopback(s.host)):
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSOpportunistic))
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	}
	if s.cfg.User != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(s.cfg.User),
			gomail.WithPassword(s.cfg.Pass))
	}
	client, err := gomail.NewClient(s.host, opts...)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// isLoopback reports whether host names this machine.
func isLoopback(host string) bool {
	if host == localhostName {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

// Recorder is an in-memory Sender for tests. With Fail set, every send fails with it.
type Recorder struct {
	mu       sync.Mutex
	messages []Message
	Fail     error
}

func (r *Recorder) Send(_ context.Context, m Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Fail != nil {
		return r.Fail
	}
	r.messages = append(r.messages, m)
	return nil
}

// Messages returns a copy of everything sent so far.
func (r *Recorder) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.messages...)
}

//go:embed templates/*.tmpl
var templateFS embed.FS

var (
	textTemplates *texttemplate.Template
	htmlTemplates *htmltemplate.Template
)

// Both template sets share three helpers: text runs a text template block (so copy such as
// headlines and status labels lives in one place), status labels a ticket status for the
// recipient's audience, and color names a palette colour.
func init() {
	funcs := map[string]any{textFuncName: runText, statusFuncName: statusLabel, colorFuncName: color}
	textTemplates = texttemplate.Must(texttemplate.New("").Funcs(funcs).ParseFS(templateFS, textTemplateGlob))
	htmlTemplates = htmltemplate.Must(htmltemplate.New("").Funcs(funcs).ParseFS(templateFS, htmlTemplateGlob))
}

func runText(name string, data any) (string, error) {
	var b bytes.Buffer
	err := textTemplates.ExecuteTemplate(&b, name, data)
	return b.String(), err
}

// statusArgs feeds the label.status block.
type statusArgs struct {
	Status string
	Portal bool
}

func statusLabel(status string, portal bool) (string, error) {
	return runText(statusLabelTemplate, statusArgs{Status: status, Portal: portal})
}

// Render builds a message from templates/<name>.txt.tmpl and <name>.html.tmpl.
func Render(name, to, toName string, data any) (Message, error) {
	var subject, text, html bytes.Buffer
	if err := textTemplates.ExecuteTemplate(&subject, name+subjectSuffix, data); err != nil {
		return Message{}, fmt.Errorf("render %s subject: %w", name, err)
	}
	if err := textTemplates.ExecuteTemplate(&text, name+textSuffix, data); err != nil {
		return Message{}, fmt.Errorf("render %s text: %w", name, err)
	}
	if err := htmlTemplates.ExecuteTemplate(&html, name+htmlSuffix, data); err != nil {
		return Message{}, fmt.Errorf("render %s html: %w", name, err)
	}
	return Message{To: to, ToName: toName, Subject: subject.String(), Text: text.String(), HTML: html.String()}, nil
}

// day is a calendar day, for link lifetimes in email copy.
const day = 24 * time.Hour

// HumanDuration formats a whole number of days, hours or minutes for email copy.
func HumanDuration(d time.Duration) string {
	if d >= day && d%day == 0 {
		return plural(int(d/day), "day")
	}
	if d >= time.Hour && d%time.Hour == 0 {
		return plural(int(d/time.Hour), "hour")
	}
	return plural(int(d/time.Minute), "minute")
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
