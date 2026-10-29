# Security and privacy

Yan is a local editor. Session state includes recently opened paths, cursor locations, and review
notes with source excerpts. Untitled recovery files contain document text. Crash logs can include
paths and stack data. Inspect these files and screenshots before sharing a bug report; replace
private data with a small synthetic example.

On macOS and Linux, newly saved session files, recovery files, and crash logs use owner-only file
permissions. Existing session permissions are updated on save; crash logs are tightened on startup.
Windows access control follows the user's configuration directory ACLs.

Opening an integrated terminal or running Git uses the local account's permissions, environment,
configured remotes, credential helpers, and Git hooks. Only run commands in repositories you trust.
Automatic update checks contact the public `corn12138/yan-releases` repository; they can be disabled
from the Help menu. Update downloads are verified against the public key in `mygo.json`.

## Reporting a vulnerability

Do not post credentials, private source, session files, or full crash logs in a public issue.
Use GitHub's **Report a vulnerability** control when it is available. Otherwise, open an issue
requesting a private reporting channel, without exploit details or sensitive attachments.

## Contributing source

Keep credentials, update-signing keys, local session data, build outputs, and internal handoff
documents out of commits. Ignore rules are preventive only: they do not remove tracked files or
erase Git history. Public keys and third-party copyright notices must remain in the source.

Install [Gitleaks 8.30.1](https://github.com/gitleaks/gitleaks/releases/tag/v8.30.1), then scan the
source tree and the history that will actually be published:

```sh
gitleaks dir . --redact --ignore-gitleaks-allow --max-archive-depth 2 --no-banner
gitleaks git . --log-opts='--all --full-history' --redact --ignore-gitleaks-allow --no-banner
```

The source-privacy workflow scans the tree and fetched history for every pull request, including documentation changes. Its rules
include credential detectors and machine-specific home and temporary paths. A clean scan is one
review aid; inspect filenames, commit identities, screenshots, compressed assets, and any proposed
exceptions as well. If a real credential is committed, revoke or rotate it before removing it from
published content.
