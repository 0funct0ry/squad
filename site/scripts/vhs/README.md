# squad cli terminal demos

GIF recordings of real `squad cli` sessions, made with
[VHS](https://github.com/charmbracelet/vhs) and embedded on the marketing
page and the `squad cli` docs page. Every recording is a real terminal
session against a copy of `examples/ecommerce.db` — nothing here is staged
or hand-edited frame-by-frame.

| Tape | Output | Shows |
|---|---|---|
| `cli-basics.tape` | `../../public/casts/cli-basics.gif` | `.tables`, `.schema`, a `SELECT`, and switching `.mode` to `markdown` |
| `cli-dotcommands.tape` | `../../public/casts/cli-dotcommands.gif` | `.timer`, `.explain`, `.grep`, `.constraints` |
| `cli-templates.tape` | `../../public/casts/cli-templates.gif` | `.echo` with `{{ }}` template functions, `.seed`, `.repeat` |

## Regenerating

Requires `vhs`, `ffmpeg`, and a Chrome/Chromium install:

```bash
brew install vhs ffmpeg
```

From the repo root:

```bash
make build
cp examples/ecommerce.db /tmp/cli-demo-basics.db
cp examples/ecommerce.db /tmp/cli-demo-dot.db
cp examples/ecommerce.db /tmp/cli-demo-tpl.db

export VHS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
vhs site/scripts/vhs/cli-basics.tape
vhs site/scripts/vhs/cli-dotcommands.tape
vhs site/scripts/vhs/cli-templates.tape
```

Each tape starts a fresh `squad cli` session against its own `/tmp` copy of
the fixture DB — re-copy the fixture before re-recording `cli-dotcommands`
or `cli-templates`, since those run in `--write` mode and mutate the file
(`.seed`/`.repeat` insert rows).

Adjust `VHS_CHROME_PATH` to wherever Chrome/Chromium lives on your machine
if it's not at the default macOS path above.
