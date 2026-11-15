package mail

import (
	"strings"
	"testing"
)

const noValue = "<no value>"

func TestTicketUpdateRenders(t *testing.T) {
	left := 41
	data := TicketUpdateData{
		OrgName: "Demo Vendor", Portal: true, Relation: RelationReported, TicketKey: "HD-24871",
		Title: "e-Office login fails", Priority: "P1", PriorityName: "Critical", ProjectName: "e-Office",
		Main:    NoticeData{Event: "status", Portal: true, StatusFrom: "in_progress", StatusTo: "in_uat", Time: "12:45"},
		Changes: []ChangeData{{Change: "assignee", Assignee: "Rina Pratiwi"}},
		Others: []NoticeData{{Event: "reply", Portal: true, ActorName: "Rina Pratiwi", Time: "12:40"},
			{Event: "sla", Kind: "at_risk", SLAClock: "resolution", SLAMinutesLeft: &left, Time: "12:41"}},
		Quote:     &QuoteData{Author: "Rina Pratiwi", FromSupport: true, Time: "12:45", Body: "The fix is on the test server now."},
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
		"Powered by SimDesk",
	} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, msg.Text)
		}
	}
	for _, want := range []string{"Ready for your test", "View the ticket", data.SettingsURL, "Also changed"} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("html lacks %q", want)
		}
	}
	if strings.Contains(msg.Text+msg.HTML, noValue) {
		t.Error("a field rendered as <no value>")
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
