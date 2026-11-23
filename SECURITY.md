# Security

## Supported versions

Only the last release gets security fixes.

## Report a problem

Do not open a public issue for a security problem. Use the private vulnerability report of GitHub: open the Security tab of this repository and click "Report a vulnerability". You get an answer in seven days.

## What the toolchain trusts

`sing` reads the files of the content repository and the record of the Lean build. It does not run code from them. The site build writes only in its output directory, and it opens that directory with `os.Root`.
