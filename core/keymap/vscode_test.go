package keymap

import "testing"

func TestParseVSCode(t *testing.T) {
	bs, warns, err := ParseVSCode([]byte(`// 从 VS Code 粘过来的
[
	{ "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" },
	{ "key": "ctrl+shift+alt+x", "command": "editor.action.deleteLines", "when": "editorTextFocus && !editorReadonly" },
	{ "key": "cmd+d", "command": "-editor.action.addSelectionToNextFindMatch" },
	{ "command": "-workbench.action.quickOpen" },
	{ "key": "meta+p", "command": "workbench.action.quickOpen", }, /* Linux 的叫法 */
	{ "key": "numpad_add", "command": "x" },
	{ "key": "ctrl+a", "command": "y", "when": "a &&" },
	{ "command": "z" },
]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 5 || len(warns) != 3 {
		t.Fatalf("bindings %+v, warnings %q", bs, warns)
	}
	if bs[0].Key != "cmd+k cmd+s" || bs[0].When != nil {
		t.Errorf("chord %+v", bs[0])
	}
	if w := bs[1].When; w == nil || !w.Eval(func(n string) bool { return n == "editorTextFocus" }) || len(w.Names()) != 2 {
		t.Errorf("when %+v", bs[1])
	}
	if !bs[2].Removal() || !bs[3].Removal() || bs[3].Key != "" {
		t.Errorf("removals %+v %+v", bs[2], bs[3])
	}
	if cs, _ := Parse(bs[4].Key, ModCtrl); len(cs) != 1 || cs[0].Mods != ModSuper {
		t.Errorf("meta = %+v", cs)
	}
	if _, _, err := ParseVSCode([]byte(`{"key": 1}`)); err == nil {
		t.Error("not a list")
	}
}
