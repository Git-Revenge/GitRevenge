package semdiff

import (
	"context"
	"strings"
	"testing"
)

func TestImportsAddedInOneSpotMergeAsASet(t *testing.T) {
	base := "\t\"fmt\"\n\t\"os\"\n"
	// Both add after "fmt": as tokens, that's two additions in one spot.
	ours := "\t\"fmt\"\n\t\"log\"\n\t\"os\"\n"
	theirs := "\t\"fmt\"\n\t\"io\"\n\t\"os\"\n"
	s := MergePlace(context.Background(), "go", base, ours, theirs)
	if !s.OK {
		t.Fatalf("not combined: %s", s.Reason)
	}
	// Both lists were sorted, so the result is too.
	if want := "\t\"fmt\"\n\t\"io\"\n\t\"log\"\n\t\"os\"\n"; s.Text != want {
		t.Fatalf("text %q, want %q", s.Text, want)
	}
	if s.Reason != "imports merge as a set: ours added `\"log\"`; theirs added `\"io\"`" {
		t.Errorf("reason %q", s.Reason)
	}
	var got []string
	for _, o := range s.Origins {
		got = append(got, o.Side+":"+s.Text[o.From:o.To])
	}
	if strings.Join(got, "|") != `theirs:"io"|ours:"log"` {
		t.Errorf("origins %q", got)
	}
}

func TestAnUnsortedListKeepsOursOrderAndAddsTheirsAfter(t *testing.T) {
	base := "import b from './b'\nimport a from './a'\n"
	ours := "import b from './b'\nimport c from './c'\nimport a from './a'\n"
	theirs := "import b from './b'\nimport d from './d'\nimport a from './a'\n"
	s := MergePlace(context.Background(), "javascript", base, ours, theirs)
	if want := "import b from './b'\nimport c from './c'\nimport a from './a'\nimport d from './d'\n"; !s.OK || s.Text != want {
		t.Fatalf("%+v", s)
	}
}

func TestAnImportOneSideRemovedStaysRemoved(t *testing.T) {
	base := "import os\nimport sys\n"
	ours := "import os\nimport re\nimport sys\n"
	theirs := "import json\nimport sys\n"
	s := MergePlace(context.Background(), "python", base, ours, theirs)
	if want := "import json\nimport re\nimport sys\n"; !s.OK || s.Text != want {
		t.Fatalf("%+v", s)
	}
}

func TestOneModuleImportedTwoWaysStillClashes(t *testing.T) {
	base := "\t\"fmt\"\n"
	ours := "\t\"fmt\"\n\tlog \"github.com/a/log\"\n"
	theirs := "\t\"fmt\"\n\tl \"github.com/a/log\"\n"
	if s := MergePlace(context.Background(), "go", base, ours, theirs); s.OK {
		t.Fatalf("combined %q", s.Text)
	}
}

func TestOnlyPlacesOfImportsMergeAsSets(t *testing.T) {
	// Two statements added in one spot are a real conflict: their order matters.
	if s := MergePlace(context.Background(), "go", "\ta()\n", "\ta()\n\tb()\n", "\ta()\n\tc()\n"); s.OK {
		t.Fatalf("combined %q", s.Text)
	}
	// So are imports in a language with no rule for them.
	if _, ok := mergeSet("lua", "", "require 'a'\n", "require 'b'\n"); ok {
		t.Fatal("merged lua requires")
	}
}

func TestIncludesAndUsingsMergeAsSets(t *testing.T) {
	cases := []struct{ id, base, ours, theirs, want string }{
		{"c", "#include <stdio.h>\n", "#include <stdio.h>\n#include <string.h>\n", "#include <stdio.h>\n#include <stdlib.h>\n", "#include <stdio.h>\n#include <stdlib.h>\n#include <string.h>\n"},
		{"csharp", "using System;\n", "using System;\nusing System.IO;\n", "using System;\nusing System.Linq;\n", "using System;\nusing System.IO;\nusing System.Linq;\n"},
		{"java", "import a.B;\n", "import a.B;\nimport a.C;\n", "import a.B;\nimport a.D;\n", "import a.B;\nimport a.C;\nimport a.D;\n"},
		{"rust", "use std::fs;\n", "use std::fs;\nuse std::io;\n", "use std::fs;\nuse std::env;\n", "use std::fs;\nuse std::io;\nuse std::env;\n"},
	}
	for _, c := range cases {
		s := MergePlace(context.Background(), c.id, c.base, c.ours, c.theirs)
		if !s.OK || s.Text != c.want {
			t.Errorf("%s: %+v", c.id, s)
		}
	}
}
