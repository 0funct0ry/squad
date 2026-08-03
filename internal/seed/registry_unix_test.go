package seed

import (
	"regexp"
	"testing"
)

func TestUnixGenerators(t *testing.T) {
	cases := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"unix.filePath", regexp.MustCompile(`^/[a-z0-9./]+/[a-z0-9./]+/[a-z]+\.[a-z]+$`)},
		{"unix.permissionString", regexp.MustCompile(`^(0[0-7]{3}|[rwx-]{9})$`)},
		{"unix.groupName", nil},
		{"unix.processName", nil},
		{"unix.environmentVariable", regexp.MustCompile(`^[A-Z]+=.+$`)},
		{"unix.crontabEntry", regexp.MustCompile(`^\d+ (\d+|\*) \* \* \* .+$`)},
		{"unix.logFilePath", regexp.MustCompile(`^/var/log/[a-z-]+\.log$`)},
		{"unix.mountPoint", nil},
		{"unix.deviceName", regexp.MustCompile(`^/dev/.+$`)},
		{"unix.kernelModuleName", nil},
		{"unix.signalName", regexp.MustCompile(`^SIG[A-Z0-9]+$`)},
		{"unix.fileHash", regexp.MustCompile(`^[0-9a-f]{32}$`)},
	}

	if !Exists("unix.filePath") {
		t.Fatal("expected unix.filePath to be registered")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !Exists(tc.name) {
				t.Fatalf("generator %s not registered", tc.name)
			}
			for i := 0; i < 20; i++ {
				v, err := Generate(tc.name, "TEXT", nil)
				if err != nil {
					t.Fatalf("Generate(%s) error: %v", tc.name, err)
				}
				s, ok := v.(string)
				if !ok || s == "" {
					t.Fatalf("Generate(%s) returned empty/non-string value: %#v", tc.name, v)
				}
				if tc.re != nil && !tc.re.MatchString(s) {
					t.Fatalf("Generate(%s) = %q does not match %s", tc.name, s, tc.re.String())
				}
			}
		})
	}
}

func TestUnixPID(t *testing.T) {
	if !Exists("unix.pid") {
		t.Fatal("expected unix.pid to be registered")
	}
	for i := 0; i < 20; i++ {
		v, err := Generate("unix.pid", "INTEGER", nil)
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		n, ok := v.(int)
		if !ok {
			t.Fatalf("expected int, got %#v", v)
		}
		if n < 1 || n > 99999 {
			t.Fatalf("pid out of range: %d", n)
		}
	}
}

func TestUnixGeneratorOptions(t *testing.T) {
	t.Run("permissionString symbolic", func(t *testing.T) {
		v, err := Generate("unix.permissionString", "TEXT", map[string]any{"symbolic": true})
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		s := v.(string)
		if !regexp.MustCompile(`^[rwx-]{9}$`).MatchString(s) {
			t.Fatalf("expected symbolic permission string, got %q", s)
		}
	})

	t.Run("fileHash sha256", func(t *testing.T) {
		v, err := Generate("unix.fileHash", "TEXT", map[string]any{"algo": "sha256"})
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		s := v.(string)
		if len(s) != 64 {
			t.Fatalf("expected 64 hex chars, got %d: %q", len(s), s)
		}
	})
}
