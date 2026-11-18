package mail

import (
	"strings"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/richtext"
)

const (
	noValue = "<no value>"
	quoteMD = "The fix is on **the test server** now."
)

func TestTicketUpdateRenders(t *testing.T) {
	left := 41
	data := TicketUpdateData{
		OrgName: "Demo Vendor", Portal: true, Relation: RelationReported, TicketKey: "HD-24871",
		Title: "e-Office login fails", Priority: "P1", PriorityName: "Critical", ProjectName: "e-Office",
		Main:    NoticeData{Event: "status", Portal: true, StatusFrom: "in_progress", StatusTo: "in_uat", Time: "12:45"},
		Changes: []ChangeData{{Change: "assignee", Assignee: "Rina Pratiwi"}},
		Others: []NoticeData{{Event: "reply", Portal: true, ActorName: "Rina Pratiwi", Time: "12:40"},
			{Event: "sla", Kind: "at_risk", SLAClock: "resolution", SLAMinutesLeft: &left, Time: "12:41"}},
		Quote: &QuoteData{Author: "Rina Pratiwi", FromSupport: true, Time: "12:45", Body: richtext.Text(quoteMD),
			BodyHTML: richtext.HTML(quoteMD)},
		SLA:       &SLALineData{TargetValue: 4, TargetUnit: "h", PauseUAT: true},
		TicketURL: "https://helpdesk.example/portal/tickets/HD-24871", SettingsURL: "https://helpdesk.example/portal/notifications#settings",
	}
	msg, err := Render(TemplateTicketUpdate, "hendra@example.test", "Hendra", data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[HD-24871] Ready for your test: e-Office login fails"; msg.Subject != want {
		t.Errorf("subject = %q, want %q", msg.Subject, want)
	}
	for _, want := range []string{
		"Status changed: In progress → Ready for your test",
		"Rina Pratiwi · Support team · 12:45",
		"- Assigned to Rina Pratiwi",
		"- Rina Pratiwi replied (12:40)",
		"- SLA at risk: resolve due in 41 min (12:41)",
		"You're getting this because you reported HD-24871. Reply in the portal; replies to this email aren't read.",
		"HD-24871 · P1 Critical · e-Office",
		"Fix target: 4 hours · time waiting for your test doesn't count",
		"Powered by SimDesk",
		"The fix is on the test server now.",
	} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, msg.Text)
		}
	}
	for _, want := range []string{"Ready for your test", "View the ticket", data.SettingsURL, "Also changed",
		"<strong>the test server</strong>"} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("html lacks %q", want)
		}
	}
	if strings.Contains(msg.Text+msg.HTML, noValue) {
		t.Error("a field rendered as <no value>")
	}
}

// A test request asks "Does it work now?" with links that open the answer's dialog; a
// resolved ticket says until when it can be reopened.
func TestUATAndReopenRender(t *testing.T) {
	approve := "https://helpdesk.example/portal/tickets/HD-24871?answer=approve"
	reject := "https://helpdesk.example/portal/tickets/HD-24871?answer=reject"
	data := TicketUpdateData{
		OrgName: "Demo Vendor", Portal: true, Relation: RelationReported, TicketKey: "HD-24871",
		Title: "e-Office login fails", Priority: "P1", PriorityName: "Critical", ProjectName: "e-Office",
		Main: NoticeData{Event: "uat_requested", Portal: true, ActorName: "Rina Pratiwi", StatusFrom: "in_progress",
			StatusTo: "in_uat", Time: "12:45"},
		UAT:       &UATData{ApproveURL: approve, RejectURL: reject, AutoApproveOn: "18 Nov"},
		TicketURL: "https://helpdesk.example/portal/tickets/HD-24871", SettingsURL: "s",
	}
	msg, err := Render(TemplateTicketUpdate, "hendra@example.test", "Hendra", data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[HD-24871] Ready for your test: e-Office login fails"; msg.Subject != want {
		t.Errorf("subject = %q", msg.Subject)
	}
	for _, want := range []string{
		"Rina Pratiwi asked you to test the fix", "Does it work now?", "Approve, it works: " + approve,
		"Still a problem: " + reject, "If we don't hear back, the fix is approved automatically on 18 Nov.",
	} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, msg.Text)
		}
	}
	for _, want := range []string{"Does it work now?", `href="` + approve + `"`, `href="` + reject + `"`, "Approve, it works"} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("html lacks %q", want)
		}
	}

	data.UAT = nil
	data.Main = NoticeData{Event: "resolved", Portal: true, StatusFrom: "in_uat", StatusTo: "resolved", ReopenUntil: "19 Nov"}
	msg, err = Render(TemplateTicketUpdate, "hendra@example.test", "Hendra", data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "You can reopen this ticket until 19 Nov if the problem comes back."; !strings.Contains(msg.Text, want) || !strings.Contains(msg.HTML, want) {
		t.Errorf("reopen line missing:\n%s", msg.Text)
	}
	if strings.Contains(msg.Text, "Does it work now?") || strings.Contains(msg.Text+msg.HTML, noValue) {
		t.Errorf("resolved email:\n%s", msg.Text)
	}

	data.Portal, data.Relation = false, RelationSupervisorDoD
	data.Main = NoticeData{Event: "dod", Kind: "skipped", ActorName: "Rina Pratiwi", Reason: "Client confirmed by phone"}
	msg, err = Render(TemplateTicketUpdate, "dewi@example.test", "Dewi", data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[HD-24871] Resolved without its Definition of Done: e-Office login fails",
		"Rina Pratiwi resolved this ticket without meeting its Definition of Done (Client confirmed by phone)",
		"supervisors hear when a Definition of Done is skipped or changed"} {
		if !strings.Contains(msg.Subject+msg.Text, want) {
			t.Errorf("supervisor email lacks %q:\n%s\n%s", want, msg.Subject, msg.Text)
		}
	}
}

