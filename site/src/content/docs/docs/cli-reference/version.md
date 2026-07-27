---
title: squad version
description: Print version information and exit.
---

```
squad version
```

Prints the squad version and commit SHA baked in at build time via linker
flags, then exits.

```
squad version 1.0.0 (commit: abc1234)
```

`squad --version` (on the root command) prints the same information via a
custom version template.
