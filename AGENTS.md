# AI Coding Assistant Rules for runvoy

## Testing instructions

- Always use `just` to run common testing/QA/build/deploy commands (if not and we need a new one, let's add it)
- Always run `just check` before **ANY** commit or before considering a change ready (it's also a pre-commit hook)

## Style guidelines

- **Commit messages**: Follow the [Conventional Commits v1.0.0 specification](https://www.conventionalcommits.org/en/v1.0.0/) - see detailed guidelines in [CONTRIBUTING.md](CONTRIBUTING.md#commit-messages)
  - Use format: `<type>[optional scope]: <description>`
  - Required types: `feat` (new feature), `fix` (bug fix)
  - Additional types: `docs`, `test`, `refactor`, `chore`, `style`, `perf`, `ci`, `build`, `tool`
  - Use scopes to provide context: `feat(cli):`, `fix(auth):`, `docs(architecture):`
  - Mark breaking changes with `!` or `BREAKING CHANGE:` in footer
- The project is beta quality, no need to keep backward compatibility in mind, we can break things as we go
- Don't comment code inline unless strictly necessary or to disambiguate code, prefer main function documentation and/or function signature comments
- For each change, verify that [README.md](README.md), [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) are up to date, if not update them
- Don't create big changes, split them into smaller ones, e.g. one sizable chunks per commit
- **README.md is automatically updated** - The README contains sections like between `<!-- CLI_HELP_START -->` and `<!-- CLI_HELP_END -->` markers that are automatically populated with automatically generated output, don't edit it directly
- **docs/CLI.md is automatically updated** - The CLI documentation is automatically updated by running `just generate-cli-docs`, don't edit it directly
