package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

const (
	constraintTicketTypeDetail      = "tickets_type_detail"
	constraintTicketNewUnassigned   = "tickets_new_unassigned"
	constraintTicketAssignedHas     = "tickets_assigned_has_assignee"
	constraintTicketResolvedAt      = "tickets_resolved_at"
	constraintTicketClosedAt        = "tickets_closed_at"
	constraintTicketKeyFormat       = "tickets_key_format"
	constraintTicketKeyMatches      = "tickets_key_matches_number"
	constraintTicketClientFK        = "tickets_client_id_fkey"
	constraintTicketSystemFK        = "tickets_system_id_fkey"
	constraintTicketCatalogItemFK   = "tickets_catalog_item_id_fkey"
	constraintAttachmentMessageFK   = "attachments_message_fkey"
	constraintAttachmentNeedsTicket = "attachments_message_needs_ticket"
	constraintAttachmentLinkedAt    = "attachments_linked_at"
	constraintSystemName            = "systems_name_key"
	constraintCatalogItemName       = "catalog_items_name_key"
)

// ticketFixture is a client with a requester and an agent, plus one system and one catalogue item.
type ticketFixture struct {
	pool                     *pgxpool.Pool
	client, requester, agent uuid.UUID
	system, catalogItem      uuid.UUID
	nextNumber               int
}

