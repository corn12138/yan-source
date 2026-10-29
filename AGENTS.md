# AGENTS.md

- 每个用户看得到的改动，都在 `CHANGELOG.md` 的 `## Unreleased` 下加一条（英文，写给用砚的人看：做了什么、怎么用）；
  没有这一节就在最新的版本上面建一个。只改内部的不写。
- 提交前跑 `make check`（gofmt、vet、测试、竞态检测、架构边界、体积门禁），失败就不提交。
- 发布用 `scripts/bump.sh`，流程见 `CONTRIBUTING.md`。
- 窗口版以后还要出 Windows 和 Linux 版：快捷键走 `keymap.Profile.OS` / `core/app/platform.go`，
  「访达」「废纸篓」这类叫法走 `app.FileManager` / `app.TrashName`，界面上不要写死 ⌘ 或 Cmd。
