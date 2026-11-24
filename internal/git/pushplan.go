package git

import (
	"context"
	"strings"
)

// Where a push goes (#927), worked out by git's own rules, so the app's Push goes where
// `git push` would and the Pushing settings mean what they say:
//
//   - the remote is branch.<name>.pushRemote, else remote.pushDefault, else the remote the
//     branch tracks;
//   - the branch's name there follows push.default: current and simple use the same name
//     (simple refuses when the branch tracks a differently named branch on that remote),
//     upstream uses the tracked branch, and nothing pushes nothing;
//   - a branch with nowhere to go yet is set up to track the branch it creates when
//     push.autoSetupRemote is on; when it's false, the app asks.
//
// Where git config says nothing, the app keeps its own defaults, friendlier than git's (the
// user's ruling for #927): a first push publishes to the only remote, or origin, and tracks
// it; and a branch pushes to the branch it tracks on its own remote, whatever that's named.
// Anything the user writes is followed exactly as git reads it.

// PushPlan is where pushing a branch would go.
type PushPlan struct {
	Branch string `json:"branch"`
	// Remote and Ref are the destination: "fork" and "feature". Empty when the push needs
	// asking or is refused.
	Remote string `json:"remote"`
	Ref    string `json:"ref"`
	// SetUpstream says the push also makes the branch track what it creates.
	SetUpstream bool `json:"setUpstream"`
	// Why names the setting that chose the remote: "branch.feature.pushRemote",
	// "remote.pushDefault", "its upstream", or "the only remote".
	Why string `json:"why"`
	// AppDefaults names what the app decided where git config says nothing: "first push"
	// (published and tracked) and "tracked name" (pushed to a differently named upstream).
	AppDefaults []string `json:"appDefaults"`
	// Ask means no rule says where a first push goes: the user is asked, among Remotes.
	Ask     bool     `json:"ask"`
	Remotes []string `json:"remotes"`
	// Refusal says why git wouldn't push, in words, such as push.default being nothing.
	Refusal string `json:"refusal,omitempty"`
}

// pushConfig reads what push planning needs in one git call: the push keys, lower case, and
// the branch's own keys without their "branch.<name>." prefix. A key written bare is true.
func (r *Repo) pushConfig(ctx context.Context, branch string) map[string]string {
	cfg := map[string]string{}
	prefix := "branch." + branch + "."
	for _, kv := range r.configMatching(ctx, `^(remote\.pushdefault|push\.(default|autosetupremote)|branch\..*\.(pushremote|remote|merge))$`) {
		value := kv.value
		if kv.bare {
			value = "true"
		}
		// The branch's name keeps its case in git's keys; the rest is lower case.
		if rest, ok := strings.CutPrefix(kv.key, prefix); ok {
			cfg[strings.ToLower(rest)] = value
		} else if !strings.HasPrefix(strings.ToLower(kv.key), "branch.") {
			cfg[strings.ToLower(kv.key)] = value
		}
	}
	return cfg
}

// PlanPush works out where pushing a branch goes, without pushing.
func (r *Repo) PlanPush(ctx context.Context, branch string) (PushPlan, error) {
	plan := PushPlan{Branch: branch, Remotes: []string{}, AppDefaults: []string{}}
	if err := CheckRevs(branch); err != nil {
		return plan, err
	}
	remotes, err := r.Remotes(ctx)
	if err != nil {
		return plan, err
	}
	for _, rm := range remotes {
		plan.Remotes = append(plan.Remotes, rm.Name)
	}
	cfg := r.pushConfig(ctx, branch)
	upRemote, upRef := cfg["remote"], strings.TrimPrefix(cfg["merge"], "refs/heads/")
	if upRemote == "." {
		// It tracks a local branch, which isn't somewhere to push.
		upRemote, upRef = "", ""
	}
	mode := strings.ToLower(cfg["push.default"])
	if mode == "nothing" {
		plan.Refusal = "push.default is nothing, so git pushes nothing unless told where."
		return plan, nil
	}

	switch {
	case cfg["pushremote"] != "":
		plan.Remote, plan.Why = cfg["pushremote"], "branch."+branch+".pushRemote"
	case cfg["remote.pushdefault"] != "":
		plan.Remote, plan.Why = cfg["remote.pushdefault"], "remote.pushDefault"
	case upRemote != "":
		plan.Remote, plan.Why = upRemote, "its upstream"
	}
	// Unset, a first push publishes and tracks: the app's default. Written, it's git's.
	autoSet := cfg["push.autosetupremote"] != ""
	auto := !autoSet || configBool(configValue{value: cfg["push.autosetupremote"]})

	if plan.Remote == "" && len(remotes) == 0 {
		plan.Refusal = "This repository has no remotes. Add one first."
		return plan, nil
	}
	if plan.Remote == "" {
		// A first push with nowhere set: git pushes to the only remote, or origin, and
		// tracks what it makes when push.autoSetupRemote says so; otherwise it's asked.
		if !auto {
			plan.Ask = true
			return plan, nil
		}
		switch {
		case len(remotes) == 1:
			plan.Remote, plan.Why = remotes[0].Name, "the only remote"
		case contains(plan.Remotes, "origin"):
			plan.Remote, plan.Why = "origin", "origin"
		default:
			plan.Ask = true
			return plan, nil
		}
		plan.Ref, plan.SetUpstream = branch, true
		if !autoSet {
			plan.AppDefaults = append(plan.AppDefaults, "first push")
		}
		return plan, nil
	}

	triangular := plan.Remote != upRemote
	switch mode {
	case "upstream":
		if triangular || upRef == "" {
			plan.Refusal = "push.default is upstream, which pushes only to the branch " + branch + " tracks, and it doesn't track one on " + plan.Remote + "."
			plan.Remote = ""
			return plan, nil
		}
		plan.Ref = upRef
	case "simple":
		if !triangular && upRef != "" && upRef != branch {
			plan.Refusal = branch + " tracks " + upRemote + "/" + upRef + ", a different name, and push.default is simple, which pushes only to the same name. Choose upstream for push.default in Settings, Pushing, to push there."
			plan.Remote = ""
			return plan, nil
		}
		plan.Ref = branch
	case "":
		// Unset: the branch it tracks on its own remote, whatever that's named, else the
		// same name. Git's simple would refuse a different name.
		plan.Ref = branch
		if !triangular && upRef != "" {
			plan.Ref = upRef
			if upRef != branch {
				plan.AppDefaults = append(plan.AppDefaults, "tracked name")
			}
		}
	default: // current, matching
		plan.Ref = branch
	}
	// No upstream yet: it's set up when push.autoSetupRemote is on, or unset.
	plan.SetUpstream = upRemote == "" && auto
	if plan.SetUpstream && !autoSet {
		plan.AppDefaults = append(plan.AppDefaults, "first push")
	}
	return plan, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
