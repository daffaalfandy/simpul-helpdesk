// Package mail renders and sends email.
package mail

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"net"
	"strconv"
	"sync"
	texttemplate "text/template"
	"time"

	gomail "github.com/wneessen/go-mail"
)

const (
	// smtpsPort uses implicit TLS; other ports use STARTTLS when offered.
	smtpsPort   = 465
	sendTimeout = 15 * time.Second

	textTemplateGlob = "templates/*.txt.tmpl"
	htmlTemplateGlob = "templates/*.html.tmpl"
	textSuffix       = ".txt.tmpl"
	htmlSuffix       = ".html.tmpl"
	subjectSuffix    = ".subject" // each text template defines "<name>.subject"
)

// Template names (files in templates/).
const (
	TemplatePasswordReset = "password_reset"
)

// PasswordResetData fills the password_reset template.
type PasswordResetData struct {
	Name      string
	OrgName   string
	Link      string
	ExpiresIn string
}

// Message is a rendered email.
type Message struct {
	To      string
	ToName  string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers messages.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTPConfig configures the SMTP sender.
type SMTPConfig struct {
	Addr     string // host:port
	User     string
	Pass     string
	From     string
	FromName string
}

// SMTP sends mail through an SMTP server.
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
	return &SMTP{cfg: cfg, host: host, port: port}, nil
}

// Send delivers one message.
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
	msg.SetBodyString(gomail.TypeTextPlain, m.Text)
	if m.HTML != "" {
		msg.AddAlternativeString(gomail.TypeTextHTML, m.HTML)
	}

	opts := []gomail.Option{gomail.WithPort(s.port), gomail.WithTimeout(sendTimeout)}
	if s.port == smtpsPort {
		opts = append(opts, gomail.WithSSL())
	} else {
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSOpportunistic))
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

// Recorder is an in-memory Sender for tests.
type Recorder struct {
	mu       sync.Mutex
	messages []Message
}

// Send records the message.
func (r *Recorder) Send(_ context.Context, m Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	textTemplates = texttemplate.Must(texttemplate.ParseFS(templateFS, textTemplateGlob))
	htmlTemplates = htmltemplate.Must(htmltemplate.ParseFS(templateFS, htmlTemplateGlob))
)

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

// HumanDuration formats a whole number of hours or minutes for email copy.
func HumanDuration(d time.Duration) string {
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
