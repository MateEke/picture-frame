# Public-cut checklist

The owner, repo name, install URL, and docs URL are already baked to their final values
(`MateEke/picture-frame`, `https://picture-frame-2kf.pages.dev`). They point to resources that go
live at the public cut, so until then the links resolve only for the owner:

- the public **`MateEke/picture-frame`** repository (rename the current private repo to free the
  name, then create the public one with the same name),
- its **releases** (the `install.sh` one-liner resolves once a release exists),
- the **deployed docs** at <https://picture-frame-2kf.pages.dev> (live once this docs PR merges and
  Cloudflare Pages builds).

If the docs ever move to a **custom domain**, update the URL in three places: `docs/astro.config.mjs`
(`SITE_URL`), the repo `README.md`, and `deploy/README.md`.

## Hosting: Cloudflare Pages

The docs deploy to **Cloudflare Pages**, served at the domain root. Because there is no base path,
internal links are plain root-relative (`/manual/dashboard/`) and the build's link validator runs
at its strict default. Keep it that way: do not add an Astro `base`, or every internal link would
need rewriting.

`.github/workflows/docs.yml` only **builds and link-checks** (a CI gate). It does not deploy.
Deployment is via Cloudflare's own build, set up in the dashboard:

- **Workers & Pages → Create application → Pages → Connect to Git**, pick `MateEke/picture-frame`.
- Build settings: framework preset **Astro**, build command **`npm run build`**, output directory
  **`dist`**, and **Root directory (advanced) = `docs`** (the site is in the `docs/` subfolder).
- The production branch deploys on push and other branches get preview URLs.

Trade-off to note: Git integration deploys whatever is on the production branch, so the live docs
track that branch rather than release tags. If release-only docs are wanted later, point the
production branch at a `docs-live` branch advanced on release, or deploy from CI on tags with
`cloudflare/wrangler-action` instead of Git integration.
