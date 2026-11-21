package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The graph, the commit panel and the lists of commits show names as the mailmap git reads
// gives them: the .mailmap in the work tree, and mailmap.file or mailmap.blob when the
// user has set them. Nothing is pinned, unlike Insights and the Map's Authors mode (see
// authorPinned), so what the app shows is what `git log` shows in a terminal.

// Mailmap returns each identity as git's mailmap resolves it, in the same order. It's for
// names that don't come through git log, such as the author of a commit read as an object.
// An identity with no email can't be looked up and comes back as it is, and so does every
// identity when git can't be asked.
func (r *Repo) Mailmap(ctx context.Context, ids []Identity) ([]Identity, error) {
	return r.checkMailmap(ctx, nil, ids)
}

// checkMailmap asks git check-mailmap about ids, with pre before the command: the -c
// settings that say which mailmap to read, or none for git's own.
func (r *Repo) checkMailmap(ctx context.Context, pre []string, ids []Identity) ([]Identity, error) {
	out := make([]Identity, len(ids))
	copy(out, ids)
	var in strings.Builder
	var asked []int
	for i, id := range ids {
		// A line break would be read as another contact.
		if id.Email == "" || strings.ContainsAny(id.Name+id.Email, "\r\n<>") {
			continue
		}
		fmt.Fprintf(&in, "%s <%s>\n", id.Name, id.Email)
		asked = append(asked, i)
	}
	if len(asked) == 0 {
		return out, nil
	}
	args := append(append([]string{}, pre...), "check-mailmap", "--stdin")
	var lines []string
	err := r.streamCmd(ctx, Cmd{Args: args, Stdin: strings.NewReader(in.String())}, func(rd io.Reader) error {
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		return sc.Err()
	})
	if err != nil {
		return out, err
	}
	// One line back for each line in; anything else and the answers can't be lined up.
	if len(lines) != len(asked) {
		return out, nil
	}
	for k, i := range asked {
		out[i] = ParseIdentity(lines[k])
	}
	return out, nil
}

// MailmapState is how the mailmap git reads stood, so a later look can tell whether names
// may read differently now.
type MailmapState struct {
	// Config is the mailmap.file and mailmap.blob settings, as git config lists them.
	Config string
	// Files are the mailmap files git reads from disk: the work tree's .mailmap, and the
	// one mailmap.file names. Stat is their sizes and times, with a blank for a missing one.
	Files []string
	Stat  string
}

// MailmapState reads where the mailmap stands now. A mailmap.blob isn't followed: it names
// a blob through a ref, and the graph reloads when refs move anyway.
func (r *Repo) MailmapState(ctx context.Context) MailmapState {
	s := MailmapState{Files: []string{filepath.Join(r.dir, ".mailmap")}}
	// Exit status 1 is no such setting, the usual case.
	cfg, _ := r.ReadString(ctx, "config", "--type=path", "--get-regexp", `^mailmap\.(file|blob)$`)
	s.Config = strings.TrimSpace(cfg)
	for line := range strings.Lines(s.Config) {
		key, value, _ := strings.Cut(strings.TrimSpace(line), " ")
		if !strings.EqualFold(key, "mailmap.file") || value == "" {
			continue
		}
		// As git reads it: a relative path is from the top of the work tree.
		if !filepath.IsAbs(value) {
			value = filepath.Join(r.dir, value)
		}
		s.Files = append(s.Files, value)
	}
	s.Stat = statFiles(s.Files)
	return s
}

// FilesMoved reports whether a mailmap file has changed, appeared or gone since the state
// was read. It asks the file system and not git, so it's cheap enough to ask on every
// change to the work tree.
func (s MailmapState) FilesMoved() bool { return statFiles(s.Files) != s.Stat }

// Key is the state as one string, for telling a kept answer from a stale one.
func (s MailmapState) Key() string { return s.Config + "\n\n" + s.Stat }

// Same reports whether two states read names alike.
func (s MailmapState) Same(other MailmapState) bool {
	return s.Config == other.Config && s.Stat == other.Stat
}

func statFiles(paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			fmt.Fprintf(&b, "%d@%d", info.Size(), info.ModTime().UnixNano())
		}
		b.WriteByte('\n')
	}
	return b.String()
}
