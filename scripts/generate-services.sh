#!/bin/bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
# Remove stale registration imports before the implementation scanner loads packages.
rm -f internal/pkg/services/services.gen.go
export LOG_LEVEL=warn
exec bash scripts/make/servicepack/service_registration.sh
