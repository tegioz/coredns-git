package git

import (
	"io/ioutil"
	"os"
	"strings"
)

// writeScriptFile writes content to a temporary file.
// It changes the temporary file mode to executable and
// closes it to prepare it for execution.
func writeScriptFile(content []byte) (file *os.File, err error) {
	if file, err = ioutil.TempFile("", "coredns"); err != nil {
		return nil, err
	}
	if _, err = file.Write(content); err != nil {
		return nil, err
	}
	if err = file.Chmod(os.FileMode(0755)); err != nil {
		return nil, err
	}
	return file, file.Close()
}

// gitWrapperScript forms content for git.sh script
func gitWrapperScript() []byte {
	scriptTemplate := `#!/bin/sh

# The MIT License (MIT)
# Copyright (c) 2013 Alvin Abad

if [ $# -eq 0 ]; then
    echo "Git wrapper script that can specify an ssh-key file
Usage:
    git.sh -i ssh-key-file git-command
    "
    exit 1
fi

# remove temporary file on exit
trap 'rm -f {tmp_dir}/.git_ssh.$$' 0

if [ "$1" = "-i" ]; then
    SSH_KEY=$2; shift; shift
    echo -e "#!/usr/bin/env {shell}\n \
    ssh -i $SSH_KEY \$@" > {tmp_dir}/.git_ssh.$$
    chmod +x {tmp_dir}/.git_ssh.$$
    export GIT_SSH={tmp_dir}/.git_ssh.$$
fi

# in case the git command is repeated
[ "$1" = "git" ] && shift

# Run the git command
git "$@"

`
	replacer := strings.NewReplacer(
		"{tmp_dir}", strings.TrimSuffix(os.TempDir(), "/"),
	)
	return []byte(replacer.Replace(scriptTemplate))
}

// bashScript forms content of bash script to clone or update a repo using ssh
func bashScript(gitSSHPath string, repo *Repo, params []string) []byte {
	scriptTemplate := `#!/bin/sh

mkdir -p ~/.ssh;
touch ~/.ssh/known_hosts;
ssh-keyscan -t rsa,dsa {repo_host} 2>&1 | sort -u - ~/.ssh/known_hosts > ~/.ssh/tmp_hosts;
cat ~/.ssh/tmp_hosts | while read line
do
  grep -q "$line" ~/.ssh/known_hosts || echo $line >> ~/.ssh/known_hosts;
done
{git_ssh_path} -i {ssh_key_path} {ssh_params};
`
	replacer := strings.NewReplacer(
		"{repo_host}", repo.Host,
		"{git_ssh_path}", gitSSHPath,
		"{ssh_key_path}", repo.KeyPath,
		"{ssh_params}", strings.Join(params, " "),
	)
	return []byte(replacer.Replace(scriptTemplate))
}
