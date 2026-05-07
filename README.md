# ctrssh

Expose a remote container as a regular SSH endpoint.

```bash
ctrssh add work --host me@server --container devbox --user vscode
ctrssh prepare work
ctrssh config-ssh
ssh work.dev
```

See `docs/superpowers/specs/` for design.
