package git

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("git")

const (
	// DefaultInterval is the minimum interval to delay before
	// requesting another git pull
	DefaultInterval time.Duration = time.Hour
)

func init() { plugin.Register("git", setup) }

func setup(c *caddy.Controller) error {
	git, err := parse(c)
	if err != nil {
		return err
	}

	var startupFuncs []func() error // functions to execute at startup

	// loop through all repos and and start monitoring
	for i := range git {
		repo := git.Repo(i)

		startupFuncs = append(startupFuncs, func() error {

			// Start service routine in background
			Start(repo)

			// Do a pull right away to return error
			return repo.Pull()
		})
	}

	// ensure the functions are executed once per server block
	// for cases like server1.com, server2.com { ... }
	c.OncePerServerBlock(func() error {
		for i := range startupFuncs {
			c.OnStartup(startupFuncs[i])
		}
		return nil
	})

	return nil
}

func parse(c *caddy.Controller) (Git, error) {
	var git Git

	config := dnsserver.GetConfig(c)
	for c.Next() {
		repo := &Repo{Branch: "master", Interval: DefaultInterval, Path: config.Root}

		args := c.RemainingArgs()

		clonePath := func(s string) string {
			if filepath.IsAbs(s) {
				return filepath.Clean(s)
			}
			return filepath.Join(config.Root, s)
		}

		switch len(args) {
		case 2:
			repo.Path = clonePath(args[1])
			fallthrough
		case 1:
			repo.URL = args[0]
		}

		for c.NextBlock() {
			switch c.Val() {
			case "repo":
				if !c.NextArg() {
					return nil, plugin.Error("git", c.ArgErr())
				}
				repo.URL = c.Val()
			case "path":
				if !c.NextArg() {
					return nil, plugin.Error("git", c.ArgErr())
				}
				repo.Path = clonePath(c.Val())
			case "branch":
				if !c.NextArg() {
					return nil, plugin.Error("git", c.ArgErr())
				}
				repo.Branch = c.Val()
			case "interval":
				if !c.NextArg() {
					return nil, plugin.Error("git", c.ArgErr())
				}
				t, _ := strconv.Atoi(c.Val())
				if t > 0 {
					repo.Interval = time.Duration(t) * time.Second
				}
			case "args":
				repo.CloneArgs = c.RemainingArgs()
			case "pull_args":
				repo.PullArgs = c.RemainingArgs()
			default:
				return nil, plugin.Error("git", c.ArgErr())
			}
		}

		// if repo is not specified, return error
		if repo.URL == "" {
			return nil, plugin.Error("git", fmt.Errorf("no URL set"))
		}

		if repo.Path == "" {
			return nil, plugin.Error("git", fmt.Errorf("no path set"))
		}

		// prepare repo for use
		if err := repo.Prepare(); err != nil {
			return nil, plugin.Error("git", err)
		}

		git = append(git, repo)
	}

	return git, nil
}
