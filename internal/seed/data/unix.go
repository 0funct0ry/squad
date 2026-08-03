package data

// UnixDirPool is a curated pool of top-level Linux/Unix directory names used
// to build plausible filesystem paths.
var UnixDirPool = []string{"home", "var", "etc", "opt", "usr/local"}

// UnixFilenameWords is a curated pool of common filename stems.
var UnixFilenameWords = []string{
	"config", "backup", "output", "data", "report", "notes", "settings",
	"session", "cache", "index", "archive", "snapshot", "log", "debug",
}

// UnixFileExtensions is a curated pool of common file extensions.
var UnixFileExtensions = []string{"txt", "log", "conf", "sh", "json", "yaml", "csv", "bak"}

// UnixGroupNames and UnixGroupWeights drive weighted Unix group selection.
var UnixGroupNames = []string{"root", "wheel", "staff", "docker", "www-data"}
var UnixGroupWeights = []int{30, 15, 15, 25, 15}

// UnixProcessNames and UnixProcessWeights drive weighted process/daemon name
// selection.
var UnixProcessNames = []string{"nginx", "sshd", "systemd", "cron", "postgres", "node"}
var UnixProcessWeights = []int{20, 20, 20, 15, 15, 10}

// UnixEnvVarNames is a curated list of common shell environment variable
// names.
var UnixEnvVarNames = []string{"PATH", "HOME", "SHELL", "LANG", "USER", "TERM", "PWD", "EDITOR"}

// UnixEnvVarValues is a curated list of plausible values, loosely paired with
// UnixEnvVarNames by index but reused freely across all names.
var UnixEnvVarValues = []string{
	"/usr/local/bin:/usr/bin:/bin", "/home/user", "/bin/bash", "en_US.UTF-8",
	"deploy", "xterm-256color", "/var/www", "vim",
}

// UnixMountPointPrefixes and UnixMountPointWeights drive weighted mount point
// selection; the "/mnt" and "/media" entries are extended with a slug by the
// generator.
var UnixMountPointPrefixes = []string{"/", "/home", "/var", "/mnt", "/media"}
var UnixMountPointWeights = []int{25, 20, 20, 20, 15}

// UnixDeviceNames and UnixDeviceWeights drive weighted device node selection.
var UnixDeviceNames = []string{"/dev/sda1", "/dev/nvme0n1p2", "/dev/null", "/dev/tty1"}
var UnixDeviceWeights = []int{30, 25, 25, 20}

// UnixKernelModules and UnixKernelModuleWeights drive weighted kernel module
// name selection.
var UnixKernelModules = []string{"nf_conntrack", "overlay", "br_netfilter", "ext4"}
var UnixKernelModuleWeights = []int{25, 25, 25, 25}

// UnixSignalNames and UnixSignalWeights drive weighted POSIX signal name
// selection.
var UnixSignalNames = []string{"SIGTERM", "SIGKILL", "SIGINT", "SIGHUP", "SIGUSR1"}
var UnixSignalWeights = []int{35, 20, 20, 15, 10}

// UnixCronCommands is a curated pool of shell-ish commands used as the
// command portion of a crontab entry.
var UnixCronCommands = []string{
	"/usr/bin/backup.sh", "certbot renew", "logrotate /etc/logrotate.conf",
	"/usr/local/bin/cleanup.sh", "rsync -a /data /backup",
	"/usr/bin/find /tmp -mtime +7 -delete",
}
