#!/bin/bash

set -e

# remove generated files first
rm *.css.go *.png.go *.js.go *.svg.go 2>/dev/null || true

# then re-create them
find . -name '*.css' -or -name '*.png' -or -name '*.js' -or -name '*.svg' \
    | while read name; do
        name="${name#./*}"

        go_name="${name//\//_}"

        func_name="${go_name//./_}"
        func_name="${func_name//-/_}"

        go-bindata -toc -out="${go_name}.go" -pkg=standalone -func "$func_name" "$name"
      done
