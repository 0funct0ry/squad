# squad marketing + docs site

Standalone Astro project for squad's public-facing website: a marketing
landing page at `/` and a [Starlight](https://starlight.astro.build)-powered
docs site at `/docs/*`. Independent of `web/` (the in-app SPA embedded in
the `squad` binary) — this project is never embedded into the Go binary and
is deployed separately to GitHub Pages.

## Local development

```bash
cd site
npm install
npm run dev
```

This serves the site at `http://localhost:4321/squad/` (note the `/squad`
base path — see "URL shape" below). `npm run build` produces a static build
in `site/dist/`; `npm run preview` serves that build locally.

## Marketing vs. docs split

- Marketing copy lives in `.astro` components under `src/components/`,
  composed together in `src/pages/index.astro`. There's no CMS — copy is
  edited directly in the component files.
- Docs content lives as Markdown under `src/content/docs/`, following
  Starlight's file-based routing convention (a file's path under
  `content/docs/` becomes its URL under `/docs/`).
- Both surfaces share one design-token file, `src/styles/tokens.css`,
  consumed by the Tailwind config (marketing) and passed to Starlight via
  `customCss` (docs) — so colors, spacing, and type stay in sync without
  duplicating them.

## Adding a docs page

1. Add a new `.md` file under `src/content/docs/<group>/<page>.md` with
   `title` and `description` frontmatter.
2. Add an entry for it to the matching group in the `sidebar` array inside
   `astro.config.mjs` (`slug` must match the file's path relative to
   `content/docs/`, without the extension).
3. `npm run build` to confirm it's picked up with no warnings.

Search is powered by Starlight's built-in Pagefind integration — no extra
config needed; it indexes automatically on every build.

## Deploy trigger

`.github/workflows/deploy-site.yml` builds and deploys this project to
GitHub Pages whenever a tag matching `v*` is pushed, or on a manual
`workflow_dispatch` run — deploys are tied to releases, not every commit
to `main` that touches `site/**`. It uses
`actions/configure-pages` + `actions/upload-pages-artifact` (uploading
`site/dist`) + `actions/deploy-pages` — the modern GitHub Pages Actions
flow, not a `gh-pages` branch or a third-party publish action.

## URL shape: project path vs. custom domain

There is no `public/CNAME` file in this project, so `astro.config.mjs` uses
the default GitHub Pages **project-path** form:
`site: 'https://0funct0ry.github.io'`, `base: '/squad'`. Every internal
link goes through Astro's base-aware helpers (`import.meta.env.BASE_URL`,
Starlight's own link handling) rather than hardcoded absolute paths, so the
same build works unchanged if a custom domain is added later — at that
point, add a `public/CNAME` file and change `base` to `'/'`.

## One-time manual step

This workflow cannot enable GitHub Pages itself. Before the first deploy
will actually go live, a repo admin must go to **Settings → Pages** and set
**Source** to **"GitHub Actions"** once. After that, pushing a tag like
`v1.0.0` (e.g. `git tag v1.0.0 && git push origin v1.0.0`) deploys
automatically.
