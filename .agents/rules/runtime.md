# Runtime configuration

- Generate service credentials and MCP settings through `internal/pkg/setup/`. Keep real values out of examples, instructions, test fixtures and logs.
- Update `.env.example` and `docs/configuration.md` when operator settings change.
- Keep the browser viewer behind the existing tailnet access path. Test deployment changes with disposable credentials and isolated fixtures.
