# AI Coding Assistant Rules for runvoy

## Style guidelines

- The project is in pre-alpha, no need to keep backward compatibility in mind, we can break things as we go
- Don't comment code inline unless strictly necessary or to disambiguate code, prefer main function documentation and/or function signature comments
- For each change, verify that README.md and docs/ARCHITECTURE.md are up to date, if not update them
- Don't create big changes, split them into smaller ones, e.g. one sizable chunks per commit
- **README.md is automatically updated** - The README contains a section between `<!-- CLI_HELP_START -->` and `<!-- CLI_HELP_END -->` markers that is automatically populated with the latest CLI help output, don't edit it directly
- **docs/CLI.md is automatically updated** - The CLI documentation is automatically updated by running `just generate-cli-docs`, don't edit it directly

## Testing instructions

- Always use `just` to run common testing/QA/build/deploy commands (if not and we need a new one, let's add it)
- Always run `just check` before **ANY** commit or before considering a change ready (it's also a pre-commit hook)
