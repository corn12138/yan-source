#!/bin/sh
# 定下新版本：CHANGELOG.md 的「## Unreleased」改成「## <版本>」，按它写好 docs/releases/v<版本>.md
# （已经有就用写好的），README 里的版本号跟着换，mygo.json 的 version 换成版本的数字部分；
# 提交成「Release v<版本>」，打上同名的附注标签，问过之后把 main 和标签一起推上去。
# 之后在这台 Mac 上 make release VERSION=v<版本> 做发布包，scripts/publish-macos.sh v<版本> 发到 GitHub。
# 照着 Mayoi（MIT，scripts/bump.sh），多了预览版（-alpha.N）。
#
# 用法：scripts/bump.sh <版本 | alpha | patch | minor | major> [一句话说明]
#   alpha  下一个预览版：0.1.0-alpha.3 → 0.1.0-alpha.4；正式版之后是下一个补丁版的 alpha.1
#   patch  预览版之后是它的正式版（0.1.0-alpha.3 → 0.1.0），否则 0.1.0 → 0.1.1；minor、major 同理
#   说明写进标签，如「macOS preview with Markdown preview」
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
[ $# -ge 1 ] && [ $# -le 2 ] || { sed -n '2,12s/^# \{0,1\}//p' "$0" >&2; exit 2; }
ARG=$1
SUMMARY=${2:-macOS preview}

fail() {
	echo "bump: $*" >&2
	exit 1
}

PREV=$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null) || fail "还没有 v 开头的标签"
PREV=${PREV#v}
semver='^[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\(-alpha\.[0-9][0-9]*\)\{0,1\}$'
echo "$PREV" | grep -q "$semver" || fail "上一个标签 v$PREV 不是 X.Y.Z 或 X.Y.Z-alpha.N"

# next 按 ARG 算出新版本。
next() {
	echo "$PREV" | awk -F'[.-]' -v arg="$1" '{
		major = $1; minor = $2; patch = $3; pre = ($4 == "alpha") ? $5 : -1
		if (arg == "alpha") {
			if (pre >= 0) { print major "." minor "." patch "-alpha." pre + 1 }
			else { print major "." minor "." patch + 1 "-alpha.1" }
		} else if (arg == "patch") {
			if (pre >= 0) { print major "." minor "." patch } else { print major "." minor "." patch + 1 }
		} else if (arg == "minor") {
			if (pre >= 0 && patch == 0) { print major "." minor ".0" } else { print major "." minor + 1 ".0" }
		} else if (arg == "major") {
			if (pre >= 0 && minor == 0 && patch == 0) { print major ".0.0" } else { print major + 1 ".0.0" }
		}
	}'
}
case "$ARG" in
alpha | patch | minor | major) VERSION=$(next "$ARG") ;;
*) VERSION=${ARG#v} ;;
esac
echo "$VERSION" | grep -q "$semver" || fail "$VERSION 不是 X.Y.Z 或 X.Y.Z-alpha.N"

# later 报告 $1 是不是排在 $2 后面（同一个 X.Y.Z 的预览版在正式版前面，同 semver）。
later() {
	printf '%s\n%s\n' "$1" "$2" | awk -F'[.-]' '
		{ k[NR] = sprintf("%09d%09d%09d%09d", $1, $2, $3, ($4 == "alpha") ? $5 : 999999999) }
		END { exit !(k[1] > k[2]) }'
}
later "$VERSION" "$PREV" || fail "$VERSION 不在上一个版本 $PREV 之后"
TAG=v$VERSION
BASE=${VERSION%%-*}

[ "$(git symbolic-ref --short HEAD 2>/dev/null)" = main ] || fail "要在 main 上发布：先切过去"
[ -z "$(git status --porcelain)" ] || fail "有未提交的修改：先提交"
git fetch --quiet --tags origin main
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || fail "main 和 origin/main 不一样：先拉取或推送"
if git rev-parse --verify --quiet "refs/tags/$TAG" >/dev/null || [ -n "$(git ls-remote --tags origin "refs/tags/$TAG")" ]; then
	fail "标签 $TAG 已经有了"
fi
! gh release view "$TAG" --repo corn12138/yan-releases >/dev/null 2>&1 || fail "GitHub 上已经有 $TAG 的发布"

# CHANGELOG：已经有「## <版本>」就用它，否则把有内容的「## Unreleased」改成它。
if grep -q "^## $VERSION\$" CHANGELOG.md; then
	:
elif scripts/release-notes.sh Unreleased >/dev/null 2>&1; then
	sed -i '' "s/^## Unreleased\$/## $VERSION/" CHANGELOG.md
else
	fail "CHANGELOG.md 里既没有「## $VERSION」，「## Unreleased」下也没有内容：先写好、提交"
fi
NEWS=$(scripts/release-notes.sh "$VERSION")
FILES="CHANGELOG.md"

# features.json：「## Unreleased」下的功能（"since": "unreleased"）从这个版本起有了，导览、菜单和命令面板不再标「未发布」。
if grep -q '"since": "unreleased"' core/app/features.json; then
	sed -i '' "s/\"since\": \"unreleased\"/\"since\": \"$VERSION\"/" core/app/features.json
	FILES="$FILES core/app/features.json"
fi

# mygo.json 只能是数字（Apple 的 CFBundleShortVersionString）：只在 X.Y.Z 变了时改。
if ! grep -q "\"version\": \"$BASE\"" mygo.json; then
	sed -i '' "s/\"version\": \"[^\"]*\"/\"version\": \"$BASE\"/" mygo.json
	FILES="$FILES mygo.json"
fi

# Each installed build keeps its channel; a stable release also advances preview clients at publication.
CHANNEL=stable
case "$VERSION" in *-alpha.*) CHANNEL=preview ;; esac
if ! grep -q "releases/download/$CHANNEL\"" mygo.json; then
 sed -E -i '' "s@releases/download/(preview|stable)@releases/download/$CHANNEL@" mygo.json
 FILES="$FILES mygo.json"
