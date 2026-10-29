# 开发与发布

## 开发

```sh
make dev              # 编译窗口版并打开当前目录；配置、会话、keybindings.json 都在 build/dev-config，不碰 ~/.config/yan
make run              # 终端版
make check            # 提交前：gofmt、vet、测试、竞态检测、架构边界、体积门禁（终端版 ≤10 MB）
SHOTS=$PWD/build/shots go test ./ui/gui   # 无头界面测试顺便截图
```

编辑终端标签、焦点或拖动时，另运行 `CGO_ENABLED=0 go test -tags selftest ./ui/gui -run TestNativeTerminal`。
这组测试加载 libghostty-vt，以模拟连接验证真实终端控件，不启动用户 shell；原生库需已在本机缓存或应用资源中。
评审交接另有 `TestNativeTerminalReviewHandoff` 和真实窗口 `YAN_SELFTEST_MODE=review`；后者使用独立目录中的 `review.go`，验证多行粘贴未提交及撤销清空。
真实窗口的 shell 验证入口为 `YAN_SELFTEST_MODE=terminal`（`-tags selftest` 构建、独立配置）。
窗口复用、Dock 回调与真实第二进程投递的入口为 `YAN_SELFTEST_MODE=window-workflows`；
测试目录准备 `dirty.txt`、`other/` 和指向自身的 `alias` 目录链接，并使用独立配置与 `XDG_RUNTIME_DIR`。

改动用户看得到的，都在 `CHANGELOG.md` 的 `## Unreleased` 下加一条（英文，写给用砚的人看）。
一个功能一个分支（`codex/<主题>`），做完开 PR 合进 `main`。

公式预览原生验收使用 `YAN_SELFTEST_MODE=math`（`-tags selftest`、独立目录和配置），覆盖深浅色、双栏、字号、源码编辑/撤销和配置热重载。

## CI

`.github/workflows/ci.yml` 只在 PR 上跑（也能在 Actions 页面手动运行），一台 Linux：gofmt、vet、
macOS 的交叉编译、全部测试。只改文档的 PR 不跑，同一个 PR 推了新提交会取消还在跑的那次。
GitHub Actions 按分钟计费，所以竞态检测、体积门禁、打包和签名都不在 CI 里，留给本机的 `make check` 和 `make release`。

## 发布

1. `## Unreleased` 写好、提交、合进 `main`，推上去。想自己写发布说明的，先把 `docs/releases/v<版本>.md` 写好提交。
2. 在 `main` 上：

   ```sh
   scripts/bump.sh alpha "macOS preview with …"   # 或 patch、minor、major，或者写明版本，如 0.2.0
   ```

   它检查在 `main` 上、没有未提交的修改、和 `origin/main` 一样、标签还没有；把 `## Unreleased` 改成版本号，
   没写发布说明的按 CHANGELOG 生成 `docs/releases/v<版本>.md`，换掉 README 里的版本号，
   提交成 `Release v<版本>`、打附注标签，问过再把 `main` 和标签一起推上去。
3. 在 Apple Silicon 的 Mac 上：

   ```sh
   make release VERSION=v<版本>        # dist/v<版本>/：dmg、zip、第三方声明、SHA256SUMS，逐项校验
   scripts/publish-macos.sh v<版本>    # 问过再在 GitHub 上发布（预览版标成 Pre-release）
   ```

发布包只是 ad-hoc 签名、没有公证：要 Apple 开发者账号（Developer ID 和公证的凭据）才能改。


## Signed updates and public distribution

Source is distributed under the MIT license. Installers, signed update archives, manifests,
checksums, and license notices use the separate `corn12138/yan-releases` repository. Tags in that
distribution repository refer to its README, independently of source tags.

Before contributing, read [SECURITY.md](SECURITY.md) and run its source and history scans.

The existing update key lives at `~/.config/yan/update-keys/mygo-update.key` (0600, directory 0700).
Back it up securely; do not regenerate it for a release. The public key is embedded via `mygo.json`.
Another build machine may supply `YAN_UPDATE_KEY_FILE` or `MYGO_UPDATER_PRIVATE_KEY`; never commit
or print the secret. `make release` fails if the key is missing or does not match the embedded key.
The Ed25519 update signature is separate from Developer ID signing and Apple notarization.

`make release` packages the final app with `internal/tools/updatepack`, preserving the complete
alpha tag in the manifest. MyGo's numeric bundle version stays suitable for macOS; runtime update
comparisons use the tagged Go build version. Update archives use MyGo's format and verification.
Deltas are disabled in this first integration. Ordinary development builds still use `make app`.

`publish-macos.sh` first publishes immutable numbered assets, downloads and verifies the update
anonymously, then promotes the `preview` or `stable` manifest. Stable releases may also advance the
preview channel; channel versions never move backwards. If promotion is interrupted after a release
is published, retry `scripts/promote-updates.sh v<version> preview` (or `stable preview` for stable).
It must run from that release's checkout with its `dist` files. It never replaces numbered assets.

Channel promotion uses `internal/tools/promoteupdates` after verifying the public numbered
archive against the embedded update key. Only an HTTP 404 from the channel tag lookup permits
creating a release; authentication failures, timeouts, and asset-list errors stop the operation.
The tool uploads and anonymously checks a content-addressed pending manifest before renaming
the current one to a retained `.previous-<asset-id>` name and activating the pending asset.
Lost mutation responses are resolved by reading asset identities before retrying. Cancellation
uses a separate bounded recovery context to restore the previous name when necessary.

Run publishers serially. GitHub's two asset renames are not an atomic swap, so a short missing-name
interval is still possible. A process kill or continued network outage can require rerunning
`scripts/promote-updates.sh`; it restores the newest retained pending/previous manifest before
checking version order. It never downgrades a known channel or changes bytes for the same version.
An existing empty channel without any recovery asset stops with a diagnostic, since its last
version cannot be inferred safely. Keep retained manifests; do not delete them during an incident.
The final public manifest must match the requested release before promotion reports success.
Identity checks read each asset by its ID, because GitHub can serve a renamed asset's old bytes under
the download name for a while. The public name is then retried for about two minutes; if it is still
stale, the channel itself is already updated, and rerunning `scripts/promote-updates.sh` confirms it
without staging or renaming anything.
GitHub's [release asset API](https://docs.github.com/en/rest/releases/assets#update-a-release-asset)
documents the rename operation used here.

The native self-test `YAN_SELFTEST_MODE=update-restart` checks updater-window creation and a real
relaunch with a saved dirty buffer; prepare `edited.txt` and independent configuration. The
`update-feed` mode verifies the published feed and replaces its own disposable verification bundle
with the signed release. Set `YAN_SELFTEST_UPDATE_VERSION` to the version without `v`, embed the
public feed/key and production flag, and never run that mode from an installed user application.

### Desktop proxy verification

Update traffic uses `internal/updatehttp` and the `platform/netproxy` provider. Explicit
HTTP_PROXY/HTTPS_PROXY and NO_PROXY settings take precedence; macOS desktop launches fall
back to the effective system HTTP/HTTPS/SOCKS proxy and exception list. Settings refresh
within 15 seconds. PAC-only proxy configurations are reported as unsupported. Windows/Linux
currently retain environment-proxy support with a separate native provider boundary.
TLS and update signature checks are unchanged. Unrelated HTTP requests retain their original
transport. To test a desktop launch, run the existing disposable `update-feed` self-test with
all upper/lower-case proxy environment variables cleared; never target the installed user app.
