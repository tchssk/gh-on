// Package runner checks notifications once and runs the commands of matching rules.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tchssk/gh-on/internal/config"
	"github.com/tchssk/gh-on/internal/github"
	"github.com/tchssk/gh-on/internal/state"
)

// Launcher runs a rule's command with the target URLs.
type Launcher interface {
	Launch(ctx context.Context, r *config.Rule, urls []string) error
}

type Runner struct {
	Config    *config.Config
	State     *state.State
	StatePath string
	Client    github.ClientOps
	Launcher  Launcher
	Log       io.Writer
	Now       func() time.Time

	DryRun bool // print the commands instead of running them, and save nothing
	All    bool // check every rule without looking at notifications
}

type target struct {
	url string
	key string
}

func (rn *Runner) logf(format string, args ...any) {
	fmt.Fprintf(rn.Log, "%s %s\n", rn.Now().Format(time.DateTime), fmt.Sprintf(format, args...))
}

func (rn *Runner) save() error {
	if rn.DryRun {
		return nil
	}
	return rn.State.Save(rn.StatePath)
}

// Run checks notifications once. It returns the errors of all rules joined.
func (rn *Runner) Run(ctx context.Context) error {
	st := rn.State
	names := make([]string, len(rn.Config.Rules))
	for i, r := range rn.Config.Rules {
		names[i] = r.Name
	}
	st.Retain(names)

	if rn.All {
		for _, r := range rn.Config.Rules {
			st.Rule(r.Name).Pending = true
		}
	} else {
		ns, etag, notModified, err := rn.Client.Notifications(st.ETag)
		if err != nil {
			return err
		}
		if !notModified {
			rn.markPending(ns)
			st.ETag = etag
		}
	}
	// Save before running commands so a crash does not process the same notifications again.
	if err := rn.save(); err != nil {
		return err
	}

	var errs []error
	for _, r := range rn.Config.Rules {
		if !st.Rule(r.Name).Pending {
			continue
		}
		if err := rn.process(ctx, r); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
		}
	}
	return errors.Join(errs...)
}

// markPending marks the rules that match notifications newer than Since, and advances Since.
func (rn *Runner) markPending(ns []github.Notification) {
	st := rn.State
	since := st.Since
	for _, n := range ns {
		if !n.UpdatedAt.After(since) {
			continue
		}
		if n.UpdatedAt.After(st.Since) {
			st.Since = n.UpdatedAt
		}
		for _, r := range rn.Config.Rules {
			if r.Match(n.Subject.Type, n.Reason, n.Repository.FullName) {
				st.Rule(r.Name).Pending = true
			}
		}
	}
}

func (rn *Runner) process(ctx context.Context, r *config.Rule) error {
	rs := rn.State.Rule(r.Name)
	targets, err := rn.findTargets(r)
	if err != nil {
		return err // stay pending
	}
	rs.Pending = false
	if len(targets) == 0 {
		return rn.save()
	}

	urls := make([]string, len(targets))
	for i, t := range targets {
		urls[i] = t.url
	}
	// Clear Pending before running so a failing command is not retried every minute.
	// The targets are picked up again on the next matching notification.
	if err := rn.save(); err != nil {
		return err
	}
	if rn.DryRun {
		rn.logf("%s: would run: %s -- %s", r.Name, r.Command, strings.Join(urls, " "))
		return nil
	}
	rn.logf("%s: running for %s", r.Name, strings.Join(urls, " "))
	if err := rn.Launcher.Launch(ctx, r, urls); err != nil {
		return err
	}
	now := rn.Now()
	for _, t := range targets {
		rs.Done[t.key] = now
	}
	rn.logf("%s: done", r.Name)
	return rn.save()
}

// findTargets returns the search results that are not done yet.
// It forgets done keys that no longer appear (closed, unassigned, new head, ...).
func (rn *Runner) findTargets(r *config.Rule) ([]target, error) {
	issues, err := rn.Client.SearchIssues(r.Query)
	if err != nil {
		return nil, err
	}
	rs := rn.State.Rule(r.Name)
	current := map[string]bool{}
	var out []target
	for _, it := range issues {
		// Never run for repositories outside the rule, even if the query is too broad.
		if !r.MatchRepo(it.Repo()) {
			continue
		}
		key := it.HTMLURL
		if r.Key == config.KeyURLHead && it.PullRequest != nil {
			sha, err := rn.Client.PullRequestHead(it.Repo(), it.Number)
			if err != nil {
				return nil, err
			}
			key += "@" + sha
		}
		current[key] = true
		if _, ok := rs.Done[key]; !ok {
			out = append(out, target{url: it.HTMLURL, key: key})
		}
	}
	for k := range rs.Done {
		if !current[k] {
			delete(rs.Done, k)
		}
	}
	return out, nil
}