func TestTeamWordingAndRelations(t *testing.T) {
	data := TicketUpdateData{
		OrgName: "Demo Vendor", Relation: RelationAssigned, TicketKey: "HD-1", Title: "VPN",
		Main:      NoticeData{Event: "status", StatusFrom: "in_progress", StatusTo: "waiting_on_client"},
		TicketURL: "u", SettingsURL: "s",
	}
	msg, err := Render(TemplateTicketUpdate, "a@example.test", "A", data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[HD-1] Status changed: VPN", "In progress → Waiting on client", "you're assigned to HD-1", "Reply in SimDesk"} {
		if !strings.Contains(msg.Subject+msg.Text, want) {
			t.Errorf("lacks %q:\n%s\n%s", want, msg.Subject, msg.Text)
		}
	}
}

// Supervisors hear about a ticket nobody is assigned to, and why they get the email.
func TestNewTicketRenders(t *testing.T) {
	data := TicketUpdateData{
		OrgName: "Demo Vendor", Relation: RelationSupervisorUnassigned, TicketKey: "HD-7", Title: "VPN down",
		Priority: "P2", PriorityName: "High", ProjectName: "Network",
		Main:      NoticeData{Event: "new_ticket", ActorName: "Hendra Gunawan", Time: "09:12"},
		TicketURL: "https://helpdesk.example/tickets/HD-7", SettingsURL: "s",
	}
	msg, err := Render(TemplateTicketUpdate, "dewi@example.test", "Dewi", data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[HD-7] New ticket: VPN down"; msg.Subject != want {
		t.Errorf("subject = %q, want %q", msg.Subject, want)
	}
	for _, want := range []string{
		"Hendra Gunawan reported this ticket; nobody is assigned yet",
		"You're getting this because supervisors hear about tickets nobody is assigned to.",
		"View the ticket: https://helpdesk.example/tickets/HD-7",
	} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, msg.Text)
		}
	}
	if !strings.Contains(msg.HTML, "supervisors hear about tickets nobody is assigned to") || strings.Contains(msg.Text+msg.HTML, noValue) {
		t.Errorf("html:\n%s", msg.HTML)
	}
}

// A question for the client says the ticket waits for them, quotes the team's reply or says
// where to look, and offers to reply.
func TestQuestionRenders(t *testing.T) {
	url := "https://helpdesk.example/portal/tickets/HD-7"
	data := TicketUpdateData{
		OrgName: "Demo Vendor", Portal: true, Relation: RelationReported, TicketKey: "HD-7", Title: "VPN down",
		Main: NoticeData{Event: "question", Portal: true, ActorName: "Rina Pratiwi", StatusFrom: "in_progress",
			StatusTo: "waiting_on_client", Time: "09:12"},
		TicketURL: url, SettingsURL: "s",
	}
	msg, err := Render(TemplateTicketUpdate, "hendra@example.test", "Hendra", data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[HD-7] We're waiting for your reply: VPN down"; msg.Subject != want {
		t.Errorf("subject = %q, want %q", msg.Subject, want)
	}
	fallback := "Open the ticket to see what the team needs from you."
	for _, want := range []string{"Rina Pratiwi needs your reply or confirmation to continue", fallback, "Reply now: " + url} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, msg.Text)
		}
	}
	for _, want := range []string{fallback, `href="` + url + `"`, "Reply now"} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("html lacks %q", want)
		}
	}
	if strings.Contains(msg.Text+msg.HTML, "View the ticket") || strings.Contains(msg.Text+msg.HTML, noValue) {
		t.Errorf("question email:\n%s", msg.Text)
	}

	data.Quote = &QuoteData{Author: "Rina Pratiwi", FromSupport: true, Time: "09:14", Body: "Which office?", BodyHTML: "<p>Which office?</p>"}
	msg, err = Render(TemplateTicketUpdate, "hendra@example.test", "Hendra", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Text, "Which office?") || strings.Contains(msg.Text+msg.HTML, fallback) {
		t.Errorf("quoted question:\n%s", msg.Text)
	}
}

