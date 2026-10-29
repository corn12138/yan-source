package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// moduleRoot is the repository, two folders up from this package.
const moduleRoot = "../.."

// sourceFiles are the module's Go files that ship: tests and build tools left out.
func sourceFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && rel != "." || rel == "build" || rel == "dist" || rel == filepath.Join("internal", "tools") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func literal(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	return s, err == nil
}

// messages are the catalog keys the source uses, by the file using them: the literal
// arguments of T, TC, F, N, Error and In.
func messages(t *testing.T) map[string]string {
	t.Helper()
	used := map[string]string{}
	for path, f := range sourceFiles(t) {
		local := path == "core/i18n/i18n.go"
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); ok && x.Name == "i18n" {
					name = fn.Sel.Name
				}
			case *ast.Ident:
				if local {
					name = fn.Name
				}
			}
			args := call.Args
			switch name {
			case "T", "F", "N", "Error":
				if len(args) > 0 {
					if s, ok := literal(args[0]); ok {
						used[s] = path
					}
				}
			case "TC":
				if len(args) == 2 {
					c, ok1 := literal(args[0])
					s, ok2 := literal(args[1])
					if ok1 && ok2 {
						used[c+"\x04"+s] = path
					}
				}
			case "In":
				if len(args) == 2 {
					if s, ok := literal(args[1]); ok {
						used[s] = path
					}
				}
			}
			return true
		})
	}
	return used
}

// Every message the source shows has a Chinese translation, and the catalog keeps none that
// nothing shows: an English text changed without its catalog entry fails here.
func TestCatalogCoversSource(t *testing.T) {
	used := messages(t)
	if len(used) < 1000 {
		t.Fatalf("found only %d messages: the scan is broken", len(used))
	}
	var missing, unused []string
	for msg, path := range used {
		if _, ok := zhHans[msg]; !ok {
			missing = append(missing, path+": "+strconv.Quote(msg))
		}
	}
	for msg := range zhHans {
		if _, ok := used[msg]; !ok {
			unused = append(unused, strconv.Quote(msg))
		}
	}
	slices.Sort(missing)
	slices.Sort(unused)
	for _, m := range missing {
		t.Errorf("no Chinese translation: %s", m)
	}
	for _, m := range unused {
		t.Errorf("catalog entry nothing uses: %s", m)
	}
}

// keptChinese are the string literals that stay Chinese in every language: native language
// names, a label naming both languages, a sample showing a font's Chinese width, the welcome
// page's mark, a self-test's document text, and a tab path that must not change with the
// language. Constants named …Template… are new files' text, chosen by locale.
var keptChinese = map[string]bool{
	// Practice paths are persistent identities across language changes.
	"编辑基础.go": true,
	"多光标.go":  true,
	"导航.go":   true,
	"转到定义.go": true,
	"布局.go":   true,
	"最大化.go":  true,
	"笔记.md":   true,
	"新文件.txt": true,
	"冲突.go":   true,
	"说明.md":   true,

	"简体中文":                      true,
	"Interface Language / 界面语言": true,
	"Aa 0O 1lI {} 中文注释":         true,
	"砚":                         true,
	"// 未保存的修改\n":               true,
	"所有改动":                      true,
	// Full-width prefixes typed with a Chinese input method, read as their ASCII twins.
	"：": true, "》": true, "＞": true, "＠": true,
	// The update self-test reads --version of releases built before English existed.
	"（": true,
}

// No Yan-owned text is left out of the catalogs: a Chinese literal in shipped code is either
// translated through it or listed in keptChinese.
func TestNoUntranslatedChinese(t *testing.T) {
	han := func(s string) bool {
		return strings.IndexFunc(s, func(r rune) bool {
			// Han characters, and the full-width punctuation Chinese text is written with.
			return unicode.Is(unicode.Han, r) || r >= 0x3000 && r <= 0x303f || r >= 0xff00 && r <= 0xffef
		}) >= 0
	}
	for path, f := range sourceFiles(t) {
		if path == "core/i18n/zh_hans.go" || strings.HasSuffix(path, "_selftest.go") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if v, ok := n.(*ast.ValueSpec); ok && len(v.Names) == 1 && strings.Contains(v.Names[0].Name, "Template") {
				return false
			}
			if bl, ok := n.(*ast.BasicLit); ok {
				if s, ok := literal(bl); ok && han(s) && !keptChinese[s] {
					t.Errorf("%s: untranslated %q", path, s)
				}
			}
			return true
		})
	}
}
