package cmd

import "github.com/spf13/pflag"

// commonFlags holds the flags shared between the root command and
// `squad sandbox`: bind address/port, browser auto-open, bearer token, and
// log level. Flags specific to one command (--write, --rest,
// --read-only-pragma on root; --dir, --max-upload-size on sandbox) are
// registered separately by each command.
type commonFlags struct {
	Addr     string
	Port     int
	Open     bool
	Token    string
	LogLevel string
}

func registerCommonFlags(fs *pflag.FlagSet, c *commonFlags) {
	fs.StringVarP(&c.Addr, "addr", "a", "127.0.0.1", "Bind address")
	fs.IntVarP(&c.Port, "port", "p", 7071, "Port to listen on")
	fs.BoolVarP(&c.Open, "open", "o", true, "Auto-open default browser on start")
	fs.StringVarP(&c.Token, "token", "t", "", "Optional bearer token gate for the API")
	fs.StringVarP(&c.LogLevel, "log-level", "l", "info", "Log level (debug/info/warn/error)")
}

// restFlags holds the flags shared between the root command and `squad
// sandbox` for the auto-REST capability (SPEC.md §5.7): --rest unlocks it,
// --rest-port/--rest-bind-addr configure the separate listener it uses once
// the user starts it from the REST tab.
type restFlags struct {
	Rest         bool
	RestPort     int
	RestBindAddr string
}

func registerRestFlags(fs *pflag.FlagSet, r *restFlags) {
	fs.BoolVarP(&r.Rest, "rest", "r", false, "Enable auto REST endpoints for tables")
	fs.IntVar(&r.RestPort, "rest-port", 7072, "Port for the separate REST listener (distinct from --port)")
	fs.StringVar(&r.RestBindAddr, "rest-bind-addr", "127.0.0.1", "Bind address for the REST listener")
}
