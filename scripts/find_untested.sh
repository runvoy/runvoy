#!/bin/bash

# Find all source files (not tests, not mocks)
echo "=== Source Files Without Tests ==="
echo ""

for file in $(find internal cmd -name "*.go" -not -name "*_test.go" -not -name "mock_*.go" -not -path "*/testutil/*" | sort); do
    dir=$(dirname "$file")
    base=$(basename "$file" .go)
    test_file="${dir}/${base}_test.go"

    if [ ! -f "$test_file" ]; then
        echo "$file"
    fi
done
