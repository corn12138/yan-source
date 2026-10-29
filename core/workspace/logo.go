package workspace

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

// A project's logo, for its square on the rail, follows Mayoi (MIT, cmd/mayoi/projects.go
// logoCandidates, extractIconHref and resolveIconHref at 8727168): the usual favicon and logo
// files first, then an icon the project's HTML or TSX sources declare. Reading is bounded: only
// the head of a source file, and nothing that is not a regular file.

// logoCandidates are a project's favicon or logo files, in order: the root and public/ first,
// then app/, src/ and the asset folders.
var logoCandidates = []string{
	"favicon.svg", "favicon.ico", "favicon.png", "favicon.webp",
	"logo.svg", "logo.png", "logo.webp",
	"icon.svg", "icon.png", "icon.webp",
	"public/favicon.svg", "public/favicon.ico", "public/favicon.png", "public/favicon.webp",
	"public/logo.svg", "public/logo.png", "public/logo.webp",
	"public/icon.svg", "public/icon.png", "public/icon.webp",
	"public/apple-touch-icon.png",
	"app/favicon.ico", "app/favicon.png", "app/favicon.webp",
	"app/icon.svg", "app/icon.png", "app/icon.webp", "app/icon.ico",
	"app/apple-icon.png",
	"src/favicon.ico", "src/favicon.svg",
	"src/app/favicon.ico", "src/app/icon.svg", "src/app/icon.png", "src/app/icon.ico",
	"src/app/apple-icon.png",
	"src/assets/logo.svg", "src/assets/logo.png",
	"assets/icon.svg", "assets/icon.png",
	"assets/logo.svg", "assets/logo.png",
	"static/logo.svg", "static/logo.png",
}

// iconSourceFiles may declare the icon with a <link rel="icon"> or icon metadata.
var iconSourceFiles = []string{
	"index.html", "public/index.html", "src/index.html", "site/index.html", "docs/index.html",
	"app/layout.tsx", "src/app/layout.tsx", "app/routes/__root.tsx", "src/routes/__root.tsx",
	"app/root.tsx", "src/root.tsx",
}

// iconSourceHead is how much of a source file is read: the head declares the icon, and a
// minified bundle must not cost megabytes of regular expressions.
const iconSourceHead = 64 << 10

var (
	linkTagRe  = regexp.MustCompile(`(?i)<link\b[^>]*>`)
	relAttrRe  = regexp.MustCompile(`(?i)\srel\s*=\s*["']([^"']+)["']`)
	hrefAttrRe = regexp.MustCompile(`(?i)\shref\s*=\s*["']([^"']+)["']`)
	iconRelRe  = regexp.MustCompile(`(?:[{,\s])rel\s*:\s*["']([^"']+)["']`)
	iconHrefRe = regexp.MustCompile(`(?:[{,\s])href\s*:\s*["']([^"'?#]+)`)
	iconURLRe  = regexp.MustCompile(`(?:[{,\s])url\s*:\s*["']([^"'?#]+)`)
	iconStrRe  = regexp.MustCompile(`(?:[{,\s])icon\s*:\s*["']([^"'?#]+)["']`)
	iconAppRe  = regexp.MustCompile(`(?:[{,\s])apple\s*:\s*["']([^"'?#]+)["']`)
)

// LogoFiles returns the files that may be the logo of the project at root, best first: the
// candidates that exist, then those a source declares. A frontend shows the first it can decode.
func LogoFiles(root string) []string {
	var out []string
	add := func(rel string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	for _, rel := range logoCandidates {
		add(rel)
	}
	for _, src := range iconSourceFiles {
		data, err := readHead(filepath.Join(root, filepath.FromSlash(src)))
		if err != nil {
			continue
		}
		href := extractIconHref(string(data))
		if href == "" {
			continue
		}
		base := path.Dir(src)
		if base == "." {
			base = ""
		}
		for _, rel := range resolveIconHref(href, base) {
			add(rel)
		}
	}
	return out
}

// readHead reads the head of a regular file, opening it without blocking so that a FIFO
// neither stalls the scan nor leaks it.
func readHead(p string) ([]byte, error) {
	f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil {
		return nil, err
	} else if !fi.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	return io.ReadAll(io.LimitReader(f, iconSourceHead))
}

// extractIconHref returns the icon an HTML or TSX source declares, or "": its path, keeping
// the leading slash of one from the site's root.
func extractIconHref(source string) string {
	for _, tag := range linkTagRe.FindAllString(source, -1) {
		rel := relAttrRe.FindStringSubmatch(tag)
		if rel == nil || !hasIconRel(rel[1]) {
			continue
		}
		if href := hrefAttrRe.FindStringSubmatch(tag); href != nil {
			if clean := declaredIconHref(href[1]); clean != "" {
				return clean
			}
		}
	}
	// Icon metadata counts when rel and href share a brace-free run; Next.js metadata also
	// names icons with url:, or a bare icon: or apple: path.
	for _, run := range strings.Split(source, "}") {
		if rel := iconRelRe.FindStringSubmatch(run); rel != nil && hasIconRel(rel[1]) {
			for _, re := range []*regexp.Regexp{iconHrefRe, iconURLRe} {
				if m := re.FindStringSubmatch(run); m != nil {
					if clean := declaredIconHref(m[1]); clean != "" {
						return clean
					}
				}
			}
			continue
		}
		for _, re := range []*regexp.Regexp{iconStrRe, iconAppRe} {
			if m := re.FindStringSubmatch(run); m != nil {
				if clean := declaredIconHref(m[1]); clean != "" {
					return clean
				}
			}
		}
	}
	return ""
}

func hasIconRel(rel string) bool {
	for _, f := range strings.Fields(strings.ToLower(rel)) {
		if f == "icon" || f == "apple-touch-icon" {
			return true
		}
	}
	return false
}

// cleanIconHref takes the path of an icon href, or "" for a data URI, a remote URL, nothing,
// or a path that leaves the project.
func cleanIconHref(href string) string {
	href = strings.TrimSpace(href)
	if i := strings.IndexAny(href, "?#"); i >= 0 {
		href = href[:i]
	}
	if href == "" || strings.HasPrefix(href, "data:") || strings.Contains(href, "://") || strings.HasPrefix(href, "//") {
		return ""
	}
	href = strings.TrimPrefix(strings.TrimPrefix(href, "./"), "/")
	if href == "" || strings.HasSuffix(href, "/") {
		return ""
	}
	for _, seg := range strings.FieldsFunc(href, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return ""
		}
	}
	return href
}

func declaredIconHref(href string) string {
	clean := cleanIconHref(href)
	if clean != "" && strings.HasPrefix(strings.TrimSpace(href), "/") {
		return "/" + clean
	}
	return clean
}

// resolveIconHref maps an icon href declared by a source in folder base ("" for the root) to
// the files it may be: a relative href is of base first; one from the site's root names
// public/, then the root, then base as a site of its own.
func resolveIconHref(href, base string) []string {
	clean := cleanIconHref(href)
	if clean == "" {
		return nil
	}
	inBase := clean
	if base != "" {
		inBase = base + "/" + clean
	}
	paths := []string{inBase, "public/" + clean, clean}
	if strings.HasPrefix(strings.TrimSpace(href), "/") {
		paths = []string{"public/" + clean, clean, inBase}
	}
	var out []string
	for _, p := range paths {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}
