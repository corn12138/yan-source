#!/usr/bin/env bash
# Bundles MathJax, which draws TeX math, into ui/gui/mathjax.js.gz:
# one ES2017 script, which goja runs, that sets the global YanMath, its
# render(tex, display) drawing a formula as SVG, its glyphs paths, with
# MathJax's license at its top. Needs Node's npx.
# Usage: scripts/mathjax.sh [version, 3.2.2 by default]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-3.2.2}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cd "$TMP"
npm init -y >/dev/null
npm install --silent --no-audit --no-fund "mathjax-full@$VERSION"
cat >entry.js <<'JS'
import {mathjax} from 'mathjax-full/js/mathjax.js';
import {TeX} from 'mathjax-full/js/input/tex.js';
import {SVG} from 'mathjax-full/js/output/svg.js';
import {liteAdaptor} from 'mathjax-full/js/adaptors/liteAdaptor.js';
import {RegisterHTMLHandler} from 'mathjax-full/js/handlers/html.js';
import 'mathjax-full/js/input/tex/base/BaseConfiguration.js';
import 'mathjax-full/js/input/tex/ams/AmsConfiguration.js';
import 'mathjax-full/js/input/tex/newcommand/NewcommandConfiguration.js';
import 'mathjax-full/js/input/tex/boldsymbol/BoldsymbolConfiguration.js';

const adaptor = liteAdaptor();
RegisterHTMLHandler(adaptor);
export function render(tex, display) {
// A fresh input processor prevents document macros from affecting another formula.
const doc = mathjax.document('', {
	InputJax: new TeX({packages: ['base', 'ams', 'newcommand', 'boldsymbol'], maxBuffer: 32768, maxMacros: 1000}),
	OutputJax: new SVG({fontCache: 'none'}),
});
	return adaptor.innerHTML(doc.convert(tex, {display}));
}
JS
{
	printf '/*! MathJax %s, https://github.com/mathjax/MathJax-src\n' "$VERSION"
	sed 's/^/ * /; s/ *$//' node_modules/mathjax-full/LICENSE
	printf ' */\n'
} >banner.js
npx --yes esbuild@0.25.0 entry.js --bundle --format=iife --global-name=YanMath \
	--target=es2017 --minify --legal-comments=none --outfile=mathjax.js
cp node_modules/mathjax-full/LICENSE "$ROOT/packaging/licenses/mathjax.txt"
cat banner.js mathjax.js | gzip -9n >"$ROOT/ui/gui/mathjax.js.gz"
echo "MathJax $VERSION in ui/gui/mathjax.js.gz ($(wc -c <"$ROOT/ui/gui/mathjax.js.gz" | tr -d ' ') bytes)"