fi

# README 的安装说明写着上一个版本号。
if grep -q "v$PREV" README.md; then
	sed -i '' "s/v$PREV/$TAG/g" README.md
	FILES="$FILES README.md"
fi

# 发布说明：没写好的，按 CHANGELOG 和固定的下载、安装说明生成。
NOTES=docs/releases/$TAG.md
if [ ! -f "$NOTES" ]; then
	KIND="macOS release"
	BANNER="> Apple Silicon only, macOS 13 or newer. Ad-hoc signed and **not notarized**."
	case "$VERSION" in *-alpha.*)
		KIND="macOS preview"
		BANNER="> **Preview, not production.** Apple Silicon only, macOS 13 or newer. Ad-hoc signed and **not notarized**."
		;;
	esac
	cat >"$NOTES" <<NOTES
yan (砚) $TAG, $KIND for Apple Silicon.

$BANNER

## What's new

$NEWS

## Download

| File | What it is |
|---|---|
| \`yan-$TAG-macos-arm64.dmg\` | **Recommended.** Disk image: drag \`yan\` to Applications |
| \`yan-$TAG-macos-arm64.app.zip\` | The same \`yan.app\`, zipped (fallback) |
| \`SHA256SUMS\` | SHA-256 checksums of the release files |
| \`yan-$TAG-darwin-arm64.update.tar.gz\`, \`update-darwin-arm64.json\` | Signed automatic-update archive and manifest |
| \`THIRD_PARTY_LICENSES.txt\` | Third-party copyright and license texts (also inside the app) |

## Install or update

From alpha.11 onward, **Help → 检查更新…** can install signed updates. Automatic checks can be disabled in the same menu. Older versions need this one-time manual installation.

1. Download the \`.dmg\` and \`SHA256SUMS\` (no GitHub login required, or \`gh release download $TAG -R corn12138/yan-releases\`), then run \`shasum -a 256 -c SHA256SUMS --ignore-missing\` in the download folder. Each file should say \`OK\`.
2. **Updating:** quit yan first (Cmd+Q). Your settings and sessions are kept in \`~/.config/yan/\`.
3. Open the \`.dmg\` and drag **yan** onto **Applications**; choose **Replace** if asked.
4. Double-click yan. If macOS says it cannot verify the app (it may ask again, because this is a new ad-hoc signed build), click **Done**, then open **System Settings › Privacy & Security** (系统设置 › 隐私与安全性) and click **Open Anyway** (仍要打开). Confirm, then click **Open**.
5. Check the status bar: it should read **预览版 $TAG** (a stable release shows **$TAG**).
NOTES
	FILES="$FILES $NOTES"
fi

echo "==> v$PREV → $TAG"
git add $FILES
git --no-pager diff --cached --stat
git commit --quiet -m "Release $TAG"
git tag -a "$TAG" -m "yan $TAG: $SUMMARY (Apple Silicon, ad-hoc signed)"
git --no-pager log -1 --format='%h %s' "$TAG"
echo
echo "==> What's new（来自 CHANGELOG.md）："
echo "$NEWS"
echo

printf '把 main 和 %s 推到 origin 吗？[y/N] ' "$TAG"
read -r answer || answer=
case "$answer" in
[yY]*) ;;
*)
	echo "没推。以后推：git push --atomic origin main $TAG"
	echo "撤销：git tag -d $TAG && git reset --hard HEAD~1"
	exit 0
	;;
esac
git push --quiet --atomic origin main "$TAG"
echo "==> 推上去了。接着：make release VERSION=$TAG && scripts/publish-macos.sh $TAG"
