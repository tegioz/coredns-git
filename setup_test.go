package git

import (
	"fmt"
	"testing"

	"github.com/coredns/caddy"
)

func TestGitParse(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  *Repo
	}{
		{`git git@github.com:user/repo {
			path /tmp/git1
			args --depth 1
		}`, false, &Repo{
			URL:       "git@github.com:user/repo",
			Path:      "/tmp/git1",
			CloneArgs: []string{"--depth", "1"},
		}},
		{`git user:pass@github.com/user/repo.git {
			path /tmp/git1
			args --depth 1
		}`, false, &Repo{
			URL:       "user:pass@github.com/user/repo.git",
			Path:      "/tmp/git1",
			CloneArgs: []string{"--depth", "1"},
		}},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		git, err := parse(c)
		if !test.shouldErr && err != nil {
			t.Errorf("Test %v should not error but found %v", i, err)
			continue
		}
		if test.shouldErr && err == nil {
			t.Errorf("Test %v should error but found nil", i)
			continue
		}
		repo := git.Repo(0)
		if !reposEqual(test.expected, repo) {
			t.Errorf("Test %v expects %v but found %v", i, test.expected, repo)
		}
	}
}

func reposEqual(expected, repo *Repo) bool {
	if expected == nil {
		return repo == nil
	}
	if expected.Branch != "" && expected.Branch != repo.Branch {
		return false
	}
	if expected.Interval != 0 && expected.Interval != repo.Interval {
		return false
	}
	if expected.Path != "" && expected.Path != repo.Path {
		return false
	}
	if expected.URL != "" && expected.URL != repo.URL {
		return false
	}
	if fmt.Sprint(expected.CloneArgs) != fmt.Sprint(repo.CloneArgs) {
		return false
	}
	return true
}
