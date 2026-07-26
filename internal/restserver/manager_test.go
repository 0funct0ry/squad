package restserver

import (
	"database/sql"
	"errors"
	"testing"
)

type fakeProvider struct {
	conn  *sql.DB
	scope string
	label string
	err   error
}

func (p *fakeProvider) CurrentDB() (*sql.DB, string, string, error) {
	if p.err != nil {
		return nil, "", "", p.err
	}
	return p.conn, p.scope, p.label, nil
}

func newTestManager(t *testing.T, enabled, write bool, provider DBProvider) (*Manager, *ConfigStore) {
	t.Helper()
	configs := NewConfigStore()
	// Port 0 lets the OS pick a free ephemeral port, avoiding conflicts
	// between parallel test runs.
	m := NewManager(enabled, write, "127.0.0.1", 0, provider, configs)
	t.Cleanup(func() { _ = m.Stop("test cleanup") })
	return m, configs
}

func TestManagerStart_ErrorsWhenDisabled(t *testing.T) {
	conn := newTestConn(t)
	m, _ := newTestManager(t, false, false, &fakeProvider{conn: conn, label: "test.db"})
	if err := m.Start(); err == nil {
		t.Fatal("expected an error when --rest was not passed at launch")
	}
	if m.Status().Running {
		t.Error("expected Running=false after a failed Start")
	}
}

func TestManagerStart_ErrorsWithNoActiveSandboxDB(t *testing.T) {
	m, _ := newTestManager(t, true, true, &fakeProvider{err: errors.New("no active sandbox database selected")})
	if err := m.Start(); err == nil {
		t.Fatal("expected an error when the provider has no active database")
	}
}

func TestManagerStartStopLifecycle(t *testing.T) {
	conn := newTestConn(t)
	m, configs := newTestManager(t, true, false, &fakeProvider{conn: conn, label: "test.db"})
	configs.Set("", "users", TableConfig{Exposed: true})

	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !m.Status().Running {
		t.Fatal("expected Running=true after Start")
	}

	if err := m.Stop("manual"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if m.Status().Running {
		t.Error("expected Running=false after Stop")
	}
	if m.Status().LastStopReason != "manual" {
		t.Errorf("expected lastStopReason=manual, got %q", m.Status().LastStopReason)
	}

	// Stop on an already-stopped manager is a no-op.
	if err := m.Stop("manual"); err != nil {
		t.Errorf("expected Stop on a stopped manager to be a no-op, got: %v", err)
	}
}

func TestManagerStart_SnapshotImmutableAfterConfigChange(t *testing.T) {
	conn := newTestConn(t)
	m, configs := newTestManager(t, true, false, &fakeProvider{conn: conn, label: "test.db"})
	configs.Set("", "users", TableConfig{Exposed: true})

	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	_, _, tables := m.RunningSnapshotTables()
	if _, ok := tables["users"]; !ok {
		t.Fatal("expected users to be in the running snapshot")
	}

	// Changing config after Start must not affect the running snapshot.
	configs.Set("", "users", TableConfig{Exposed: false})
	configs.Set("", "orders", TableConfig{Exposed: true})

	_, _, tablesAfter := m.RunningSnapshotTables()
	if _, ok := tablesAfter["users"]; !ok {
		t.Error("expected users to remain in the running snapshot after a live config change")
	}
	if _, ok := tablesAfter["orders"]; ok {
		t.Error("expected orders (exposed only after Start) to NOT appear in the running snapshot")
	}
}

func TestNotifyActiveDBChanged(t *testing.T) {
	conn := newTestConn(t)
	m, configs := newTestManager(t, true, false, &fakeProvider{conn: conn, label: "db-a"})
	configs.Set("", "users", TableConfig{Exposed: true})

	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if stopped := m.NotifyActiveDBChanged("db-a"); stopped {
		t.Error("expected no stop when the label is unchanged")
	}
	if !m.Status().Running {
		t.Fatal("expected still running")
	}

	if stopped := m.NotifyActiveDBChanged("db-b"); !stopped {
		t.Error("expected a stop when the active database label changed")
	}
	if m.Status().Running {
		t.Error("expected Running=false after an active-database-changed stop")
	}
	if m.Status().LastStopReason != "active database changed" {
		t.Errorf("expected lastStopReason to explain the auto-stop, got %q", m.Status().LastStopReason)
	}
}
