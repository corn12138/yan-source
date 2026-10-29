package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/review"
)

// State 是编辑器自己记下的东西（照着 Mayoi 的 state.json）：菜单里选的主题、外观、字号、自动保存，
// 每个文件夹上次开着的标签，最近打开的文件夹。它和手写的 config.toml 分开放，免得改写用户的注释。
// 菜单里选过的设置优先于 config.toml。
type State struct {
	Theme *string `json:"theme,omitempty"`
	// Language is the interface language chosen in Settings: "system", "zh-Hans" or "en". It
	// applies from the next launch and wins over [ui] language; an explicit "system" is a choice
	// too, unlike an absent one.
	Language     *string  `json:"language,omitempty"`
	ThemeEffects *bool    `json:"themeEffects,omitempty"`
	Appearance   *string  `json:"appearance,omitempty"`
	FontSize     *float64 `json:"fontSize,omitempty"`
	// FontFamily and LineHeight are the code font and line height chosen in the window.
	FontFamily   *string  `json:"fontFamily,omitempty"`
	LineHeight   *float64 `json:"lineHeight,omitempty"`
	AutoSave     *bool    `json:"autoSave,omitempty"`
	Vim          *bool    `json:"vim,omitempty"`
	SidebarRight *bool    `json:"sidebarRight,omitempty"`
	AllBranches  bool     `json:"allBranches,omitempty"`
	// RepositoryPanel is Source Control's repository panel open; RepositoryRemotes shows its
	// remotes rather than its branches.
	RepositoryPanel   bool `json:"repositoryPanel,omitempty"`
	RepositoryRemotes bool `json:"repositoryRemotes,omitempty"`
	// MergeManual turns off applying the changes no conflict holds as a merge view opens.
	MergeManual bool `json:"mergeManual,omitempty"`
	// SCMFolded are Source Control's sections folded: "merge", "staged", "changes",
	// "untracked" and "commits".
	CommitFilesOpen  bool     `json:"commitFilesOpen,omitempty"`
	CommitFilesWidth float64  `json:"commitFilesWidth,omitempty"`
	SCMFolded        []string `json:"scmFolded,omitempty"`
	// UpdateChecksOff turns off the window's hourly checks for a new version.
	UpdateChecksOff bool `json:"updateChecksOff,omitempty"`
	// GuideDone are the lessons of the feature guide finished, by id.
	GuideDone []string `json:"guideDone,omitempty"`
	// GuideLesson and GuideStep are the lesson left unfinished and its step, to resume;
	// GuideHome is the folder a lesson's practice folder replaced, to go back to.
	GuideLesson string `json:"guideLesson,omitempty"`
	GuideStep   int    `json:"guideStep,omitempty"`
	GuideHome   string `json:"guideHome,omitempty"`
	// GuideOffered is set once the window has offered the guide on a first launch and the user
	// took or declined it.
	GuideOffered bool  `json:"guideOffered,omitempty"`
	WordWrap     *bool `json:"wordWrap,omitempty"`
	// DiffWhitespace is the diffs' whitespace filter chosen from the menus, as [diff] whitespace.
	DiffWhitespace *string `json:"diffWhitespace,omitempty"`
	// DiffHighlight, DiffContext and DiffLayout are the diffs' view chosen from the menus, as the
	// [diff] keys of the same names.
	DiffHighlight *string `json:"diffHighlight,omitempty"`
	DiffContext   *int    `json:"diffContext,omitempty"`
	DiffLayout    *string `json:"diffLayout,omitempty"`
	// DiffSyncScroll and DiffAlignedRows are the split diff's choices from the menus.
	DiffSyncScroll  *bool `json:"diffSyncScroll,omitempty"`
	DiffAlignedRows *bool `json:"diffAlignedRows,omitempty"`
	// PanelPosition is the terminal panel's place chosen from the menus: "", "left" or "right".
	PanelPosition    *string `json:"panelPosition,omitempty"`
	TerminalLocation *string `json:"terminalLocation,omitempty"`
	// ProjectLooks are the looks chosen for projects' squares on the rail, by folder.
	ProjectLooks map[string]ProjectLook `json:"projectLooks,omitempty"`
	// ProjectTabs 是窗口版把所有文件夹放在一个窗口里、左边一列方块切换（照着 Mayoi 的 projectTabs）；
	// Projects 是上次关掉时那个窗口里的文件夹，下次从访达、Dock 打开时都回来。
	ProjectTabs *bool              `json:"projectTabs,omitempty"`
	Projects    []string           `json:"projects,omitempty"`
	Sessions    map[string]Session `json:"sessions,omitempty"` // 文件夹 → 上次的标签
	Recent      []string           `json:"recent,omitempty"`   // 最近打开的文件夹，最近的在前

	path string
}

// Session 是一个文件夹上次关掉时的样子：各列的标签、哪一列有焦点、侧边栏。
type Session struct {
	Review  []review.Note   `json:"review,omitempty"`
	Columns []SessionColumn `json:"columns"`
	Active  int             `json:"active"`
	Sidebar bool            `json:"sidebar"`
}

// SessionColumn 是一列的标签。
type SessionColumn struct {
	Tabs   []SessionTab `json:"tabs"`
	Active int          `json:"active"`
	Weight float64      `json:"weight,omitempty"` // 占编辑区宽度的份数；没有时是 1
	Locked bool         `json:"locked,omitempty"` // 锁定的列：空的也恢复
}

