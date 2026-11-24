package git

import (
	"context"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// Each case asks git itself where `git push` would go (a dry run on the checked-out branch)
// and wants PlanPush to agree, or to refuse or ask where git refuses.
func TestPlanPushAgreesWithGit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		config [][2]string
		// track is the branch's upstream on origin, "" for none.
		track string
		want  string // "remote ref", "refuse" or "ask"
		// appDefault marks a case git config says nothing about, where the app is
		// friendlier than git and git itself refuses.
		appDefault bool
	}{
		{"tracks the same name", nil, "feature", "origin feature", false},
		{"unset pushes to a differently named upstream", nil, "topic", "origin topic", true},
		{"simple refuses a different name", [][2]string{{"push.default", "simple"}}, "topic", "refuse", false},
		{"upstream pushes to the tracked name", [][2]string{{"push.default", "upstream"}}, "topic", "origin topic", false},
		{"current pushes the same name", [][2]string{{"push.default", "current"}}, "topic", "origin feature", false},
		{"pushDefault sends it to the fork", [][2]string{{"remote.pushDefault", "fork"}}, "topic", "fork feature", false},
		{"pushRemote beats pushDefault", [][2]string{{"remote.pushDefault", "fork"}, {"branch.feature.pushRemote", "origin"}}, "feature", "origin feature", false},
		{"upstream refuses another remote", [][2]string{{"push.default", "upstream"}, {"remote.pushDefault", "fork"}}, "topic", "refuse", false},
		{"nothing pushes nothing", [][2]string{{"push.default", "nothing"}}, "feature", "refuse", false},
		{"a first push with pushDefault", [][2]string{{"remote.pushDefault", "fork"}}, "", "fork feature", false},
		{"a first push with autoSetupRemote", [][2]string{{"push.autoSetupRemote", "true"}}, "", "origin feature", false},
		{"a first push with nothing set publishes", nil, "", "origin feature", true},
		{"a first push with autoSetupRemote false asks", [][2]string{{"push.autoSetupRemote", "false"}}, "", "ask", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			up, fork := gittest.NewBare(t), gittest.NewBare(t)
			fx := gittest.New(t)
			fx.Commit("one")
			fx.Git("remote", "add", "origin", up.Dir)
			fx.Git("remote", "add", "fork", fork.Dir)
			fx.Git("push", "-q", "origin", "main")
			fx.Git("switch", "-q", "-c", "feature")
			fx.Commit("two")
			if tc.track != "" {
				fx.Git("push", "-q", "origin", "feature:"+tc.track)
				fx.Git("branch", "-q", "--set-upstream-to=origin/"+tc.track)
			}
			for _, kv := range tc.config {
				fx.Git("config", kv[0], kv[1])
			}
			repo := OpenRepo(newTestRunner(t), fx.Dir)
			plan, err := repo.PlanPush(context.Background(), "feature")
			if err != nil {
				t.Fatal(err)
			}
			got := plan.Remote + " " + plan.Ref
			switch {
			case plan.Refusal != "":
				got = "refuse"
			case plan.Ask:
				got = "ask"
			}
			if got != tc.want {
				t.Fatalf("plan = %+v, want %s", plan, tc.want)
			}

			// What git itself does with a plain `git push`.
			out, gerr := repo.Read(context.Background(), "push", "--dry-run", "--porcelain")
			gitSays := "refuse"
			if gerr == nil {
				remote, ref := "", ""
				for line := range strings.SplitSeq(string(out), "\n") {
					if u, ok := strings.CutPrefix(line, "To "); ok {
						if SameRemoteURL(u, up.Dir) {
							remote = "origin"
						} else if SameRemoteURL(u, fork.Dir) {
							remote = "fork"
						}
					}
					if parts := strings.Split(line, "\t"); len(parts) == 3 {
						_, dst, _ := strings.Cut(parts[1], ":")
						ref = strings.TrimPrefix(dst, "refs/heads/")
					}
				}
				gitSays = remote + " " + ref
			}
			// Where git config says nothing, git refuses and the app goes ahead; where git
			// refuses a first push, the app asks instead.
			if tc.appDefault || tc.want == "ask" {
				if gitSays != "refuse" {
					t.Fatalf("git pushed a first push with nothing set: %s", gitSays)
				}
				return
			}
			if gitSays != got {
				t.Fatalf("git says %q, the plan %q (%+v)", gitSays, got, plan)
			}
		})
	}
}
