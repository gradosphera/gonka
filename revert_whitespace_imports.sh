#!/bin/bash

force_revert=false
if [ "$1" == "-f" ]; then
    force_revert=true
fi

set -e

changed_files=$(git diff --name-only)

for file in $changed_files; do
    if [[ "$file" == *.go ]]; then
        echo "Checking $file for whitespace/import-only changes..."
        git show HEAD:"$file" | goimports > /tmp/original.go
        goimports < "$file" > /tmp/current.go

        if diff -q /tmp/original.go /tmp/current.go >/dev/null; then
            if [ "$force_revert" = true ]; then
                git checkout HEAD "$file"
                echo "Reverted $file (only whitespace/import changes)"
            else
                echo "Would revert $file (only whitespace/import changes)"
            fi
        fi

        rm -f /tmp/original.go /tmp/current.go
    fi
done