// SessionTab 是一个标签：文件、是不是预览、光标和滚动位置。
type SessionTab struct {
	Repository string `json:"repository,omitempty"`

	Revision      string `json:"revision,omitempty"`
	CompareFrom   string `json:"compareFrom,omitempty"`
	Path          string `json:"path"`
	Terminal      bool   `json:"terminal,omitempty"` // Path 是工作目录；恢复时开新 shell，不重放历史命令
	Wrap          int8   `json:"wrap,omitempty"`
	Preview       bool   `json:"preview,omitempty"`
	RenderPreview bool   `json:"render_preview,omitempty"`
	PreviewLine   int    `json:"preview_line,omitempty"`
	Anchor        int    `json:"anchor,omitempty"`
	Head          int    `json:"head,omitempty"`
	ScrollY       int    `json:"scrollY,omitempty"`
}

// StatePath 返回状态文件的路径（在用户配置文件旁边）。
func StatePath() string { return filepath.Join(filepath.Dir(UserPath()), "state.json") }

// LoadState 读状态文件；没有或读不懂时返回空的状态（照样能用、能存）。
func LoadState(path string) *State {
	s := &State{path: path}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, s)
	}
	s.path = path
	return s
}

// Apply 用菜单里选过的设置覆盖 c。
func (s *State) Apply(c *Config) {
	if s == nil {
		return
	}
	if s.ThemeEffects != nil {
		c.ThemeEffects = *s.ThemeEffects
	}
	if s.Theme != nil {
		c.Theme = *s.Theme
	}
	if s.Appearance != nil {
		c.Appearance = *s.Appearance
	}
	if s.FontSize != nil && *s.FontSize >= 9 && *s.FontSize <= 28 {
		c.FontSize = *s.FontSize
	}
	if s.FontFamily != nil && strings.TrimSpace(*s.FontFamily) != "" {
		c.FontFamily = *s.FontFamily
	}
	if s.LineHeight != nil && *s.LineHeight >= 1 && *s.LineHeight <= 3 {
		c.LineHeight = *s.LineHeight
	}
	if s.AutoSave != nil {
		v := *s.AutoSave
		c.AutoSave = &v
	}
	if s.Vim != nil {
		c.Vim = *s.Vim
	}
	if s.SidebarRight != nil {
		c.SidebarRight = *s.SidebarRight
	}
	if s.TerminalLocation != nil {
		if location, ok := terminalLocation(*s.TerminalLocation); ok {
			c.TerminalLocation = location
		}
	}
	if s.PanelPosition != nil {
		c.PanelPosition = *s.PanelPosition
	}
	if s.WordWrap != nil {
		c.WordWrap = *s.WordWrap
	}
	if s.DiffHighlight != nil {
		if h, ok := DiffHighlightMode(*s.DiffHighlight); ok {
			c.DiffHighlight = h
		}
	}
	if s.DiffContext != nil && *s.DiffContext >= 0 && *s.DiffContext <= MaxDiffContext {
		c.DiffContext = *s.DiffContext
	}
	if s.DiffSyncScroll != nil {
		c.DiffSyncScroll = *s.DiffSyncScroll
	}
	if s.DiffAlignedRows != nil {
		c.DiffAlignedRows = *s.DiffAlignedRows
	}
	if s.DiffLayout != nil {
		if l, ok := DiffLayoutMode(*s.DiffLayout); ok {
			c.DiffLayout = l
		}
	}
	if s.DiffWhitespace != nil {
		if w, ok := DiffWhitespaceMode(*s.DiffWhitespace); ok {
			c.DiffWhitespace = w
		}
	}
}

// LanguagePreference is the interface language the settings and configuration choose: a valid
// choice saved in Settings, else a valid [ui] language in the user configuration (c), else
// System. An invalid saved choice is reported and skipped.
func (s *State) LanguagePreference(c *Config) (i18n.Preference, []string) {
	var warns []string
	if s != nil && s.Language != nil {
		if p, ok := i18n.ParsePreference(*s.Language); ok {
			return p, nil
		}
		warns = append(warns, i18n.F("state.json: unknown interface language %q, ignored", *s.Language))
	}
	if c != nil {
		if p, ok := i18n.ParsePreference(c.Language); ok {
			return p, warns
		}
	}
	return i18n.System, warns
}

// SetLanguage saves the interface language chosen in Settings.
func (s *State) SetLanguage(p i18n.Preference) error {
	v := string(p)
	previous := s.Language
	s.Language = &v
	if err := s.Save(); err != nil {
		// All windows share this state. A failed write must not appear as a saved choice,
		// or make retrying the same choice skip persistence.
		s.Language = previous
		return err
	}
	return nil
}

// AddRecent 把文件夹放到最近打开的最前面（最多 10 个）。
func (s *State) AddRecent(dir string) {
	out := []string{dir}
	for _, d := range s.Recent {
		if d != dir && len(out) < 10 {
			out = append(out, d)
		}
	}
	s.Recent = out
}

// Save 原子地写回状态文件。
func (s *State) Save() error {
	if s == nil || s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Sessions include source excerpts and review text. Create a new owner-only
	// file so a pre-existing temporary path cannot redirect the write or retain
	// broader permissions; replacing the state also protects older state files.
	f, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ProjectLook is how a project's square on the rail looks when chosen by hand: Mark is up to two
// characters or an emoji in place of its initials, Color a lane from 1 (0 picks one by folder).
type ProjectLook struct {
	Mark  string `json:"mark,omitempty"`
	Color int    `json:"color,omitempty"`
	// Icon is a picture chosen for the square, by its file name in the icons folder beside the
	// configuration: a copy, so the square keeps it when the original moves.
	Icon string `json:"icon,omitempty"`
}
