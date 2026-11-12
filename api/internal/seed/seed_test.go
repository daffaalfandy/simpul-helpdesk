package seed_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"simdesk/internal/auth"
	"simdesk/internal/config"
	"simdesk/internal/seed"
	"simdesk/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

var cheapParams = auth.Params{Memory: 64, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	h := auth.NewHasher(cheapParams, 1)
	for range 2 {
		if err := seed.Run(ctx, config.EnvDev, st, h, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	var users, clients int
	_ = st.Pool().QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users)
	_ = st.Pool().QueryRow(ctx, `SELECT count(*) FROM clients`).Scan(&clients)
	if users != 6 || clients != 8 {
		t.Fatalf("users=%d clients=%d, want 6 and 8", users, clients)
	}
	org, err := st.Q().GetOrganization(ctx)
	if err != nil || org.Name != seed.OrgName {
		t.Fatalf("org = %+v err %v", org, err)
	}
}

func TestSeedRefusesOutsideDev(t *testing.T) {
	err := seed.Run(context.Background(), config.EnvProd, testdb.New(t), auth.NewHasher(cheapParams, 1), io.Discard)
	if !errors.Is(err, seed.ErrNotDev) {
		t.Fatalf("err = %v, want ErrNotDev", err)
	}
}
