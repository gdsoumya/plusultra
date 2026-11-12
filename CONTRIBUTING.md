# Contributing

Thanks for helping. Keep changes small, tested and in the shape of the code around them.

## Setup

Go 1.27, Node 22, Docker with Compose, and `make tools` for the proto and database generators.
`make db-up` starts Postgres for the server tests; `make app-install` installs the app.

## Before you open a PR

```sh
make check       # server: buf format and lint, golangci-lint, go vet, go test
make app-check   # app: typecheck, eslint, prettier, jest
```

- Read `CLAUDE.md` first: it lists the make targets, the layout and the rules the code follows.
- Write the failing test first. Tests live beside the code.
- Proto files under `proto/` are the API; run `make generate` after changing them and never edit
  `gen/` or `app/src/gen` by hand.
- Comment only a non-obvious why.
- Use the latest dependency versions and run `govulncheck ./...` after changing Go dependencies.

## Commits and PRs

Conventional Commits, signed off: `git commit -sm "type(scope): summary"`. Keep PR descriptions to
a sentence or two on what and why, with `Fixes #n` when it closes an issue.

## Licence

By contributing you agree that your contribution is licensed under the repository's
[PolyForm Noncommercial 1.0.0](LICENSE) licence.
