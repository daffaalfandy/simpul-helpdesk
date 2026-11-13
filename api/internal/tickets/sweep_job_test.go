package tickets

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/storage"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

func TestSweeperPurgesOnlyExpiredPendingFiles(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	files, err := storage.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close() })
	pool := st.Pool()

	var client, user, ticketID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO clients (name) VALUES ('C') RETURNING id`).Scan(&client); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, name, role, client_id)
		VALUES ('r@c.test', 'R', 'requester', $1) RETURNING id`, client).Scan(&user); err != nil {
		t.Fatal(err)
	}
	project := testdb.Project(t, st, client, "e-Office", true)
	if err := pool.QueryRow(ctx, `INSERT INTO tickets (number, key, type, status, priority, channel, client_id,
		requester_id, created_by, project_id, subject, description, sla_snapshot)
		VALUES (1, 'HD-1', 'incident', 'new', 'P3', 'portal', $1, $2, $2, $3, 'S', 'D', '{}') RETURNING id`,
		client, user, project).Scan(&ticketID); err != nil {
		t.Fatal(err)
	}

	// put stores a file and its row; age backdates the upload, linked attaches it to the ticket.
	put := func(age time.Duration, linked bool) string {
		key := storage.NewKey()
		if _, err := files.Put(ctx, key, bytes.NewReader(pngHead)); err != nil {
			t.Fatal(err)
		}
		var tid *uuid.UUID
		if linked {
			tid = &ticketID
		}
		if _, err := pool.Exec(ctx, `INSERT INTO attachments (storage_key, filename, content_type, size_bytes, sha256,
			uploaded_by, ticket_id, linked_at, created_at)
			VALUES ($1, 'a.png', 'image/png', 10, decode(repeat('ab', 32), 'hex'), $2, $3,
			        CASE WHEN $3::uuid IS NOT NULL THEN now() END, now() - $4::interval)`,
			key, user, tid, age.String()); err != nil {
			t.Fatal(err)
		}
		return key
	}
	fresh := put(time.Hour, false)
	expired := put(PendingAttachmentTTL+time.Hour, false)
	oldLinked := put(PendingAttachmentTTL+time.Hour, true)

	w := NewAttachmentSweeper(st, files)
	if err := w.Work(ctx, &river.Job[PurgePendingAttachmentsArgs]{}); err != nil {
		t.Fatal(err)
	}
	exists := func(key string) bool {
		_, _, err := files.Open(ctx, key)
		if err == nil {
			return true
		}
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatal(err)
		}
		return false
	}
	if !exists(fresh) || exists(expired) || !exists(oldLinked) {
		t.Fatalf("files: fresh %v, expired %v, old linked %v", exists(fresh), exists(expired), exists(oldLinked))
	}
	var rows, purged int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM attachments`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("rows = %d, %v", rows, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = $1 AND actor_name = $2`,
		ActionAttachmentsPurged, ActorNameAttachmentSweep).Scan(&purged); err != nil || purged != 1 {
		t.Fatalf("purge audit entries = %d, %v", purged, err)
	}
}
