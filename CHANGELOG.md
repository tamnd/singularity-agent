# Changelog

All changes that a user can see are in this file. The format follows Keep a Changelog, and the versions follow Semantic Versioning.

## Unreleased

## 0.1.0 - 2026-11-23

This release closes milestone S0 of tamnd/singularity. The code is the same as in 0.0.1. The site that `sing site build` makes from the content of S0 is live at https://singularity.tamnd.com.

## 0.0.1 - 2026-11-23

### Added

- The package `blueprint` loads the nodes, the routes, the papers and the record of the Lean build of the content repository. It checks the map and computes the status of each node.
- The package `site` builds singularity.tamnd.com: the main page, the pages of `site/`, the map with a page for each node, the papers, the routes, the trust report and the search. It renders the math to MathML with the katex package of Math 2.0, and it writes Markdown copies, JSON in `api/`, `llms.txt` and a sitemap.
- `site.Check` checks the links and the limits of Cloudflare static assets, and `site.Handler` serves a built site.
- The command `sing`, with `version`, `check`, `status`, `index` and `site build`, `site check` and `site serve`. The commands `trust`, `lab` and `draft` exit with code 3 and name the milestone that adds them.

### Changed

- A Lean name of a node that is not in `records/lean.json` is no longer a fault of `sing check`. The node gets the status `planned` or `stated`, and the site shows the reason. This lets a node name the declaration that it will have before the declaration exists.
- Paper records can have the uses `reuse` and `bridge`.
