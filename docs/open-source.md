# Source distribution

Yan's source is licensed under MIT. The public source starts with a clean snapshot of the
alpha.32 codebase plus the changes listed under **Unreleased** in `CHANGELOG.md`. Its initial
commit does not include the earlier development repository's commits, branches, tags, pull
requests, or CI logs. Earlier release notes and validation reports describe their recorded
versions; historical development links in those records may require access to the original
repository.

The source contains both frontends, the shared core, tests, build scripts, documentation, and
the bundled resources needed to build. Machine-generated executables, signing keys, local
configuration/session data, and internal handoff documents are excluded. Third-party copyright
notices, license texts, and the update verification public key remain included.

Installers and signed update feeds continue to use
[`corn12138/yan-releases`](https://github.com/corn12138/yan-releases). Source snapshots are development
builds until a new release is tagged and packaged; publishing source does not replace an installed
application or publish a new installer.

See [CONTRIBUTING.md](../CONTRIBUTING.md) for builds and release checks, and
[SECURITY.md](../SECURITY.md) for source scans and privacy guidance. A scan does not establish that
all runtime behavior is free of vulnerabilities; changes still need normal source review and
appropriate tests.