func TestCodeActivityRenders(t *testing.T) {
	tests := []struct {
		code CodeData
		want string
	}{
		{CodeData{Provider: "github", Kind: "commit", Ref: "a1f9c2e", Branch: "fix/HD-1", Outcome: "linked"}, "GitHub: commit a1f9c2e on fix/HD-1 mentions this ticket"},
		{CodeData{Provider: "gitlab", Kind: "commit", Ref: "a1f9c2e", Branch: "main", Outcome: "linked"}, "GitLab: commit a1f9c2e on main mentions this ticket"},
		// Notifications from before 17 Nov keep their wording, outcome aside.
		{CodeData{Provider: "github", Kind: "commit", Ref: "a1f9c2e", Branch: "fix/HD-1", Outcome: "fix_committed"}, "GitHub: commit a1f9c2e on fix/HD-1 mentions this ticket"},
		{CodeData{Provider: "github", Kind: "pr_merged", Ref: "142", Target: "main", Outcome: "merged"}, "GitHub: PR #142 merged into main"},
		{CodeData{Provider: "gitlab", Kind: "pr_opened", Ref: "57", Target: "release/2026.11", Outcome: "in_review"}, "GitLab: MR !57 opened → release/2026.11"},
		{CodeData{Provider: "gitlab", Kind: "pipeline", Ref: "31", Branch: "main", Outcome: "ci_passed"}, "GitLab: CI passed on main"},
		{CodeData{Provider: "github", Kind: "deployment", Ref: "production", Outcome: "deployed"}, "GitHub: deployed to production"},
	}
	for _, tt := range tests {
		code := tt.code
		data := TicketUpdateData{
			OrgName: "Demo Vendor", Relation: RelationAssigned, TicketKey: "HD-1", Title: "VPN",
			Main: NoticeData{Event: "code", Code: &code}, TicketURL: "u", SettingsURL: "s",
		}
		msg, err := Render(TemplateTicketUpdate, "a@example.test", "A", data)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(msg.Text, tt.want) || !strings.Contains(msg.Subject, "Code activity") {
			t.Errorf("lacks %q:\n%s\n%s", tt.want, msg.Subject, msg.Text)
		}
	}
}

func TestDigestRenders(t *testing.T) {
	data := DigestData{
		OrgName: "Demo Vendor", Count: 2, SettingsURL: "s",
		Tickets: []DigestTicket{{TicketKey: "HD-1", Title: "VPN", PriorityName: "High", ProjectName: "Network", TicketURL: "u",
			Notices: []NoticeData{
				{Event: "comment", ActorName: "Hendra", Time: "09:12"},
				{Event: "priority", ActorName: "Dewi", PriorityFrom: "P3", PriorityTo: "P2", Reason: "Whole office", Time: "09:40"},
			}}},
	}
	msg, err := Render(TemplateDigest, "a@example.test", "A", data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Demo Vendor Support: Your hourly summary", "2 updates on 1 ticket", "- Hendra replied (09:12)",
		"- Dewi changed priority: P3 → P2 (Whole office) (09:40)"} {
		if !strings.Contains(msg.Subject+"\n"+msg.Text, want) {
			t.Errorf("lacks %q:\n%s\n%s", want, msg.Subject, msg.Text)
		}
	}
	if strings.Contains(msg.Text+msg.HTML, noValue) {
		t.Error("a field rendered as <no value>")
	}
}

func TestAutomaticHeaders(t *testing.T) {
	h := AutomaticHeaders("https://helpdesk.example/notifications#settings")
	if h[HeaderAutoSubmitted] != AutoSubmittedGenerated || h[HeaderListUnsubscribe] != "<https://helpdesk.example/notifications#settings>" {
		t.Errorf("headers = %v", h)
	}
}

func TestHumanDuration(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want string
	}{
		{7 * 24 * time.Hour, "7 days"},
		{24 * time.Hour, "1 day"},
		{36 * time.Hour, "36 hours"},
		{time.Hour, "1 hour"},
		{30 * time.Minute, "30 minutes"},
	} {
		if got := HumanDuration(tc.in); got != tc.want {
			t.Errorf("HumanDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSMTPTLSPolicy(t *testing.T) {
	for _, tt := range []struct {
		host string
		want bool
	}{
		{"localhost", true}, {"127.0.0.1", true}, {"::1", true},
		{"smtp.example.test", false}, {"10.0.0.5", false}, {"mailpit", false},
	} {
		if got := isLoopback(tt.host); got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
	if s, err := NewSMTP(SMTPConfig{Addr: "smtp.example.test:587"}); err != nil || s.cfg.TLS != TLSAuto {
		t.Fatalf("an unset policy = %v, %v; want auto", s, err)
	}
	if _, err := NewSMTP(SMTPConfig{Addr: "smtp.example.test:587", TLS: "sometimes"}); err == nil {
		t.Fatal("an unknown policy is accepted")
	}
}
