package app

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/internal/version"
)

// Features are what the guide teaches and the commands behind them, each with the version
// that brought it, so the guide, the command palette and the menus can tell people on the
// official release which entry points are not in their build yet. features.json is data, not
// labels: a feature still under CHANGELOG.md's "## Unreleased" says "since": "unreleased",
// and scripts/bump.sh writes the new version there when it cuts the release.

//go:embed features.json
var featuresJSON []byte

// sinceUnreleased is the since of a feature not in any tagged version yet.
const sinceUnreleased = "unreleased"

// feature is one entry of features.json.
type feature struct {
	ID       string   `json:"id"`
	Since    string   `json:"since"`
	Title    string   `json:"title"`
	Commands []string `json:"commands"`
}

var (
	features     = loadFeatures()
	commandSince = map[string]string{} // command → the since of its feature
)

func loadFeatures() map[string]feature {
	var list []feature
	if err := json.Unmarshal(featuresJSON, &list); err != nil {
		panic("features.json: " + err.Error())
	}
	out := make(map[string]feature, len(list))
	for _, f := range list {
		out[f.ID] = f
		for _, id := range f.Commands {
			commandSince[id] = f.Since
		}
	}
	return out
}

// buildRelease is the tagged version this build is, "" for a development build (tests set it).
var buildRelease = version.Release

// unreleased reports whether a feature that came in since is missing from the official
// release: always for one under "## Unreleased" (a development build has it, a release does
// not), and for a version later than the build's own.
func unreleased(since string) bool {
	if since == sinceUnreleased {
		return true
	}
	r := buildRelease()
	return since != "" && r != "" && versionLater(since, r)
}

// FeatureSince returns the version a feature came in, "unreleased" before its release.
func FeatureSince(id string) string { return features[id].Since }

// CommandUnreleased reports whether a command belongs to a feature the official release does
// not have yet: the menus and the palette badge it.
func CommandUnreleased(id string) bool { return unreleased(commandSince[id]) }

// versionLater reports whether version a comes after b, as scripts/bump.sh orders them:
// X.Y.Z, a preview -alpha.N before its release.
func versionLater(a, b string) bool {
	ka, kb := versionKey(a), versionKey(b)
	for i := range ka {
		if ka[i] != kb[i] {
			return ka[i] > kb[i]
		}
	}
	return false
}

func versionKey(v string) [4]int {
	v = strings.TrimPrefix(v, "v")
	core, pre, _ := strings.Cut(v, "-alpha.")
	var k [4]int
	for i, p := range strings.SplitN(core, ".", 3) {
		k[i], _ = strconv.Atoi(p)
	}
	k[3] = 1 << 30 // a release after all its previews
	if pre != "" {
		k[3], _ = strconv.Atoi(pre)
	}
	return k
}

// PretendRelease makes the build report release v until restore runs: for the frontends'
// tests and screenshots of the badges, never for a shipped build.
func PretendRelease(v string) (restore func()) {
	old := buildRelease
	buildRelease = func() string { return v }
	return func() { buildRelease = old }
}
