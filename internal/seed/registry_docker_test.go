package seed

import (
	"regexp"
	"strings"
	"testing"
)

func TestDockerGenerators(t *testing.T) {
	cases := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"docker.imageName", regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+(-[a-z0-9]+)?$`)},
		{"docker.imageTag", nil},
		{"docker.imageRef", regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+(-[a-z0-9]+)?:.+$`)},
		{"docker.containerName", regexp.MustCompile(`^[a-z]+_[a-z]+$`)},
		{"docker.containerID", regexp.MustCompile(`^[0-9a-f]{12}$`)},
		{"docker.dockerfileInstruction", nil},
		{"docker.volumeName", regexp.MustCompile(`(-data|_vol)$`)},
		{"docker.networkName", regexp.MustCompile(`(-net|_default)$`)},
		{"docker.portMapping", regexp.MustCompile(`^\d+:\d+$`)},
		{"docker.envVar", regexp.MustCompile(`^[A-Z_]+=.+$`)},
		{"docker.registryURL", nil},
		{"docker.imageDigest", regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)},
		{"docker.buildContextPath", nil},
		{"docker.composeServiceName", nil},
		{"docker.labelPair", regexp.MustCompile(`^com\.example\..+=.+$`)},
		{"docker.healthCheckCommand", regexp.MustCompile(`^CMD curl -f http://localhost:\d+/health \|\| exit 1$`)},
		{"docker.entrypointCommand", regexp.MustCompile(`^\[".+", ".+"\]$`)},
		{"docker.layerSize", regexp.MustCompile(`^\d+(\.\d+)?MB$`)},
		{"docker.containerStatus", nil},
		{"docker.hubRepoDescription", nil},
	}

	if !Exists("docker.imageName") {
		t.Fatal("expected docker.imageName to be registered")
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

func TestDockerGeneratorOptions(t *testing.T) {
	t.Run("containerID long", func(t *testing.T) {
		v, err := Generate("docker.containerID", "TEXT", map[string]any{"long": true})
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		s := v.(string)
		if len(s) != 64 {
			t.Fatalf("expected 64 hex chars, got %d: %q", len(s), s)
		}
	})

	t.Run("layerSize bytes", func(t *testing.T) {
		v, err := Generate("docker.layerSize", "TEXT", map[string]any{"unit": "bytes"})
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		s := v.(string)
		if strings.Contains(s, "MB") {
			t.Fatalf("expected raw byte count, got %q", s)
		}
	})

	t.Run("healthCheckCommand custom port", func(t *testing.T) {
		v, err := Generate("docker.healthCheckCommand", "TEXT", map[string]any{"port": 9090})
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		s := v.(string)
		if !strings.Contains(s, ":9090/health") {
			t.Fatalf("expected port 9090 in output, got %q", s)
		}
	})
}
