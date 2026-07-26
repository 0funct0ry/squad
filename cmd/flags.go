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
