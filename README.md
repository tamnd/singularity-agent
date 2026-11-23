# singularity-agent

The toolchain of [singularity](https://github.com/tamnd/singularity), the public project on resolution of singularities in characteristic p. The command is `sing`. It checks the map of the content repository, gives the status of each node from the record of the Lean build, and builds the site [singularity.tamnd.com](https://singularity.tamnd.com).

The open problem is resolution of singularities for varieties of dimension 4 and more over a field of characteristic p > 0. No proof of it exists. This toolchain does not decide that a result is proved. The Lean kernel decides that, and `sing` only reads the record of the build.

## Install

You need Go 1.27 or later.

```sh
go install github.com/tamnd/singularity-agent/cmd/sing@latest
```

Each release on GitHub also has binaries for Linux, macOS and Windows, with checksums and a build provenance attestation.

## Use

Run the commands in a clone of `tamnd/singularity`.

```sh
sing check                      # check the map: tags, links, papers, routes and Lean names
sing status                     # print the status of each node
sing status -only stated        # print only the stated nodes
sing index -lean lean.json      # copy the record of the Lean build to records/lean.json
sing site build -content . -out public
sing site serve -dir public     # serve the site at http://127.0.0.1:8080/
```

The exit codes are 0 for success, 1 for a failed check, 2 for a usage error and 3 for a command that is not written yet. The help text gives the milestone of each command that is not written yet.

## The status rules

The status of a node comes from `records/lean.json` and from the nodes that it uses. The rules are in `blueprint/blueprint.go`.

| Status | When |
|---|---|
| planned | The node has no Lean statement in the build. |
| stated | The statement builds, but the proof uses `sorry` or an axiom that is not standard. |
| defined | A definition that builds with no `sorry`. |
| proved | The proof uses only `propext`, `Classical.choice` and `Quot.sound`, and each node that it uses is proved or defined. |
| proved-mod | The proof is clean but it takes named hypotheses, or a node that it uses is not proved. |
| open | A conjecture. No proof is claimed. |
| refuted | The negation is proved. |

## The site

The site uses the style and the math renderer of [Math 2.0](https://github.com/tamnd/math2-agent). The math is rendered to MathML at build time, so a page shows its math with JavaScript off. Each page has a Markdown copy next to it, and `/api/` has the map, the papers and the trust report as JSON.

## Develop

```sh
make check      # gofmt, vet, the tests with the race detector, and golangci-lint
```

Read [CONTRIBUTING.md](CONTRIBUTING.md) before you open a pull request.

## Licence

MIT. See [LICENSE](LICENSE). The files in `site/assets/` come from Math 2.0. See [site/assets/UPSTREAM.md](site/assets/UPSTREAM.md) for their sources and licences.
