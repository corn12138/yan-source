package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Node 是目录树中的一项。子项在第一次展开时才读取（懒加载）。
type Node struct {
	Name     string
	Rel      string // 相对项目根，用 / 分隔；根节点为 ""
	Dir      bool
	Ignored  bool // 被 gitignore 忽略：显示但变暗
	Expanded bool
	Depth    int
	Children []*Node
	loaded   bool
}

// Tree 是资源管理器的数据模型，只在所有者 goroutine 上使用。
type Tree struct {
	Root *Node
	root string
	ig   *Ignore
}

// NewTree 创建以 root 为根的树，根目录默认展开。
func NewTree(root string, ig *Ignore) *Tree {
	t := &Tree{Root: &Node{Name: filepath.Base(root), Dir: true, Depth: -1}, root: root, ig: ig}
	t.Expand(t.Root)
	return t
}

// Expand 展开目录，必要时读取子项。
func (t *Tree) Expand(n *Node) {
	if !n.Dir {
		return
	}
	if !n.loaded {
		t.load(n)
	}
	n.Expanded = true
}

// Refresh 重新读取所有已展开的目录（获得焦点或保存后调用），保留展开状态。
func (t *Tree) Refresh() { t.refresh(t.Root) }

// RefreshDir reads again the directory at rel, if the tree has read it, keeping what
// is expanded below. It reports whether the directory was in the tree.
func (t *Tree) RefreshDir(rel string) bool {
	n := t.find(rel)
	if n == nil || !n.Dir || !n.loaded {
		return false
	}
	t.refresh(n)
	return true
}

// LoadedDirs returns the directories read and expanded, root first, relative to the root.
func (t *Tree) LoadedDirs() []string {
	var out []string
	var walk func(n *Node)
	walk = func(n *Node) {
		if !n.Dir || !n.loaded || (!n.Expanded && n != t.Root) {
			return
		}
		out = append(out, n.Rel)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(t.Root)
	return out
}

// find returns the node at rel among those read, without reading any directory.
func (t *Tree) find(rel string) *Node {
	n := t.Root
	if rel == "" || rel == "." {
		return n
	}
	for _, p := range strings.Split(rel, "/") {
		var next *Node
		for _, c := range n.Children {
			if c.Name == p {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

func (t *Tree) refresh(n *Node) {
	if !n.Dir || !n.loaded {
		return
	}
	old := map[string]*Node{}
	for _, c := range n.Children {
		old[c.Name] = c
	}
	t.load(n)
	for i, c := range n.Children {
		if o, ok := old[c.Name]; ok && o.Dir == c.Dir {
			o.Ignored = c.Ignored
			n.Children[i] = o
			if o.Expanded {
				t.refresh(o)
			} else {
				o.loaded = false // read again when expanded: its entries or their ignore state may differ
			}
		}
	}
}

func (t *Tree) load(n *Node) {
	n.loaded = true
	t.ig.LoadDir(n.Rel)
	ents, err := os.ReadDir(filepath.Join(t.root, filepath.FromSlash(n.Rel)))
	if err != nil {
		n.Children = nil
		return
	}
	kids := make([]*Node, 0, len(ents))
	for _, e := range ents {
		if Hidden(e.Name()) {
			continue
		}
		rel := e.Name()
		if n.Rel != "" {
			rel = n.Rel + "/" + e.Name()
		}
		dir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 { // 符号链接按目标类型显示，但不跟进遍历
			if fi, err := os.Stat(filepath.Join(t.root, rel)); err == nil {
				dir = fi.IsDir()
			}
		}
		kids = append(kids, &Node{Name: e.Name(), Rel: rel, Dir: dir, Depth: n.Depth + 1, Ignored: n.Ignored || t.ig.Match(rel, dir)})
	}
	// 和 VS Code 一样：目录在前，各自按名称不区分大小写排序
	slices.SortFunc(kids, func(a, b *Node) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	n.Children = kids
}

// Visible 按显示顺序返回所有可见节点（不含根）。
func (t *Tree) Visible(dst []*Node) []*Node {
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.Children {
			dst = append(dst, c)
			if c.Dir && c.Expanded {
				walk(c)
			}
		}
	}
	walk(t.Root)
	return dst
}

// Reveal 展开到 rel 所在的目录，返回该节点（找不到时返回 nil）。
func (t *Tree) Reveal(rel string) *Node {
	n := t.Root
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		t.Expand(n)
		var next *Node
		for _, c := range n.Children {
			if c.Name == p {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		if i == len(parts)-1 {
			return next
		}
		n = next
	}
	return nil
}
