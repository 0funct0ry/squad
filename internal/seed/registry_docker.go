package seed

import (
	"fmt"

	"github.com/0funct0ry/squad/internal/seed/data"
	"github.com/brianvoe/gofakeit/v7"
)

// genDockerImageName builds a "<namespace>/<image-slug>" image name.
func genDockerImageName() string {
	namespace := pickFrom(data.DockerNamespaces)
	slug := kebabSlug(data.DockerImageSlugWords, 1, 2)
	return namespace + "/" + slug
}

// genDockerImageTag builds a weighted image tag: latest, semver, alpine,
// slim, or a short git-sha.
func genDockerImageTag() string {
	switch weightedPick([]string{"semver", "latest", "alpine", "slim", "sha"}, []int{35, 25, 15, 10, 15}) {
	case "latest":
		return "latest"
	case "alpine":
		return "alpine"
	case "slim":
		return "slim"
	case "sha":
		return hexString(7)
	default:
		return fmt.Sprintf("%d.%d.%d", gofakeit.Number(0, 3), gofakeit.Number(0, 20), gofakeit.Number(0, 30))
	}
}

// genDockerContainerID returns a 12 or 64 lowercase hex char container ID.
func genDockerContainerID(long bool) string {
	if long {
		return hexString(64)
	}
	return hexString(12)
}

// genDockerfileInstruction builds one random Dockerfile instruction line.
func genDockerfileInstruction() string {
	switch gofakeit.Number(0, 5) {
	case 0:
		return "FROM " + genDockerImageName() + ":" + genDockerImageTag()
	case 1:
		return "RUN " + pickFrom(data.DockerShellCommands)
	case 2:
		return "COPY . /app"
	case 3:
		port := weightedPick(intsToStrings(data.DockerCommonPorts), data.DockerCommonPortWeights)
		return "EXPOSE " + port
	case 4:
		key := pickFrom(data.DockerEnvVarNames)
		return fmt.Sprintf("ENV %s=%s", key, kebabSlug(data.DockerImageSlugWords, 1, 1))
	default:
		bin := pickFrom(data.DockerEntrypointBins)
		arg := pickFrom(data.DockerEntrypointArgs)
		return fmt.Sprintf(`CMD ["%s", "%s"]`, bin, arg)
	}
}

// genDockerLayerSize returns a human size string ("42.3MB") or a raw byte
// count string, depending on unit.
func genDockerLayerSize(unit string) string {
	mb := gofakeit.Float64Range(0.1, 500)
	if unit == "bytes" {
		return fmt.Sprintf("%d", int64(mb*1024*1024))
	}
	return fmt.Sprintf("%.1fMB", mb)
}

// genDockerHealthCheckCommand builds a "CMD curl -f ..." healthcheck line
// for the given port.
func genDockerHealthCheckCommand(port int) string {
	return fmt.Sprintf("CMD curl -f http://localhost:%d/health || exit 1", port)
}

// genDockerRegistryURL builds a weighted registry host.
func genDockerRegistryURL() string {
	switch weightedPick(data.DockerRegistries, data.DockerRegistryWeights) {
	case "azurecr.io":
		return pickFrom(data.DockerRegistryCompanyWords) + ".azurecr.io"
	case "ecr.amazonaws.com":
		return pickFrom(data.DockerRegistryRegions) + ".ecr.amazonaws.com"
	case "ghcr.io":
		return "ghcr.io"
	default:
		return "docker.io"
	}
}

func intsToStrings(vals []int) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

func dockerGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "docker.imageName", Group: "docker", Description: "Docker image name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genDockerImageName(), nil
		}},

		{Name: "docker.imageTag", Group: "docker", Description: "Docker image tag", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genDockerImageTag(), nil
		}},

		{Name: "docker.imageRef", Group: "docker", Description: "Docker image reference (name:tag)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genDockerImageName() + ":" + genDockerImageTag(), nil
		}},

		{Name: "docker.containerName", Group: "docker", Description: "Docker default-style container name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return pickFrom(data.DockerAdjectives) + "_" + pickFrom(data.DockerSurnames), nil
		}},

		{Name: "docker.containerID", Group: "docker", Description: "Docker container ID (hex)", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "long", Label: "Long (64 chars)", Kind: OptKindBool, Default: false},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			return genDockerContainerID(optBool(opts, "long", false)), nil
		}},

		{Name: "docker.dockerfileInstruction", Group: "docker", Description: "Random Dockerfile instruction line", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genDockerfileInstruction(), nil
		}},

		{Name: "docker.volumeName", Group: "docker", Description: "Docker volume name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			slug := kebabSlug(data.DockerImageSlugWords, 1, 1)
			if gofakeit.Number(0, 1) == 0 {
				return slug + "-data", nil
			}
			return slug + "_vol", nil
		}},

		{Name: "docker.networkName", Group: "docker", Description: "Docker network name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			slug := kebabSlug(data.DockerImageSlugWords, 1, 1)
			if gofakeit.Number(0, 1) == 0 {
				return slug + "-net", nil
			}
			return slug + "_default", nil
		}},

		{Name: "docker.portMapping", Group: "docker", Description: "Host:container port mapping", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			host := weightedPick(intsToStrings(data.DockerHostPorts), data.DockerHostPortWeights)
			container := weightedPick(intsToStrings(data.DockerCommonPorts), data.DockerCommonPortWeights)
			return host + ":" + container, nil
		}},

		{Name: "docker.envVar", Group: "docker", Description: "KEY=value environment variable", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return pickFrom(data.DockerEnvVarNames) + "=" + kebabSlug(data.DockerImageSlugWords, 1, 1), nil
		}},

		{Name: "docker.registryURL", Group: "docker", Description: "Container registry URL", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genDockerRegistryURL(), nil
		}},

		{Name: "docker.imageDigest", Group: "docker", Description: "Docker image digest (sha256)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "sha256:" + hexString(64), nil
		}},

		{Name: "docker.buildContextPath", Group: "docker", Description: "Docker build context path", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			slug := kebabSlug(data.DockerImageSlugWords, 1, 1)
			switch gofakeit.Number(0, 2) {
			case 0:
				return ".", nil
			case 1:
				return "./docker/" + slug, nil
			default:
				return "./services/" + slug, nil
			}
		}},

		{Name: "docker.composeServiceName", Group: "docker", Description: "docker-compose service name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return pickFrom(data.DockerComposeServiceWords), nil
		}},

		{Name: "docker.labelPair", Group: "docker", Description: "Container label key=value pair", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "com.example." + pickFrom(data.DockerLabelKeys) + "=" + kebabSlug(data.DockerImageSlugWords, 1, 1), nil
		}},

		{Name: "docker.healthCheckCommand", Group: "docker", Description: "Container healthcheck command", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "port", Label: "Port", Kind: OptKindInt, Default: 3000},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			return genDockerHealthCheckCommand(optInt(opts, "port", 3000)), nil
		}},

		{Name: "docker.entrypointCommand", Group: "docker", Description: "Container entrypoint command", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return fmt.Sprintf(`["%s", "%s"]`, pickFrom(data.DockerEntrypointBins), pickFrom(data.DockerEntrypointArgs)), nil
		}},

		{Name: "docker.layerSize", Group: "docker", Description: "Docker image layer size", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "unit", Label: "Unit", Kind: OptKindSelect, Choices: []string{"MB", "bytes"}, Default: "MB"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			return genDockerLayerSize(optString(opts, "unit", "MB")), nil
		}},

		{Name: "docker.containerStatus", Group: "docker", Description: "Docker container status", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.DockerContainerStatuses, data.DockerContainerStatusWeights), nil
		}},

		{Name: "docker.hubRepoDescription", Group: "docker", Description: "Docker Hub repository description", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return sentenceWithWordCount(10), nil
		}},
	}
}
