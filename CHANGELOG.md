# Changelog

All changes that a user can see are in this file. The format follows Keep a Changelog, and the versions follow Semantic Versioning.

## Unreleased

### Added

- Experiment E4 of spec 13: the candidates `cp-omega`, `cp-iota` and `cp-eps-jump`. They compute the Cossart-Piltant invariants of arXiv:1412.0868 at each step of a walk with q = p: ord F = p delta (Def. 2.11), epsilon (Def. 2.25), omega (Def. 2.68 with (2.44)) and kappa, only as 1 or at least 2 (Def. 2.68). `cp-iota` tests ι(x') ≤ ι(x) and `cp-eps-jump` tests the condition of Theorem 3.13(1) on a jump of epsilon. Both count only the steps with omega(x) > 0, the scope of Theorem 3.13. The other steps are in the new field `out_of_scope`.
- A spec with `walks_from` can now pick cases of the E1 spec with `select` (a list of n and q). A spec with experiment E4 gets a summary that compares dimension 3 and dimension 4.
- `sing lab step` prints delta, epsilon, omega and kappa when q = p.

## 0.2.0 - 2026-11-24

### Added

- `sing lab run`, `sing lab list` and `sing lab step`: the computation lab of spec 13. The engine is in the new package `lab`, in Go and not in Rust as spec 13 planned. The specs name the tool `sing-lab`.
- The package `lab` works with f = z^q + F over F_p as in section 2 of Hauser and Perlega (arXiv:1802.05010): cleaning, the residual order along a divisor and point blowups with translations. Unit tests check the residual orders of the examples in section 3 (5 to 7) and section 4 (2 to 6 to 5) of that paper.
- A run writes `record.json`, the outputs and `summary.md` in `lab/runs/<id>/`. The random walks of E1 and the candidate invariants of E2 are reproducible from the seed for each number of threads.
- The site has the page `/lab/` with the runs and a page for each run, with its files. The menu and the footer link to it.

### Changed

- `sing lab` runs. It no longer exits with code 3.

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
