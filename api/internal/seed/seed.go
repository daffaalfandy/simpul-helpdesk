// Package seed loads demo data for local development (HANDOFF §16).
package seed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/google/uuid"

	"simdesk/internal/auth"
	"simdesk/internal/config"
	"simdesk/internal/db"
	"simdesk/internal/store"
)

// Output table layout.
const (
	tableMinWidth = 0
	tableTabWidth = 0
	tablePadding  = 2
	tablePadChar  = ' '
)

const (
	// DevPassword is the password of every seeded user.
	DevPassword = "simdesk-dev"
	// OrgName is the demo organisation's name.
	OrgName = "Demo Vendor"
	// orgFooterNote is the demo data-residency line shown in the footer.
	orgFooterNote = "Data stored in Indonesia · Personal data handled under UU PDP"
	// RequesterClient is the client the demo requester belongs to.
	RequesterClient = "Dinas Kominfo Jambi"
)

// ErrNotDev is returned when seeding outside development.
var ErrNotDev = errors.New("seed only runs when APP_ENV=" + config.EnvDev)

var clients = []string{
	RequesterClient,
	"BPKAD Kota Batam",
	"Bappeda Prov. Riau",
	"Diskominfo Kab. Bintan",
	"Kementerian PANRB",
	"Setda Kab. Muaro Jambi",
	"Sekretariat DPRD Kota Batam",
	"Disdukcapil Kab. Muaro Jambi",
}

type person struct {
	email, name string
	role        auth.Role
	client      string
	jobTitle    string
}

var people = []person{
	{"admin@demo-vendor.test", "Demo Admin", auth.RoleAdmin, "", ""},
	{"dewi@demo-vendor.test", "Dewi Wulandari", auth.RoleSupervisor, "", "Team lead"},
	{"rina@demo-vendor.test", "Rina Pratiwi", auth.RoleAgent, "", "L2"},
	{"adi@demo-vendor.test", "Adi Saputra", auth.RoleAgent, "", "L2"},
	{"yoga@demo-vendor.test", "Yoga Hidayat", auth.RoleAgent, "", "L1"},
	{"hendra@kominfo-jambi.test", "Hendra Gunawan", auth.RoleRequester, RequesterClient, "IT Officer"},
}

// Run upserts the demo organisation, clients and users. It is safe to run repeatedly.
func Run(ctx context.Context, env string, st *store.Store, hasher *auth.Hasher, out io.Writer) error {
	if env != config.EnvDev {
		return ErrNotDev
	}
	hash, err := hasher.Hash(ctx, DevPassword)
	if err != nil {
		return err
	}
	err = st.Tx(ctx, func(tx store.Tx) error {
		footer := orgFooterNote
		if err := tx.Q.UpdateOrganizationProfile(ctx, db.UpdateOrganizationProfileParams{
			Name: OrgName, FooterNote: &footer,
		}); err != nil {
			return err
		}
		clientIDs := make(map[string]uuid.UUID, len(clients))
		for _, name := range clients {
			id, err := tx.Q.UpsertClient(ctx, name)
			if err != nil {
				return fmt.Errorf("client %s: %w", name, err)
			}
			clientIDs[name] = id
		}
		for _, p := range people {
			params := db.UpsertUserByEmailParams{
				Email: p.email, Name: p.name, PasswordHash: &hash, Role: string(p.role),
				Status: string(auth.UserStatusActive),
			}
			if p.client != "" {
				id := clientIDs[p.client]
				params.ClientID = &id
			}
			if p.jobTitle != "" {
				title := p.jobTitle
				params.JobTitle = &title
			}
			if _, err := tx.Q.UpsertUserByEmail(ctx, params); err != nil {
				return fmt.Errorf("user %s: %w", p.email, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Seeded organisation %q with %d clients and %d users.\n\n", OrgName, len(clients), len(people))
	tw := tabwriter.NewWriter(out, tableMinWidth, tableTabWidth, tablePadding, tablePadChar, 0)
	fmt.Fprintln(tw, "EMAIL\tNAME\tROLE")
	for _, p := range people {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", p.email, p.name, p.role)
	}
	_ = tw.Flush()
	fmt.Fprintf(out, "\nPassword for every user: %s\n", DevPassword)
	return nil
}
