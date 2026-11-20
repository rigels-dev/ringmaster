# Contributing to Ringmaster

Thanks for taking the time. Bug reports, ideas and pull requests are all welcome.

## Reporting a bug

Open an issue with the **Bug report** form. The most useful reports include:

- `ringmaster --version`, your OS, and git / Node.js versions;
- the provider type of the agents involved (OpenAI-compatible, Anthropic API, Claude subscription);
- what you did, what you expected, and what happened;
- the relevant lines from `.ringmaster/logs/run.jsonl` or `events.jsonl`.

Please remove API keys, private code and anything else you would not post publicly before you paste logs.

A security problem (a way for a worker to escape its guardrails, a leak of credentials) should not go in a public issue. Open a [private security advisory](https://github.com/rigels-dev/ringmaster/security/advisories/new) instead.

## Suggesting a feature

Open an issue with the **Feature request** form and describe the problem you want solved before the solution. Small, focused proposals are easier to say yes to.

## Development setup

You need Go 1.27+, Node.js 22+ and git 2.38+, on Linux or macOS.

```bash
git clone https://github.com/rigels-dev/ringmaster.git
cd ringmaster
make build web    # bin/ringmaster and the web UI
make test         # go vet and the Go test suite
make test-web     # the web server's unit tests
```

The code is laid out as:

- `cmd/ringmaster/`: the CLI entry point;
- `internal/`: the scheduler, workers, providers, verification, git and storage;
- `web/`: the web UI (Nuxt);
- `tests/e2e/`: end-to-end tests against a mock provider (`tests/mockserver/`).

If you installed npm packages through a mirror, run `make lockfile` before committing so `web/package-lock.json` points at the public registry; CI rejects it otherwise.

## Pull requests

1. Branch from `main`, named for the change: `feat/…`, `fix/…`, `docs/…`, `test/…`, `refactor/…`, `chore/…`.
2. Keep a pull request to one change, with tests for new behaviour or a fixed bug.
3. Make sure `make test` and `make test-web` pass; CI runs them on Linux and macOS.
4. Write commit messages in the conventional style (`fix: keep workers in their worktree`).
5. Update the docs in `README.md` or `docs/guide/` when behaviour a user can see changes.

By contributing you agree that your contribution is licensed under the [MIT License](LICENSE).
