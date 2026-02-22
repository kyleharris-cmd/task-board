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

	if _, _, err := parseStatusCommand(`cp"Task"`); err == nil {
		t.Fatalf("expected cp without separating space to fail")
	}
	if _, _, err := parseStatusCommand(`cc"Task"`); err == nil {
		t.Fatalf("expected cc without separating space to fail")
	}

	verb, arg, err := parseStatusCommand(`cp "Task"`)
	if err != nil {
		t.Fatalf("parseStatusCommand(cp) unexpected error: %v", err)
	}
	if verb != "cp" || arg != "Task" {
		t.Fatalf("parseStatusCommand(cp) = (%q, %q), want (cp, Task)", verb, arg)
	}
}
