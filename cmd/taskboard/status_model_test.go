package main

import "testing"

func TestParseStatusCommand_EditSupportsCompactAndSpaced(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		verb string
		arg  string
	}{
		{in: "e1", verb: "e", arg: "1"},
		{in: "edit1", verb: "edit", arg: "1"},
		{in: "e 1", verb: "e", arg: "1"},
		{in: "edit 1", verb: "edit", arg: "1"},
		{in: ":e1", verb: "e", arg: "1"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			verb, arg, err := parseStatusCommand(tc.in)
			if err != nil {
				t.Fatalf("parseStatusCommand(%q) error = %v", tc.in, err)
			}
			if verb != tc.verb || arg != tc.arg {
				t.Fatalf("parseStatusCommand(%q) = (%q, %q), want (%q, %q)", tc.in, verb, arg, tc.verb, tc.arg)
			}
		})
	}
}

func TestParseStatusCommand_CreateRequiresSpaceBeforeTitle(t *testing.T) {
	t.Parallel()

	verb, arg, err := parseStatusCommand(`cp`)
	if err != nil {
		t.Fatalf("parseStatusCommand(cp) unexpected error: %v", err)
	}
	if verb != "cp" || arg != "" {
		t.Fatalf("parseStatusCommand(cp) = (%q, %q), want (cp, \"\")", verb, arg)
	}

	verb, arg, err = parseStatusCommand(`cp "Task"`)
	if err != nil {
		t.Fatalf("parseStatusCommand(cp) unexpected error: %v", err)
	}
	if verb != "cp" || arg != "Task" {
		t.Fatalf("parseStatusCommand(cp) = (%q, %q), want (cp, Task)", verb, arg)
	}

	verb, arg, err = parseStatusCommand(`cc`)
	if err != nil {
		t.Fatalf("parseStatusCommand(cc) unexpected error: %v", err)
	}
	if verb != "cc" || arg != "" {
		t.Fatalf("parseStatusCommand(cc) = (%q, %q), want (cc, \"\")", verb, arg)
	}

	verb, arg, err = parseStatusCommand(`cc child title`)
	if err != nil {
		t.Fatalf("parseStatusCommand(cc child title) unexpected error: %v", err)
	}
	if verb != "cc" || arg != "child title" {
		t.Fatalf("parseStatusCommand(cc child title) = (%q, %q), want (cc, child title)", verb, arg)
	}

	verb, arg, err = parseStatusCommand(`s done`)
	if err != nil {
		t.Fatalf("parseStatusCommand(s done) unexpected error: %v", err)
	}
	if verb != "s" || arg != "done" {
		t.Fatalf("parseStatusCommand(s done) = (%q, %q), want (s, done)", verb, arg)
	}

	verb, arg, err = parseStatusCommand(`a`)
	if err != nil {
		t.Fatalf("parseStatusCommand(a) unexpected error: %v", err)
	}
	if verb != "a" || arg != "" {
		t.Fatalf("parseStatusCommand(a) = (%q, %q), want (a, \"\")", verb, arg)
	}

	verb, arg, err = parseStatusCommand(`d !`)
	if err != nil {
		t.Fatalf("parseStatusCommand(d !) unexpected error: %v", err)
	}
	if verb != "d" || arg != "!" {
		t.Fatalf("parseStatusCommand(d !) = (%q, %q), want (d, !)", verb, arg)
	}

	verb, arg, err = parseStatusCommand(`d!`)
	if err != nil {
		t.Fatalf("parseStatusCommand(d!) unexpected error: %v", err)
	}
	if verb != "d" || arg != "!" {
		t.Fatalf("parseStatusCommand(d!) = (%q, %q), want (d, !)", verb, arg)
	}
}

