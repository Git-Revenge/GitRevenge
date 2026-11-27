package git

import (
	"context"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Storage is how a repository keeps its objects, from git count-objects (#937), with git's own
// thresholds for tidying them.
type Storage struct {
	// Loose objects are stored one file each; packs hold the rest.
	Loose      int   `json:"loose"`
	LooseBytes int64 `json:"looseBytes"`
	Packs      int   `json:"packs"`
	PackBytes  int64 `json:"packBytes"`
	// Garbage is files in the object folder that are neither: a pack left half written, say.
	Garbage      int   `json:"garbage"`
	GarbageBytes int64 `json:"garbageBytes"`
	// PrunePackable is loose objects that a pack has too.
	PrunePackable int `json:"prunePackable"`
	// GCAuto and AutoPackLimit are gc.auto and gc.autoPackLimit, git's defaults when unset:
	// past either, git gc --auto tidies up. 0 turns that one off.
	GCAuto        int `json:"gcAuto"`
	AutoPackLimit int `json:"autoPackLimit"`
	// Registered says git maintenance runs this repository on its schedule: maintenance.repo
	// names it.
	Registered bool `json:"registered"`
}

// Bytes is everything the objects take on disk.
func (s Storage) Bytes() int64 { return s.LooseBytes + s.PackBytes + s.GarbageBytes }

// Due says git's own thresholds are passed: what git gc --auto would act on.
func (s Storage) Due() bool {
	return (s.GCAuto > 0 && s.Loose > s.GCAuto) || (s.AutoPackLimit > 0 && s.Packs > s.AutoPackLimit)
}

// git's defaults for gc.auto and gc.autoPackLimit.
const (
	defaultGCAuto        = 6700
	defaultAutoPackLimit = 50
)

// Storage reads how the repository keeps its objects. On the Linux kernel it takes a quarter
// of a second: count-objects counts the loose files and adds up the packs.
func (r *Repo) Storage(ctx context.Context) (Storage, error) {
	s := Storage{GCAuto: defaultGCAuto, AutoPackLimit: defaultAutoPackLimit}
	out, err := r.ReadString(ctx, "count-objects", "-v")
	if err != nil {
		return s, err
	}
	for line := range strings.SplitSeq(out, "\n") {
		key, val, _ := strings.Cut(strings.TrimSpace(line), ": ")
		n, _ := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		switch key {
		case "count":
			s.Loose = int(n)
		case "size":
			s.LooseBytes = n * 1024
		case "packs":
			s.Packs = int(n)
		case "size-pack":
			s.PackBytes = n * 1024
		case "garbage":
			s.Garbage = int(n)
		case "size-garbage":
			s.GarbageBytes = n * 1024
		case "prune-packable":
			s.PrunePackable = int(n)
		}
	}
	// One read for the thresholds and the schedule: keys come back in lower case.
	cfg, _ := r.ReadString(ctx, "config", "--get-regexp", `^(gc\.auto|gc\.autopacklimit|maintenance\.repo)$`)
	var repos []string
	for line := range strings.SplitSeq(cfg, "\n") {
		key, val, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch key {
		case "gc.auto":
			if n, err := strconv.Atoi(val); err == nil {
				s.GCAuto = n
			}
		case "gc.autopacklimit":
			if n, err := strconv.Atoi(val); err == nil {
				s.AutoPackLimit = n
			}
		case "maintenance.repo":
			repos = append(repos, val)
		}
	}
	s.Registered = r.namedIn(ctx, repos)
	return s, nil
}

// namedIn says one of paths is this repository, as git maintenance register writes it: its
// working tree's top, or for a bare repository its git folder.
func (r *Repo) namedIn(ctx context.Context, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	mine := []string{r.dir}
	if top, err := r.ReadString(ctx, "rev-parse", "--show-toplevel"); err == nil && top != "" {
		mine = append(mine, top)
	} else if dir, err := r.ReadString(ctx, "rev-parse", "--absolute-git-dir"); err == nil {
		mine = append(mine, dir)
	}
	for _, p := range paths {
		for _, m := range mine {
			if samePath(p, m) {
				return true
			}
		}
	}
	return false
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(filepath.FromSlash(a)), filepath.Clean(filepath.FromSlash(b))
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// RunMaintenance runs git maintenance run: the tasks the repository's config enables, or gc
// when it enables none. Some tasks fetch (prefetch), so it takes the network options. git
// prints no progress when it isn't writing to a terminal.
func (r *Repo) RunMaintenance(ctx context.Context, o NetOptions) error {
	c := o.cmd("maintenance", "run", "--no-quiet")
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}

// ScheduleMaintenance puts the repository on git maintenance's schedule (git maintenance start:
// it registers the repository in the global config, and makes sure the system's scheduler runs
// git maintenance hourly, daily and weekly) or takes it off (git maintenance unregister, which
// leaves the schedule for any other repository on it).
func (r *Repo) ScheduleMaintenance(ctx context.Context, on bool) error {
	args := []string{"maintenance", "unregister", "--force"}
	if on {
		args = []string{"maintenance", "start"}
	}
	_, err := r.runner.Run(ctx, Cmd{Dir: r.dir, Args: args})
	return err
}
