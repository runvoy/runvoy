#!/usr/bin/env bash
set -euo pipefail

# Script to update README.md with latest CLI help output
# This ensures the README stays in sync with the CLI commands

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
README="$REPO_ROOT/README.md"
TEMP_HELP=$(mktemp)

# Build the CLI if it doesn't exist or if source files are newer
cd "$REPO_ROOT"
if [ ! -f "./bin/runvoy" ] || [ "$(find cmd/runvoy -name '*.go' -newer ./bin/runvoy 2>/dev/null | wc -l)" -gt 0 ]; then
    echo "Building CLI..."
    just build-cli > /dev/null 2>&1 || go build -o bin/runvoy ./cmd/runvoy > /dev/null 2>&1
fi

# Capture help output
HELP_OUTPUT=$(./bin/runvoy --help 2>&1 || true)

# Create the updated help section
cat > "$TEMP_HELP" << 'HELP_EOF'
### Available Commands

To see all available commands and their descriptions:

```bash
runvoy --help
```

```bash
HELP_EOF
echo "$HELP_OUTPUT" >> "$TEMP_HELP"
cat >> "$TEMP_HELP" << 'HELP_EOF'
```

For more details about a specific command, use:

```bash
runvoy [command] --help
```

For example, to see all user management commands:

```bash
runvoy users --help
```
HELP_EOF

# Update README.md using Python for reliable text replacement
python3 << PYTHON_SCRIPT
import re
import sys

README_PATH = "$README"
HELP_FILE = "$TEMP_HELP"

with open(README_PATH, 'r') as f:
    content = f.read()

with open(HELP_FILE, 'r') as f:
    help_section = f.read()

start_marker = "<!-- CLI_HELP_START -->"
end_marker = "<!-- CLI_HELP_END -->"

if start_marker not in content or end_marker not in content:
    print("Error: Could not find <!-- CLI_HELP_START --> and <!-- CLI_HELP_END --> markers in README.md", file=sys.stderr)
    sys.exit(1)

# Replace content between markers
pattern = re.compile(
    re.escape(start_marker) + r'.*?' + re.escape(end_marker),
    re.DOTALL
)

replacement = start_marker + '\n' + help_section + '\n' + end_marker

new_content = pattern.sub(replacement, content)

with open(README_PATH, 'w') as f:
    f.write(new_content)

print("? Updated README.md with latest CLI help output")
PYTHON_SCRIPT

rm -f "$TEMP_HELP"