func TestParseTitleAndBody(t *testing.T) {
	t.Parallel()

	title, body, err := parseTitleAndBody("Title: Add cache layer\n- detail 1\n- detail 2\n")
	if err != nil {
		t.Fatalf("parseTitleAndBody unexpected error: %v", err)
	}
	if title != "Add cache layer" {
		t.Fatalf("title = %q, want %q", title, "Add cache layer")
	}
	if body != "- detail 1\n- detail 2" {
		t.Fatalf("body = %q, want %q", body, "- detail 1\n- detail 2")
	}

	title, body, err = parseTitleAndBody("Plain title\nnotes")
	if err != nil {
		t.Fatalf("parseTitleAndBody plain-title unexpected error: %v", err)
	}
	if title != "Plain title" || body != "notes" {
		t.Fatalf("plain-title parse = (%q, %q), want (%q, %q)", title, body, "Plain title", "notes")
	}

	if _, _, err := parseTitleAndBody("Title:\nbody"); err == nil {
		t.Fatalf("expected empty Title: value to fail")
	}
}

func TestListPathSuggestions(t *testing.T) {
	t.Parallel()

	files := []string{
		"README.md",
		"cmd/taskboard/main.go",
		"cmd/taskboard/status_model.go",
		"internal/app/service.go",
	}

	root := listPathSuggestions("", files)
	wantRoot := []string{"cmd/", "internal/", "README.md"}
	if len(root) != len(wantRoot) {
		t.Fatalf("root suggestions len=%d want=%d (%v)", len(root), len(wantRoot), root)
	}
	for i := range wantRoot {
		if root[i] != wantRoot[i] {
			t.Fatalf("root[%d]=%q want %q", i, root[i], wantRoot[i])
		}
	}

	cmdLevel := listPathSuggestions("cmd/", files)
	wantCmd := []string{"cmd/taskboard/"}
	if len(cmdLevel) != len(wantCmd) || cmdLevel[0] != wantCmd[0] {
		t.Fatalf("cmd level suggestions = %v want %v", cmdLevel, wantCmd)
	}

	prefix := listPathSuggestions("cmd/t", files)
	if len(prefix) != 1 || prefix[0] != "cmd/taskboard/" {
		t.Fatalf("prefix suggestions = %v want [cmd/taskboard/]", prefix)
	}
}

func TestResolveKeymapMode(t *testing.T) {
	t.Parallel()

	if got := resolveKeymapMode("vim"); got != keymapVim {
		t.Fatalf("resolveKeymapMode(vim)=%q want %q", got, keymapVim)
	}
	if got := resolveKeymapMode("VIM"); got != keymapVim {
		t.Fatalf("resolveKeymapMode(VIM)=%q want %q", got, keymapVim)
	}
	if got := resolveKeymapMode(""); got != keymapDefault {
		t.Fatalf("resolveKeymapMode(empty)=%q want %q", got, keymapDefault)
	}
	if got := resolveKeymapMode("unknown"); got != keymapDefault {
		t.Fatalf("resolveKeymapMode(unknown)=%q want %q", got, keymapDefault)
	}
}

func TestParseStateCommandArg(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "done", want: "Complete"},
		{in: "ready", want: "Design"},
		{in: "rfi", want: "Design"},
		{in: "in-progress", want: "In Progress"},
		{in: "context", want: "Scoping"},
	}

	for _, tc := range cases {
		got, err := parseStateCommandArg(tc.in)
		if err != nil {
			t.Fatalf("parseStateCommandArg(%q) error: %v", tc.in, err)
		}
		if string(got) != tc.want {
			t.Fatalf("parseStateCommandArg(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseStateCommandForCompletion(t *testing.T) {
	t.Parallel()

	verb, prefix, ok := parseStateCommandForCompletion(":s ")
	if !ok || verb != "s" || prefix != "" {
		t.Fatalf("parseStateCommandForCompletion(:s ) = (%q,%q,%v)", verb, prefix, ok)
	}

	verb, prefix, ok = parseStateCommandForCompletion("state rea")
	if !ok || verb != "state" || prefix != "rea" {
		t.Fatalf("parseStateCommandForCompletion(state rea) = (%q,%q,%v)", verb, prefix, ok)
	}

	if _, _, ok = parseStateCommandForCompletion("cp test"); ok {
		t.Fatalf("expected cp to not parse as state completion command")
	}
}
