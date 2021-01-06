package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coredns/caddy"
)

const (
	// Number of retries if git pull fails
	numRetries = 3

	// variable for latest tag
	latestTag = "{latest}"
)

// Git represent multiple repositories.
type Git []*Repo

// Repo retrieves repository at i or nil if not found.
func (g Git) Repo(i int) *Repo {
	if i < len(g) {
		return g[i]
	}
	return nil
}

// Repo is the structure that holds required information
// of a git repository.
type Repo struct {
	URL        string        // Repository URL
	Path       string        // Directory to pull to
	Branch     string        // Git branch
	Interval   time.Duration // Interval between pulls
	CloneArgs  []string      // Additonal cli args to pass to git clone
	PullArgs   []string      // Additonal cli args to pass to git pull
	pulled     bool          // true if there was a successful pull
	lastPull   time.Time     // time of the last successful pull
	lastCommit string        // hash for the most recent commit
	latestTag  string        // latest tag name
	sync.Mutex
}

// Pull attempts a git pull.
// It retries at most numRetries times if error occurs
func (r *Repo) Pull() error {
	r.Lock()
	defer r.Unlock()

	// prevent a pull if the last one was less than 5 seconds ago
	if time.Since(r.lastPull) < 5*time.Second {
		return nil
	}

	// keep last commit hash for comparison later
	lastCommit := r.lastCommit

	var err error
	// Attempt to pull at most numRetries times
	for i := 0; i < numRetries; i++ {
		if err = r.pull(); err == nil {
			break
		}
		log.Warning(err)
	}

	if err != nil {
		return err
	}

	// check if there are new changes,
	// then execute post pull command
	if r.lastCommit == lastCommit {
		log.Info("No new changes")
	}
	return nil
}

// pull performs git pull, or git clone if repository does not exist.
func (r *Repo) pull() error {

	// if not pulled, perform clone
	if !r.pulled {
		return r.clone()
	}

	// if latest tag config is set
	if r.Branch == latestTag {
		if err := r.checkoutLatestTag(); err != nil {
			log.Errorf("Error retrieving latest tag: %s", err)
			return err
		}
		return nil
	}

	params := append([]string{"pull"}, append(r.PullArgs, "origin", r.Branch)...)
	var err error
	if err = r.gitCmd(params, r.Path); err == nil {
		r.pulled = true
		r.lastPull = time.Now()
		log.Infof("pulled: %v", r.URL)
		r.lastCommit, err = r.mostRecentCommit()
	}
	return err
}

// clone performs git clone.
func (r *Repo) clone() error {
	params := append([]string{"clone", "-b", r.Branch}, append(r.CloneArgs, r.URL, r.Path)...)

	tagMode := r.Branch == latestTag
	if tagMode {
		params = append([]string{"clone"}, append(r.CloneArgs, r.URL, r.Path)...)
	}

	var err error
	if err = r.gitCmd(params, ""); err == nil {
		r.pulled = true
		r.lastPull = time.Now()
		log.Infof("pulled: %v", r.URL)
		r.lastCommit, err = r.mostRecentCommit()

		// if latest tag config is set.
		if tagMode {
			if err := r.checkoutLatestTag(); err != nil {
				log.Errorf("Error retrieving latest tag: %s", err)
			}
			return err
		}
	}

	return err
}

// checkoutLatestTag checks out the latest tag of the repository.
func (r *Repo) checkoutLatestTag() error {
	tag, err := r.fetchLatestTag()
	if err != nil {
		return err
	}
	if tag == "" {
		return fmt.Errorf("no tags found for repo: %v", r.URL)
	} else if tag == r.latestTag {
		return nil
	}

	params := []string{"checkout", "tags/" + tag}
	if err = r.gitCmd(params, r.Path); err == nil {
		r.latestTag = tag
		r.lastCommit, err = r.mostRecentCommit()
	} else {
		return err
	}
	return nil
}

// checkoutCommit checks out the specified commitHash.
func (r *Repo) checkoutCommit(commitHash string) error {
	var err error
	params := []string{"checkout", commitHash}
	if err = r.gitCmd(params, r.Path); err == nil {
		log.Infof("commit %v checkout done", commitHash)
	}
	return err
}

// gitCmd performs a git command.
func (r *Repo) gitCmd(params []string, dir string) error { return runCmd("git", params, dir) }

// Prepare prepares for a git pull
// and validates the configured directory
func (r *Repo) Prepare() error {
	// check if directory exists or is empty
	// if not, create directory
	fs, err := ioutil.ReadDir(r.Path)
	if err != nil || len(fs) == 0 {
		return os.MkdirAll(r.Path, os.FileMode(0755))
	}

	// validate git repo
	isGit := false
	for _, f := range fs {
		if f.IsDir() && f.Name() == ".git" {
			isGit = true
			break
		}
	}

	if isGit {
		// check if same repository
		var repoURL string
		if repoURL, err = r.originURL(); err == nil {
			if strings.TrimSuffix(repoURL, ".git") == strings.TrimSuffix(r.URL, ".git") {
				r.pulled = true
				return nil
			}
		}
		if err != nil {
			return fmt.Errorf("cannot retrieve repo url for %v: %s", r.Path, err)
		}
		return fmt.Errorf("another git repo '%v' exists at %v", repoURL, r.Path)
	}
	return fmt.Errorf("cannot git clone into %v, directory not empty", r.Path)
}

// getMostRecentCommit gets the hash of the most recent commit to the
// repository. Useful for checking if changes occur.
func (r *Repo) mostRecentCommit() (string, error) {
	command := "git" + ` --no-pager log -n 1 --pretty=format:"%H"`
	c, args, err := caddy.SplitCommandAndArgs(command)
	if err != nil {
		return "", err
	}
	return runCmdOutput(c, args, r.Path)
}

// fetchLatestTag retrieves the most recent tag in the repository.
func (r *Repo) fetchLatestTag() (string, error) {
	// fetch updates to get latest tag
	params := []string{"fetch", "origin", "--tags"}
	err := r.gitCmd(params, r.Path)
	if err != nil {
		return "", err
	}
	// retrieve latest tag
	command := "git" + ` describe origin --abbrev=0 --tags`
	c, args, err := caddy.SplitCommandAndArgs(command)
	if err != nil {
		return "", err
	}
	return runCmdOutput(c, args, r.Path)
}

// originURL retrieves remote origin url for the git repository at path
func (r *Repo) originURL() (string, error) {
	_, err := os.Stat(r.Path)
	if err != nil {
		return "", err
	}
	args := []string{"config", "--get", "remote.origin.url"}
	return runCmdOutput("git", args, r.Path)
}
