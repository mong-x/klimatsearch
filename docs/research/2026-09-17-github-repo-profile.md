# Modern GitHub repository profile (2026)

What a public Go/MCP repo needs so the GitHub page reads as a product, not a stub. Applied to klimatsearch.

## Sources

- [GitHub Pages](https://docs.github.com/en/pages) — project site at `https://<owner>.github.io/<repo>/`; source **GitHub Actions** (not branch `/docs`) when the marketing site must not collide with `docs/` ADRs.
- [Shields.io](https://shields.io) — CI, license, language, custom stack badges. `for-the-badge` + a shared `labelColor` is the look RepoSkein uses.
- [capsule-render](https://github.com/kyechan99/capsule-render) — README header/footer banners via a parameterized SVG URL.
- [GitHub social preview](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/customizing-your-repositorys-social-media-preview) — **1280×640**, PNG/JPG, under 1 MB. Shown on `github.com` repo cards and link unfurls.
- RepoSkein README (`reposkein/reposkein`): centered banner, for-the-badge row, live Pages demo above the fold, TOC, then task-oriented docs table.
- React Bits Pro catalog (read-only): `hero-1` two-column CTA, `hero-11` wireframe corners, `hero-18` command-palette hero, `features-11` indexed editorial cards, `features-4` tabbed feature lists. Used as **layout cues only** — klimatsearch does not depend on React.

## Profile checklist

| Surface | What to ship |
| --- | --- |
| README header | Banner + ≤8 badges (CI, e2e, Go, license, MCP, Apache). Group status vs stack. |
| One-liner | What it is + who it is for, then a live demo link. |
| Screenshot / OG | Real product surface (terminal + JSON), not a fake dashboard. |
| Quick start | One copy-paste command that actually runs. |
| Docs table | Point at ADRs / LAUNCH instead of duplicating them. |
| GitHub Pages | Static site in `site/`, deployed by Actions. `og:image` absolute on the Pages origin. |
| Social preview | Upload `site/assets/social-preview.jpg` (1280×640) in Settings → General. |
| About box | Description, homepage = Pages URL, topics. |
| Dark/light | Badge `logoColor` that survives GitHub dark theme (`E6E8EB` on `0A0B0D`). |

## What we did not copy

- Looping typing SVGs, snake contribution graphs, aurora heroes (`hero-3`/`hero-5`/`hero-7`). Those fight a developer-tool page.
- React Bits as a runtime. The Pages site is static HTML/CSS so the Go module stays CGO + stdlib `ServeMux`.
