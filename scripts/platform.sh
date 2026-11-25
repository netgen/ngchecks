#!/bin/sh
# Prints the platform of this machine, e.g. darwin_arm64.
set -eu

. "$(dirname -- "$0")/lib.sh"

host_platform
