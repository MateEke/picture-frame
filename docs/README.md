# Picture Frame docs

[![Built with Starlight](https://astro.badg.es/v2/built-with-starlight/small.svg)](https://starlight.astro.build)

The documentation site for Picture Frame, built with [Astro](https://astro.build) and
[Starlight](https://starlight.astro.build). It deploys to Cloudflare Pages at
<https://pictureframe.ekemate.hu>. `.github/workflows/docs.yml` builds the site and validates
its links on every change; Cloudflare builds and deploys the site itself.

This project is self-contained: it has its own `package.json` and does not share tooling
with the app in `web/`.

## Commands

Run from the `docs/` directory:

| Command           | Action                                   |
| :---------------- | :--------------------------------------- |
| `npm install`     | Install dependencies                     |
| `npm run dev`     | Start the dev server at `localhost:4321` |
| `npm run build`   | Build the production site to `./dist/`   |
| `npm run preview` | Preview the production build locally     |

## Structure

Pages live in `src/content/docs/` (one route per `.md`/`.mdx` file); the sidebar and site
config are in `astro.config.mjs`; screenshots and other images go in `src/assets/`.