func newTicketFixture(t *testing.T) *ticketFixture {
	t.Helper()
	ctx := context.Background()
	f := &ticketFixture{pool: testdb.New(t).Pool(), nextNumber: 1}
	scan := func(dst *uuid.UUID, sql string, args ...any) {
		t.Helper()
		if err := f.pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	scan(&f.client, `INSERT INTO clients (name) VALUES ('C') RETURNING id`)
	scan(&f.requester, `INSERT INTO users (email, name, role, client_id) VALUES ('r@c.test', 'R', 'requester', $1) RETURNING id`, f.client)
	scan(&f.agent, `INSERT INTO users (email, name, role) VALUES ('a@v.test', 'A', 'agent') RETURNING id`)
	scan(&f.system, `INSERT INTO systems (name) VALUES ('e-Office') RETURNING id`)
	scan(&f.catalogItem, `INSERT INTO catalog_items (name, delivery_minutes) VALUES ('VPN', 540) RETURNING id`)
	return f
}

// ticket is the column set a test overrides; zero values take valid defaults.
type ticket struct {
	key, typ, status    string
	number              int
	client, assignee    *uuid.UUID
	system, catalogItem *uuid.UUID
	resolved, closed    bool
}

func (f *ticketFixture) insert(ctx context.Context, tk ticket) (uuid.UUID, error) {
	if tk.number == 0 {
		tk.number = f.nextNumber
		f.nextNumber++
	}
	if tk.key == "" {
		tk.key = fmt.Sprintf("HD-%d", tk.number)
	}
	if tk.typ == "" {
		tk.typ = "incident"
	}
	if tk.status == "" {
		tk.status = "new"
	}
	if tk.client == nil {
		tk.client = &f.client
	}
	var id uuid.UUID
	err := f.pool.QueryRow(ctx, `
		INSERT INTO tickets (number, key, type, status, priority, channel, client_id, requester_id,
		                     assignee_id, created_by, system_id, catalog_item_id, subject, description,
		                     sla_snapshot, resolved_at, closed_at)
		VALUES ($1, $2, $3, $4, 'P3', 'portal', $5, $6, $7, $6, $8, $9, 'S', 'D', '{}',
		        CASE WHEN $10 THEN now() END, CASE WHEN $11 THEN now() END)
		RETURNING id`,
		tk.number, tk.key, tk.typ, tk.status, *tk.client, f.requester, tk.assignee, tk.system, tk.catalogItem,
		tk.resolved, tk.closed).Scan(&id)
	return id, err
}

func (f *ticketFixture) mustInsert(t *testing.T, tk ticket) uuid.UUID {
	t.Helper()
	id, err := f.insert(context.Background(), tk)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTicketConstraints(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	agent := &f.agent

	valid := map[string]ticket{
		"new incident with system": {system: &f.system},
		"request with catalogue":   {typ: "request", catalogItem: &f.catalogItem},
		"assigned":                 {status: "assigned", assignee: agent},
		"in progress unassigned":   {status: "in_progress"},
		"waiting on client":        {status: "waiting_on_client", assignee: agent},
		"in uat":                   {status: "in_uat", assignee: agent},
		"resolved":                 {status: "resolved", resolved: true},
		"closed":                   {status: "closed", resolved: true, closed: true},
		"long prefix":              {key: "ABCDEFGHIJ-77", number: 77},
	}
	for name, tk := range valid {
		if _, err := f.insert(ctx, tk); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := map[string]struct {
		tk         ticket
		constraint string
	}{
		"incident with catalogue":   {ticket{catalogItem: &f.catalogItem}, constraintTicketTypeDetail},
		"request with system":       {ticket{typ: "request", system: &f.system}, constraintTicketTypeDetail},
		"new with assignee":         {ticket{assignee: agent}, constraintTicketNewUnassigned},
		"assigned without assignee": {ticket{status: "assigned"}, constraintTicketAssignedHas},
		"resolved without time":     {ticket{status: "resolved"}, constraintTicketResolvedAt},
		"open with resolved time":   {ticket{status: "in_progress", resolved: true}, constraintTicketResolvedAt},
		"closed without time":       {ticket{status: "closed", resolved: true}, constraintTicketClosedAt},
		"lowercase key":             {ticket{key: "hd-500", number: 500}, constraintTicketKeyFormat},
		"key without number":        {ticket{key: "HD-", number: 501}, constraintTicketKeyFormat},
		"key number mismatch":       {ticket{key: "HD-502", number: 503}, constraintTicketKeyMatches},
		"key suffix mismatch":       {ticket{key: "HD-1504", number: 504}, constraintTicketKeyMatches},
	}
	for name, c := range invalid {
		_, err := f.insert(ctx, c.tk)
		if !store.IsViolation(err, store.SQLStateCheckViolation, c.constraint) {
			t.Errorf("%s: err = %v, want %s", name, err, c.constraint)
		}
	}

	for name, sql := range map[string]string{
		"status":   `UPDATE tickets SET status = 'done' WHERE number = 1`,
		"type":     `UPDATE tickets SET type = 'problem' WHERE number = 1`,
		"priority": `UPDATE tickets SET priority = 'P5' WHERE number = 1`,
		"channel":  `UPDATE tickets SET channel = 'fax' WHERE number = 1`,
		"snapshot": `UPDATE tickets SET sla_snapshot = '[]' WHERE number = 1`,
		"subject":  `UPDATE tickets SET subject = '' WHERE number = 1`,
	} {
		if _, err := f.pool.Exec(ctx, sql); pgCode(err) != codeCheckViolation {
			t.Errorf("%s: err = %v, want check violation", name, err)
		}
	}
}

func TestTicketsBlockDeletingWhatTheyUse(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	// A client with tickets but no users, so only the tickets block the delete.
	var bare uuid.UUID
	if err := f.pool.QueryRow(ctx, `INSERT INTO clients (name) VALUES ('Bare') RETURNING id`).Scan(&bare); err != nil {
		t.Fatal(err)
	}
	f.mustInsert(t, ticket{client: &bare, system: &f.system})
	f.mustInsert(t, ticket{typ: "request", catalogItem: &f.catalogItem})

	for constraint, sql := range map[string]string{
		constraintTicketClientFK:      `DELETE FROM clients WHERE name = 'Bare'`,
		constraintTicketSystemFK:      `DELETE FROM systems`,
		constraintTicketCatalogItemFK: `DELETE FROM catalog_items`,
	} {
		_, err := f.pool.Exec(ctx, sql)
		if !store.IsViolation(err, store.SQLStateRestrictViolation, constraint) {
			t.Errorf("%s: err = %v", constraint, err)
		}
	}
}

func TestTicketNumbersAreUnique(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	f.mustInsert(t, ticket{number: 9})
	if _, err := f.insert(ctx, ticket{number: 9, key: "XY-9"}); pgCode(err) != codeUniqueViolation {
		t.Fatalf("duplicate number: err = %v", err)
	}
	var a, b int64
	if err := f.pool.QueryRow(ctx, `SELECT nextval('ticket_number_seq'), nextval('ticket_number_seq')`).Scan(&a, &b); err != nil {
		t.Fatal(err)
	}
	if b != a+1 {
		t.Fatalf("sequence gave %d then %d", a, b)
	}
}

func TestAttachmentLinks(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	t1 := f.mustInsert(t, ticket{})
	t2 := f.mustInsert(t, ticket{})
	var msg uuid.UUID
	if err := f.pool.QueryRow(ctx, `INSERT INTO ticket_messages (ticket_id, author_id, visibility, body)
		VALUES ($1, $2, 'public', 'hi') RETURNING id`, t1, f.requester).Scan(&msg); err != nil {
		t.Fatal(err)
	}
	insert := func(key string, ticketID, messageID *uuid.UUID, linked bool) error {
		_, err := f.pool.Exec(ctx, `INSERT INTO attachments (storage_key, filename, content_type, size_bytes,
			sha256, uploaded_by, ticket_id, message_id, linked_at)
			VALUES ($1, 'a.png', 'image/png', 10, decode(repeat('ab', 32), 'hex'), $2, $3, $4,
			        CASE WHEN $5 THEN now() END)`, key, f.requester, ticketID, messageID, linked)
		return err
	}
	if err := insert("pending", nil, nil, false); err != nil {
		t.Errorf("pending: %v", err)
	}
	if err := insert("description", &t1, nil, true); err != nil {
		t.Errorf("description: %v", err)
	}
	if err := insert("message", &t1, &msg, true); err != nil {
		t.Errorf("message: %v", err)
	}
	if err := insert("other ticket", &t2, &msg, true); !store.IsViolation(err, store.SQLStateForeignKeyViolation, constraintAttachmentMessageFK) {
		t.Errorf("message of another ticket: err = %v", err)
	}
	if err := insert("no ticket", nil, &msg, false); !store.IsViolation(err, store.SQLStateCheckViolation, constraintAttachmentNeedsTicket) {
		t.Errorf("message without ticket: err = %v", err)
	}
	if err := insert("unlinked time", &t1, nil, false); !store.IsViolation(err, store.SQLStateCheckViolation, constraintAttachmentLinkedAt) {
		t.Errorf("linked without time: err = %v", err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE attachments SET size_bytes = 10485761 WHERE storage_key = 'pending'`); pgCode(err) != codeCheckViolation {
		t.Errorf("over 10 MiB: err = %v", err)
	}
}

func TestCatalogNamesAreCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	_, err := f.pool.Exec(ctx, `INSERT INTO systems (name) VALUES ('E-OFFICE')`)
	if !store.IsViolation(err, store.SQLStateUniqueViolation, constraintSystemName) {
		t.Errorf("system: err = %v", err)
	}
	_, err = f.pool.Exec(ctx, `INSERT INTO catalog_items (name, delivery_minutes) VALUES ('vpn', 60)`)
	if !store.IsViolation(err, store.SQLStateUniqueViolation, constraintCatalogItemName) {
		t.Errorf("catalogue item: err = %v", err)
	}
}
