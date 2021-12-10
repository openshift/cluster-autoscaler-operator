#!/bin/sh
# Example:  ./hack/go-lint.sh installer/... pkg/... tests/smoke

golint -set_exit_status "${@}"
