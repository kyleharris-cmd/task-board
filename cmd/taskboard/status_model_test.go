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
