#!/bin/sh
# 打印 CHANGELOG.md 里「## <版本>」那一节（不带标题），给发布说明的「What's new」用；没有这一节或它是空的就失败。
# 照着 Mayoi（MIT，scripts/release-notes.sh）。
# 用法：scripts/release-notes.sh 0.1.0-alpha.4
set -eu

[ $# -eq 1 ] || { echo "用法：$0 <版本>，如 0.1.0-alpha.4" >&2; exit 2; }
NOTES=$(awk -v heading="## $1" '
	$0 == heading { found = 1; next }
	found && /^## / { exit }
	found && /./ { printf "%s", blanks; blanks = ""; started = 1; print; next }
	found && started { blanks = blanks "\n" }
' "$(dirname "$0")/../CHANGELOG.md")
[ -n "$NOTES" ] || { echo "CHANGELOG.md 里「## $1」下面没有内容" >&2; exit 1; }
printf '%s\n' "$NOTES"
