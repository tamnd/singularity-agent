# Contributing

Thank you for your help. This file gives the rules for a change to the toolchain.

## Before you start

Open an issue or find one in [tamnd/singularity](https://github.com/tamnd/singularity/issues). Each pull request refers to a milestone issue there.

## The rules for code

1. Run `make check` before you push. It runs gofmt, go vet, the tests with the race detector and golangci-lint.
2. Add a test for each change in behaviour.
3. Keep the exit codes. A command that is not written yet exits with code 3 and names its milestone.
4. Do not add a dependency that is not necessary. A new dependency needs a reason in the pull request.
5. The toolchain never marks a node as proved. Only the status rules do that, from the record of the Lean build.

## The rules for text

Write the text in plain technical English, in the style of ASD-STE100. Use short sentences and the active voice. Use one sentence for one instruction. Do not use dashes as punctuation. Do not break a paragraph across lines in Markdown.

## Labels

Each pull request has one `area/*` label and one `type/*` label, and the milestone of its issue.
