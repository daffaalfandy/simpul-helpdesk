package tickets

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// branchSyncBatchSize is how many tickets one page of a branch re-check reads.
const branchSyncBatchSize = 100

// DoDBranchSyncer ticks "code on the branch" on a project's tickets whose linked commits already
// reached the branch its Definition of Done now names (projects.DoDBranchSyncArgs). It reads the
// project's current Definition of Done, and each ticket again under its lock, so running it twice
// or late changes nothing more. Tickets with their own Definition of Done are re-checked when it
// is set.
type DoDBranchSyncer struct {
	river.WorkerDefaults[projects.DoDBranchSyncArgs]
	svc *Service
	log *slog.Logger
}

func NewDoDBranchSyncer(svc *Service, log *slog.Logger) *DoDBranchSyncer {
	return &DoDBranchSyncer{svc: svc, log: log}
}

func (w *DoDBranchSyncer) Work(ctx context.Context, job *river.Job[projects.DoDBranchSyncArgs]) error {
	return w.svc.syncBranch(ctx, w.log, job.Args.OrganizationID, job.Args.ProjectID)
}

func (s *Service) syncBranch(ctx context.Context, log *slog.Logger, orgID, projectID uuid.UUID) error {
	q := s.store.Q()
	p, err := q.GetProject(ctx, db.GetProjectParams{ID: projectID, OrganizationID: orgID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // deleted since
	}
	if err != nil {
		return err
	}
	cfg, err := dod.Parse(p.DodConfig)
	if err != nil {
		return err
	}
	if !cfg.CodeOnBranch.Enabled || p.RepositoryID == nil {
		return nil // turned off or unlinked since
	}
	var after *uuid.UUID
	for {
		ids, err := q.ListBranchSyncTickets(ctx, db.ListBranchSyncTicketsParams{
			OrganizationID: orgID, ProjectID: projectID, ClosedStatus: string(ticket.StatusClosed),
			Item: string(dod.ItemCodeOnBranch), LinkedOutcome: string(CodeLinked), RepositoryID: *p.RepositoryID,
			Branch: cfg.CodeOnBranch.Branch, After: after, RowLimit: branchSyncBatchSize,
		})
		if err != nil {
			return err
		}
		for _, id := range ids {
			// One ticket that can't be checked must not stop the others.
			if err := s.syncTicketBranch(ctx, id); err != nil {
				log.ErrorContext(ctx, "dod branch sync: ticket skipped", "ticket_id", id, "error", err)
			}
		}
		if len(ids) < branchSyncBatchSize {
			return nil
		}
		last := ids[len(ids)-1]
		after = &last
	}
}

// syncTicketBranch re-checks one ticket under its lock against its project's current
// Definition of Done.
func (s *Service) syncTicketBranch(ctx context.Context, ticketID uuid.UUID) error {
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := tx.Q.GetTicketForCode(ctx, ticketID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.RepositoryID == nil || row.DodOverride != nil || ticket.Status(row.Status).Locked() {
			return nil
		}
		sd, err := parseDoD(row.DodConfig, nil)
		if err != nil {
			return err
		}
		cs := newChangeSet(row.ID)
		if err := tickLinkedCommit(ctx, tx.Q, cs, row.OrganizationID, *row.RepositoryID, sd.evaluated(true)); err != nil {
			return err
		}
		return s.commit(ctx, tx, cs)
	})
}
