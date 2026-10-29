#!/usr/bin/env bash
# 把画 Mermaid 图的 Pele 打包成 ui/gui/pele.js.gz：一个 goja 能跑的 ES2017 脚本，设好全局的 Pele，
# 开头是 Pele 的许可证。脚本照搬 Mayoi f1498a9 的 scripts/pele.sh（MIT），只改了输出位置。要 Node 的 npx。
# 用法：scripts/pele.sh [版本，默认 1.9.0]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-1.9.0}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cd "$TMP"
npm init -y >/dev/null
npm install --silent --no-audit --no-fund "pele@$VERSION"
{
	printf '/*! Pele %s, https://github.com/obsidianmd/pele\n' "$VERSION"
	sed 's/^/ * /; s/ *$//' node_modules/pele/LICENSE
	printf ' */\n'
} >banner.js
npx --yes esbuild@0.25.0 node_modules/pele/dist/pele.js --bundle --format=iife --global-name=Pele \
	--target=es2017 --minify --legal-comments=none --outfile=pele.js
cat banner.js pele.js | gzip -9n >"$ROOT/ui/gui/pele.js.gz"
echo "Pele $VERSION in ui/gui/pele.js.gz ($(wc -c <"$ROOT/ui/gui/pele.js.gz" | tr -d ' ') bytes)"
