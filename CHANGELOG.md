# Changelog

All changes that a user can see are in this file. The format follows Keep a Changelog, and the versions follow Semantic Versioning.

## Unreleased

### Added

- The package `blueprint` loads the nodes, the routes, the papers and the record of the Lean build of the content repository. It checks the map and computes the status of each node.
- The package `site` builds singularity.tamnd.com: the main page, the pages of `site/`, the map with a page for each node, the papers, the routes, the trust report and the search. It renders the math to MathML with the katex package of Math 2.0, and it writes Markdown copies, JSON in `api/`, `llms.txt` and a sitemap.
- `site.Check` checks the links and the limits of Cloudflare static assets, and `site.Handler` serves a built site.
