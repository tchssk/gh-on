// Package config loads the rules file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	ghconfig "github.com/cli/go-gh/v2/pkg/config"
	"go.yaml.in/yaml/v4"
)

// RuleKey identifies a target that has already been handled.
type RuleKey string

const (
	KeyURL     RuleKey = "url"      // the target URL
	KeyURLHead RuleKey = "url+head" // the target URL and, for pull requests, the head SHA
)

// Path returns the location of the rules file.
func Path() string {
	return filepath.Join(ghconfig.ConfigDir(), "gh-on", "rules.yaml")
}

type Config struct {
	Rules []*Rule `yaml:"rules"`
}

type Rule struct {
	Name string `yaml:"name"`

	// Notification conditions. Empty conditions match anything,
	// but either Owner or Repo is required.
	Type   []string `yaml:"type"`   // subject.type, e.g. PullRequest
	Reason []string `yaml:"reason"` // reason, e.g. review_requested
	Owner  []string `yaml:"owner"`  // repository owner
	Repo   []string `yaml:"repo"`   // OWNER/REPO

	// Query is the search that finds the targets. Results outside Owner and Repo are dropped.
	Query string `yaml:"query"`
	// Key identifies a handled target: url (default) or url+head.
	Key RuleKey `yaml:"key"`

	// Command is run with sh -c. The target URLs are passed as positional parameters ($@).
	Command string `yaml:"command"`
	// Dir is the working directory of the command. Defaults to the home directory.
	Dir string `yaml:"dir"`
	// SkipIfExists defers the rule to a later run while this path exists.
	SkipIfExists string `yaml:"skip_if_exists"`
}

// Load reads and validates the rules file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse decodes and validates rules.
func Parse(b []byte) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, err
	}
	if err := c.normalize(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) normalize() error {
	if len(c.Rules) == 0 {
		return errors.New("no rules")
	}
	seen := map[string]bool{}
	var errs []error
	for i, r := range c.Rules {
		if r.Name == "" {
			errs = append(errs, fmt.Errorf("rules[%d]: name is required", i))
			continue
		}
		if seen[r.Name] {
			errs = append(errs, fmt.Errorf("%s: duplicate name", r.Name))
		}
		seen[r.Name] = true
		if err := r.normalize(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (r *Rule) normalize() error {
	var errs []error
	if len(r.Owner) == 0 && len(r.Repo) == 0 {
		errs = append(errs, errors.New("owner or repo is required"))
	}
	for _, x := range r.Owner {
		if x == "" || strings.Contains(x, "/") {
			errs = append(errs, fmt.Errorf("owner %q must be OWNER", x))
		}
	}
	for _, x := range r.Repo {
		if o, n, ok := strings.Cut(x, "/"); !ok || o == "" || n == "" || strings.Contains(n, "/") {
			errs = append(errs, fmt.Errorf("repo %q must be OWNER/REPO", x))
		}
	}
	if strings.TrimSpace(r.Query) == "" {
		errs = append(errs, errors.New("query is required"))
	} else if hasQualifier(r.Query, "repo:") && hasQualifier(r.Query, "org:") {
		// GitHub search ORs org: and repo:, so the repo: restriction would be lost.
		errs = append(errs, errors.New("query must not combine org: with repo: (they are ORed); -repo: is fine"))
	}
	switch r.Key {
	case "":
		r.Key = KeyURL
	case KeyURL, KeyURLHead:
	default:
		errs = append(errs, fmt.Errorf("key %q must be %s or %s", r.Key, KeyURL, KeyURLHead))
	}
	if strings.TrimSpace(r.Command) == "" {
		errs = append(errs, errors.New("command is required"))
	}
	r.Dir = expandHome(r.Dir)
	r.SkipIfExists = expandHome(r.SkipIfExists)
	return errors.Join(errs...)
}

// hasQualifier reports whether q has the qualifier, ignoring negated ones such as -repo:.
func hasQualifier(q, prefix string) bool {
	for f := range strings.FieldsSeq(q) {
		if strings.HasPrefix(strings.ToLower(f), prefix) {
			return true
		}
	}
	return false
}

// MatchRepo reports whether OWNER/REPO is within the rule's Owner or Repo.
func (r *Rule) MatchRepo(fullName string) bool {
	owner, _, _ := strings.Cut(fullName, "/")
	return (len(r.Owner) > 0 && has(r.Owner, owner)) ||
		(len(r.Repo) > 0 && has(r.Repo, fullName))
}

// Match reports whether a notification with these fields matches the rule.
func (r *Rule) Match(subjectType, reason, repo string) bool {
	return has(r.Type, subjectType) && has(r.Reason, reason) && r.MatchRepo(repo)
}

// has reports whether v is in list, ignoring case. An empty list matches anything.
func has(list []string, v string) bool {
	return len(list) == 0 || slices.ContainsFunc(list, func(x string) bool {
		return strings.EqualFold(x, v)
	})
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[1:])
		}
	}
	return p
}
