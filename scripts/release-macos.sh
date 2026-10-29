#!/bin/sh
# 从打了标签、没有未提交修改的提交做 macOS（Apple Silicon）发布包，放到 dist/<版本>/：
#   yan-<版本>-macos-arm64.dmg       拖进「应用程序」安装
#   yan-<版本>-macos-arm64.app.zip   同一个 yan.app 的 zip（备用）
#   THIRD_PARTY_LICENSES.txt         第三方声明（yan.app 里也有一份）
#   SHA256SUMS
# zip 不带扩展属性（--norsrc）：里面只有本机的 com.apple.provenance，带上只会在解压处多出 __MACOSX。
# 用法：scripts/release-macos.sh v0.1.0-alpha.1
set -eu

VERSION=${1:-}
case "$VERSION" in
v[0-9]*) ;;
*)
	echo "用法：$0 v<版本>，如 v0.1.0-alpha.1" >&2
	exit 2
	;;
esac
cd "$(dirname "$0")/.."
export CGO_ENABLED=0
GO=${GO:-go}

fail() {
	echo "release: $*" >&2
	exit 1
}

# 发布包只能来自一个记下来的提交：工作区干净（被忽略的生成文件除外），HEAD 就是这个标签。
[ -z "$(git status --porcelain)" ] || fail "有未提交的修改或未跟踪的文件"
[ "$(git describe --exact-match --tags HEAD 2>/dev/null)" = "$VERSION" ] || fail "HEAD 上没有标签 $VERSION"
COMMIT=$(git rev-parse HEAD)
[ "$(uname -m)" = arm64 ] || fail "要在 Apple Silicon 的 Mac 上打包"

rm -rf build "dist/$VERSION"
make notices
env -u MYGO_UPDATER_PRIVATE_KEY "$GO" tool mygo build

APP=build/darwin-arm64/yan.app
EXE=$APP/Contents/MacOS/yan
DMG=$(ls build/darwin-arm64/*.dmg)
RES=$APP/Contents/Resources
[ -x "$EXE" ] || fail "没有 $EXE"

# 包里的东西
[ "$(lipo -archs "$EXE")" = arm64 ] || fail "可执行文件不是 arm64：$(lipo -archs "$EXE")"
[ "$(lipo -archs "$RES/libghostty-vt.dylib")" = arm64 ] || fail "libghostty-vt 不是 arm64"
want=$(sed -n '/"darwin-arm64"/,/}/s/.*"sha256": "\([0-9a-f]*\)".*/\1/p' "$("$GO" list -m -f '{{.Dir}}' github.com/egoist/mygo)/plugins/terminal/mygo-plugin.json")
got=$(shasum -a 256 "$RES/libghostty-vt.dylib" | cut -d' ' -f1)
[ "$want" = "$got" ] || fail "libghostty-vt 的 SHA-256 不对：$got，应为 $want"
cmp -s "$RES/THIRD_PARTY_LICENSES.txt" resources/THIRD_PARTY_LICENSES.txt || fail "包里没有第三方声明"
# 声明是按 bin/yan-app 生成的：包里的可执行文件链接的模块必须和它一样
deps() { "$GO" version -m "$1" | awk '$1 == "dep" || $1 == "=>" { print $2, $3 }'; }
[ "$(deps "$EXE")" = "$(deps bin/yan-app)" ] || fail "包里的可执行文件和生成声明用的 bin/yan-app 链接的模块不一样"
v=$("$EXE" --version)
case "$v" in
"yan-app ${VERSION}（${COMMIT%"${COMMIT#????????????}"}）") ;;
*) fail "版本号不对：$v" ;;
esac
codesign --verify --deep --strict "$APP" || fail "签名校验失败"
hdiutil verify "$DMG" >/dev/null || fail "磁盘映像校验失败"

OUT=dist/$VERSION
mkdir -p "$OUT"
cp "$DMG" "$OUT/yan-$VERSION-macos-arm64.dmg"
ditto -c -k --norsrc --keepParent "$APP" "$OUT/yan-$VERSION-macos-arm64.app.zip"
cp resources/THIRD_PARTY_LICENSES.txt "$OUT/THIRD_PARTY_LICENSES.txt"
mkdir -p .mygo
scripts/release-notes.sh "${VERSION#v}" > .mygo/update-notes.md
"$GO" run ./internal/tools/updatepack -bundle "$APP" -out "$OUT" -tag "$VERSION" -notes .mygo/update-notes.md
"$GO" run ./internal/tools/updatepack -check -out "$OUT" -tag "$VERSION"
(cd "$OUT" && shasum -a 256 yan-*.dmg yan-*.zip yan-*.update.tar.gz update-*.json THIRD_PARTY_LICENSES.txt >SHA256SUMS)

echo
echo "$v"
codesign -dv "$APP" 2>&1 | grep -E '^(Identifier|Signature|TeamIdentifier|Authority)=' || true
ls -l "$OUT"
cat "$OUT/SHA256SUMS"
