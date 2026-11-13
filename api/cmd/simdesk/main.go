// Command simdesk is the SimDesk (Simpul Helpdesk) server and admin CLI.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // business hours need IANA zones even on hosts without zoneinfo

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"golang.org/x/term"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/catalog"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/config"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/mail"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/seed"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/storage"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/webui"
)

const (
	cmdServe      = "serve"
	cmdWorker     = "worker"
	cmdMigrate    = "migrate"
	cmdSeed       = "seed"
	cmdUser       = "user"
	cmdHolidays   = "holidays"
	cmdHelp       = "help"
	subUserCreate = "create"
	subPresets    = "presets"
	subLoadPreset = "load-preset"
	migrateUp     = "up"
	migrateDown   = "down"
	migrateStatus = "status"
)

const (
	exitError = 1
	exitUsage = 2
)

const (
	readHeaderTimeout   = 10 * time.Second
	readTimeout         = 30 * time.Second
	writeTimeout        = 60 * time.Second
	idleTimeout         = 120 * time.Second
	httpShutdownTimeout = 15 * time.Second
	jobShutdownTimeout  = 30 * time.Second
	jobMaxWorkers       = 10
)

const usage = `SimDesk (Simpul Helpdesk)

Usage:
  simdesk serve [--worker]   run the web server (--worker also runs background jobs)
  simdesk worker             run background jobs only
  simdesk migrate [up|down|status]
  simdesk seed               load demo data (APP_ENV=dev only)
  simdesk user create --email E --name N [--role admin] [--client NAME] [--job-title T] [--password-stdin]
  simdesk holidays presets   list the bundled official holiday lists
  simdesk holidays load-preset --year YYYY
                             add a bundled year's missing holidays

Configuration is read from environment variables (see .env.example).
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(exitUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case cmdServe:
		err = runServe(ctx, args)
	case cmdWorker:
		err = runWorker(ctx)
	case cmdMigrate:
		err = runMigrate(ctx, args)
	case cmdSeed:
		err = runSeed(ctx)
	case cmdUser:
		err = runUser(ctx, args)
	case cmdHolidays:
		err = runHolidays(ctx, args)
	case cmdHelp, "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(exitUsage)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", errorMessage(err))
		os.Exit(exitError)
	}
}

func errorMessage(err error) string {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		return err.Error()
	}
	if ae.Detail != "" {
		return ae.Title + ": " + ae.Detail
	}
	return ae.Title
}

// app holds the shared wiring for every command.
type app struct {
	cfg     config.Config
	log     *slog.Logger
	store   *store.Store
	hasher  *auth.Hasher
	jobs    *river.Client[pgx.Tx]
	auth    *auth.Service
	clients *clients.Service
	sla     *sla.Service
	catalog *catalog.Service
	tickets *tickets.Service
	files   storage.Store
}

func (a *app) close() {
	_ = a.files.Close()
	a.store.Close()
}

func newApp(ctx context.Context, runJobs bool) (*app, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	log := newLogger(cfg)
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	mailer, err := mail.NewSMTP(mail.SMTPConfig{
		Addr: cfg.SMTPAddr, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.MailFrom, FromName: cfg.MailFromName,
	})
	if err != nil {
		st.Close()
		return nil, err
	}
	files, err := storage.NewFS(cfg.StorageDir)
	if err != nil {
		st.Close()
		return nil, err
	}
	opts := auth.DefaultOptions()
	workers := river.NewWorkers()
	river.AddWorker(workers, auth.NewResetEmailWorker(st, mailer, cfg.AppURL, opts.ResetTokenTTL))
	river.AddWorker(workers, tickets.NewAttachmentSweeper(st, files))
	riverCfg := &river.Config{Workers: workers, Logger: log.With("component", "jobs")}
	if runJobs {
		// Periodic jobs only on clients that run jobs: River refuses them on insert-only clients.
		riverCfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: jobMaxWorkers}}
		riverCfg.PeriodicJobs = []*river.PeriodicJob{tickets.AttachmentSweepJob()}
	}
	jobs, err := river.NewClient(riverpgxv5.New(st.Pool()), riverCfg)
	if err != nil {
		_ = files.Close()
		st.Close()
		return nil, fmt.Errorf("job client: %w", err)
	}
	hasher := auth.NewHasher(auth.DefaultParams, auth.DefaultMaxConcurrentHashes)
	slaSvc := sla.NewService(st)
	return &app{
		cfg: cfg, log: log, store: st, hasher: hasher, jobs: jobs, files: files,
		auth:    auth.NewService(st, hasher, jobs, opts, log),
		clients: clients.NewService(st),
		sla:     slaSvc,
		catalog: catalog.NewService(st),
		tickets: tickets.NewService(st, files, slaSvc),
	}, nil
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if cfg.IsDev() {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet(cmdServe, flag.ExitOnError)
	withWorker := fs.Bool(cmdWorker, false, "also run background jobs in this process")
	_ = fs.Parse(args)

	a, err := newApp(ctx, *withWorker)
	if err != nil {
		return err
	}
	defer a.close()

	handler, err := httpapi.New(httpapi.Deps{
		Config: a.cfg, Store: a.store, Auth: a.auth, Clients: a.clients, SLA: a.sla, Catalog: a.catalog,
		Tickets: a.tickets, Logger: a.log, WebUI: webui.FS(),
	})
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              a.cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	if *withWorker {
		if err := a.jobs.Start(ctx); err != nil {
			return fmt.Errorf("start jobs: %w", err)
		}
	}
	errc := make(chan error, 1)
	go func() {
		a.log.Info("listening", "addr", a.cfg.HTTPAddr, "env", a.cfg.Env, "worker", *withWorker, "docs", a.cfg.DocsEnabled)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	a.log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		a.log.Error("http shutdown", "err", err)
	}
	if *withWorker {
		if err := a.jobs.Stop(shutdownCtx); err != nil {
			a.log.Error("jobs shutdown", "err", err)
		}
	}
	return nil
}

func runWorker(ctx context.Context) error {
	a, err := newApp(ctx, true)
	if err != nil {
		return err
	}
	defer a.close()
	if err := a.jobs.Start(ctx); err != nil {
		return fmt.Errorf("start jobs: %w", err)
	}
	a.log.Info("worker running")
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), jobShutdownTimeout)
	defer cancel()
	return a.jobs.Stop(shutdownCtx)
}

func runMigrate(ctx context.Context, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	direction := migrateUp
	if len(args) > 0 {
		direction = args[0]
	}
	switch direction {
	case migrateUp:
		return store.MigrateUp(ctx, st.Pool(), os.Stdout)
	case migrateDown:
		return store.MigrateDown(ctx, st.Pool(), os.Stdout)
	case migrateStatus:
		return store.MigrateStatus(ctx, st.Pool(), os.Stdout)
	default:
		return fmt.Errorf("unknown migrate direction %q (use %s, %s or %s)", direction, migrateUp, migrateDown, migrateStatus)
	}
}

func runSeed(ctx context.Context) error {
	a, err := newApp(ctx, false)
	if err != nil {
		return err
	}
	defer a.close()
	return seed.Run(ctx, a.cfg.Env, a.store, a.hasher, os.Stdout)
}

const userCreateUsage = "usage: simdesk user create --email E --name N [--role admin] [--client NAME] [--job-title T] [--password-stdin]"

func runUser(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != subUserCreate {
		return errors.New(userCreateUsage)
	}
	fs := flag.NewFlagSet(cmdUser+" "+subUserCreate, flag.ExitOnError)
	email := fs.String("email", "", "email address (required)")
	name := fs.String("name", "", "full name (required)")
	role := fs.String("role", string(auth.RoleAdmin), "admin, supervisor, agent or requester")
	client := fs.String("client", "", "client name (requesters only)")
	jobTitle := fs.String("job-title", "", "job title, e.g. L2")
	fromStdin := fs.Bool("password-stdin", false, "read the password from standard input")
	_ = fs.Parse(args[1:])
	if *email == "" || *name == "" {
		return errors.New(userCreateUsage)
	}

	password, err := readPassword(*fromStdin)
	if err != nil {
		return err
	}
	a, err := newApp(ctx, false)
	if err != nil {
		return err
	}
	defer a.close()
	id, err := a.auth.CreateUser(ctx, auth.NewUser{
		Email: *email, Name: *name, Password: password, Role: auth.Role(*role),
		ClientName: *client, JobTitle: *jobTitle,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Created %s %s (%s)\n", *role, auth.NormalizeEmail(*email), id)
	return nil
}

const holidaysUsage = "usage: simdesk holidays presets | simdesk holidays load-preset --year YYYY"

func runHolidays(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(holidaysUsage)
	}
	a, err := newApp(ctx, false)
	if err != nil {
		return err
	}
	defer a.close()
	switch args[0] {
	case subPresets:
		presets, err := a.sla.HolidayPresets(ctx)
		if err != nil {
			return err
		}
		for _, p := range presets {
			fmt.Printf("%d  %s  %d holidays, %d missing\n", p.Year, p.Country, p.HolidayCount, p.MissingCount)
		}
		return nil
	case subLoadPreset:
		fs := flag.NewFlagSet(cmdHolidays+" "+subLoadPreset, flag.ExitOnError)
		year := fs.Int("year", 0, "year to load (required)")
		_ = fs.Parse(args[1:])
		if *year == 0 {
			return errors.New(holidaysUsage)
		}
		r, err := a.sla.LoadHolidayPreset(ctx, audit.SystemActor(audit.ActorNameCLI), *year)
		if err != nil {
			return err
		}
		fmt.Printf("Added %d (%d already present)\n", r.Added, r.Skipped)
		return nil
	default:
		return errors.New(holidaysUsage)
	}
}

func readPassword(fromStdin bool) (string, error) {
	fd := int(os.Stdin.Fd())
	if fromStdin || !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Repeat password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords don't match")
	}
	return string(first), nil
}
