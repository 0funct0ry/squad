package cli

import (
	"database/sql"
	"fmt"

	"github.com/0funct0ry/squad/internal/restserver"
)

// cliDBProvider implements restserver.DBProvider for the CLI's single
// connection, mirroring internal/server/server.go's singleDBProvider -- but
// reads State.DB/State.Path live (not captured once) so a later ".open" swap
// is picked up by an already-constructed Manager without reconstruction.
type cliDBProvider struct{ state *State }

func (p *cliDBProvider) CurrentDB() (*sql.DB, string, string, error) {
	return p.state.DB, "", p.state.Path, nil
}

// ensureRestManager lazily constructs the CLI's REST config store and
// manager on first ".rest"/".listener" use.
func (s *State) ensureRestManager() {
	if s.RestConfigs == nil {
		s.RestConfigs = restserver.NewConfigStore()
	}
	if s.RestManager == nil {
		s.RestManager = restserver.NewManager(true, s.Write, s.RestBindAddr, s.RestPort, &cliDBProvider{state: s}, s.RestConfigs)
	}
}

// cmdRest configures a table's REST exposure via internal/restserver's
// ConfigStore, scoped to this CLI session's DB. It only edits config -- it
// does not start the listener (matches M7's snapshot-on-Start semantics).
func (s *State) cmdRest(args []string) {
	var mode string
	var table string
	for _, a := range args {
		switch a {
		case "--r", "--rw", "--rwd":
			mode = a
		default:
			table = a
		}
	}
	if table == "" {
		s.shellError(fmt.Errorf("usage: .rest ?--r|--rw|--rwd? TABLE"))
		return
	}

	cfg := restserver.TableConfig{}
	switch mode {
	case "--r", "":
		cfg.Exposed = true
	case "--rw":
		if !s.Write {
			s.shellError(fmt.Errorf(".rest --rw is not allowed in read-only mode (READ_ONLY)"))
			return
		}
		cfg.Exposed, cfg.Create, cfg.Update = true, true, true
	case "--rwd":
		if !s.Write {
			s.shellError(fmt.Errorf(".rest --rwd is not allowed in read-only mode (READ_ONLY)"))
			return
		}
		cfg.Exposed, cfg.Create, cfg.Update, cfg.Delete = true, true, true, true
	}

	s.ensureRestManager()
	s.RestConfigs.Set("", table, cfg)
	fmt.Fprintf(s.Out, "REST config for %s: exposed=%v create=%v update=%v delete=%v (restart .listener to apply)\n",
		table, cfg.Exposed, cfg.Create, cfg.Update, cfg.Delete)
}

// cmdListener starts or stops the REST listener configured via .rest, bound
// to --rest-port/--rest-bind-addr.
func (s *State) cmdListener(args []string) {
	if len(args) != 1 || (args[0] != "start" && args[0] != "stop") {
		s.shellError(fmt.Errorf("usage: .listener start|stop"))
		return
	}
	s.ensureRestManager()
	if args[0] == "start" {
		if err := s.RestManager.Start(); err != nil {
			s.shellError(err)
			return
		}
		fmt.Fprintf(s.Out, "REST listener bound at %s:%d\n", s.RestBindAddr, s.RestPort)
		return
	}
	if err := s.RestManager.Stop("cli stop"); err != nil {
		s.shellError(err)
	}
}

// cmdToken gets or sets a bearer token on State for .rest/.listener to
// consult if a later change adds an HTTP-facing control path for these
// commands; today they drive restserver.Manager in-process with no HTTP
// call of their own to gate, so no enforcement is wired yet.
func (s *State) cmdToken(args []string) {
	if len(args) == 0 {
		if s.Token == "" {
			fmt.Fprintln(s.Out, "(no token set)")
		} else {
			fmt.Fprintln(s.Out, s.Token)
		}
		return
	}
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .token ?VALUE?"))
		return
	}
	s.Token = args[0]
}
