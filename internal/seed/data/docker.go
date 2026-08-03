package data

// DockerNamespaces is a curated list of image namespaces ("library" plus a
// handful of company-ish org slugs) used to build image names.
var DockerNamespaces = []string{
	"library", "acme", "globex", "initech", "hooli", "umbrella", "wayne",
}

// DockerImageSlugWords is a curated list of common image-name components.
var DockerImageSlugWords = []string{
	"nginx", "redis", "postgres", "mysql", "mongo", "app", "worker", "api",
	"cache", "proxy", "gateway", "scheduler", "notifier", "billing", "auth",
	"frontend", "backend", "search", "analytics", "queue",
}

// DockerAdjectives mirrors moby's namesgenerator adjective list, used for
// docker.containerName.
var DockerAdjectives = []string{
	"admiring", "adoring", "affectionate", "agitated", "amazing", "angry",
	"awesome", "blissful", "bold", "boring", "brave", "clever", "cool",
	"compassionate", "condescending", "confident", "cranky", "dazzling",
	"determined", "eager", "ecstatic", "elastic", "elegant", "epic",
	"fervent", "festive", "flamboyant", "focused", "friendly", "frosty",
	"gallant", "gifted", "goofy", "gracious", "happy", "hardcore", "heuristic",
	"hopeful", "hungry", "infallible", "inspiring", "jolly", "jovial", "keen",
	"kind", "laughing", "loving", "lucid", "magical", "mystifying",
	"nostalgic", "objective", "optimistic", "peaceful", "pedantic", "practical",
	"priceless", "quirky", "quizzical", "relaxed", "reverent", "romantic",
	"sad", "serene", "sharp", "silly", "sleepy", "stoic", "stupefied",
	"suspicious", "sweet", "tender", "trusting", "unruffled", "upbeat",
	"vibrant", "vigilant", "vigorous", "wizardly", "wonderful", "xenodochial",
	"youthful", "zealous", "zen",
}

// DockerSurnames mirrors moby's namesgenerator surname list (real scientists
// and engineers), used for docker.containerName.
var DockerSurnames = []string{
	"albattani", "allen", "almeida", "archimedes", "ardinghelli", "aryabhata",
	"austin", "babbage", "banach", "banzai", "bardeen", "bassi", "beaver",
	"bell", "benz", "bhabha", "bohr", "booth", "borg", "bose", "boyd",
	"brahmagupta", "brattain", "brown", "carson", "chandrasekhar", "chaplygin",
	"colden", "curie", "darwin", "davinci", "dijkstra", "edison", "einstein",
	"euclid", "euler", "faraday", "feynman", "franklin", "galileo", "gates",
	"goldberg", "goldstine", "goodall", "hawking", "hermann", "hertz",
	"hopper", "hugle", "hypatia", "jang", "jennings", "jepsen", "johnson",
	"joliot", "jones", "kalam", "kare", "keller", "khorana", "kilby",
	"kirch", "knuth", "kowalevski", "lalande", "lamarr", "lamport",
	"leakey", "lewin", "lichterman", "liskov", "lovelace", "lumiere",
	"mahavira", "mayer", "mccarthy", "mcclintock", "mclean", "mendel",
	"mendeleev", "meitner", "meninsky", "mestorf", "morse", "murdock",
	"newton", "nightingale", "nobel", "noether", "northcutt", "noyce",
	"panini", "pare", "pascal", "pasteur", "payne", "perlman", "pike",
	"poincare", "poitras", "ptolemy", "raman", "ramanujan", "ride",
	"ritchie", "rosalind", "rubin", "saha", "sammet", "shannon", "shaw",
	"shirley", "shockley", "sinoussi", "snyder", "spence", "stallman",
	"stonebraker", "swanson", "swartz", "swirles", "tesla", "thompson",
	"torvalds", "turing", "varahamihira", "visvesvaraya", "volhard",
	"wescoff", "wiles", "williams", "wilson", "wing", "wozniak", "wright",
	"yalow", "yonath",
}

// DockerCommonPorts and DockerCommonPortWeights drive weighted container port
// selection (web ports most common).
var DockerCommonPorts = []int{80, 443, 3000, 5432, 6379, 8080}
var DockerCommonPortWeights = []int{25, 15, 25, 12, 8, 15}

// DockerHostPortWeights parallels DockerCommonPorts for host-side port bias
// toward higher, ephemeral-ish ports.
var DockerHostPorts = []int{80, 443, 3000, 8000, 8080, 9000, 32768}
var DockerHostPortWeights = []int{10, 10, 20, 20, 20, 15, 5}

// DockerEnvVarNames is a curated list of common environment variable names.
var DockerEnvVarNames = []string{
	"NODE_ENV", "DEBUG", "DATABASE_URL", "PORT", "API_KEY", "LOG_LEVEL",
	"REDIS_URL", "SECRET_KEY", "HOST", "TZ",
}

// DockerRegistries and DockerRegistryWeights drive weighted registry URL
// selection.
var DockerRegistries = []string{"docker.io", "ghcr.io", "azurecr.io", "ecr.amazonaws.com"}
var DockerRegistryWeights = []int{45, 25, 15, 15}

// DockerRegistryCompanyWords is used to template company-scoped registry
// hosts (e.g. "<company>.azurecr.io").
var DockerRegistryCompanyWords = []string{"acme", "globex", "initech", "hooli", "contoso"}

// DockerRegistryRegions is used to template region-scoped registry hosts
// (e.g. "<region>.ecr.amazonaws.com").
var DockerRegistryRegions = []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}

// DockerComposeServiceWords is a curated list of common docker-compose
// service names.
var DockerComposeServiceWords = []string{"web", "api", "worker", "db", "redis", "nginx", "cache", "queue"}

// DockerLabelKeys is a curated list of common OCI/docker label keys.
var DockerLabelKeys = []string{"maintainer", "version", "environment", "build-date", "vcs-ref"}

// DockerEntrypointBins is a curated list of common container entrypoint
// binaries/scripts.
var DockerEntrypointBins = []string{"node", "python", "/app/start.sh", "java", "./entrypoint.sh"}

// DockerEntrypointArgs is a curated list of common entrypoint arguments,
// paired loosely with DockerEntrypointBins.
var DockerEntrypointArgs = []string{"server.js", "app.py", "-jar app.jar", "start", "--production"}

// DockerContainerStatuses and DockerContainerStatusWeights drive weighted
// container status selection.
var DockerContainerStatuses = []string{"running", "exited (0)", "exited (1)", "created", "restarting", "paused"}
var DockerContainerStatusWeights = []int{45, 20, 10, 10, 8, 7}

// DockerShellCommands is a curated pool of common RUN instruction bodies.
var DockerShellCommands = []string{
	"apt-get update && apt-get install -y curl",
	"npm install",
	"pip install -r requirements.txt",
	"go mod download",
	"yarn install --frozen-lockfile",
	"apk add --no-cache bash",
	"chmod +x /app/entrypoint.sh",
}
