#!/bin/bash

echo "=== Coverage Analysis by Package ==="
echo ""

# Group untested files by package
declare -A packages

for file in $(find internal cmd -name "*.go" -not -name "*_test.go" -not -name "mock_*.go" -not -path "*/testutil/*" | sort); do
    dir=$(dirname "$file")
    base=$(basename "$file" .go)
    test_file="${dir}/${base}_test.go"

    if [ ! -f "$test_file" ]; then
        packages["$dir"]=$((${packages["$dir"]:-0} + 1))
    fi
done

# Sort and display
for pkg in "${!packages[@]}"; do
    echo "${packages[$pkg]} untested files in: $pkg"
done | sort -rn

echo ""
echo "=== Package Statistics ==="
for dir in $(find internal cmd -type d -not -path "*/testutil" | sort); do
    total=$(find "$dir" -maxdepth 1 -name "*.go" -not -name "*_test.go" -not -name "mock_*.go" 2>/dev/null | wc -l)
    tests=$(find "$dir" -maxdepth 1 -name "*_test.go" 2>/dev/null | wc -l)

    if [ "$total" -gt 0 ]; then
        coverage=$((tests * 100 / total))
        if [ "$coverage" -lt 100 ] && [ "$total" -ge 2 ]; then
            printf "%3d%% coverage (%d/%d files) - %s\n" "$coverage" "$tests" "$total" "$dir"
        fi
    fi
done | sort -n | head -30
