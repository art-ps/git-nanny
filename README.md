# git-nanny

Cleans up the graveyard of local branches. Shows what was merged, what is left
over from branches deleted on the remote, and what nobody has touched for
months — you tick them off and delete the lot.

built in one evening · claude code + go · bubble tea

## Why not the one-liner

`git branch --merged | xargs git branch -d` does not see branches merged with a
**squash** commit — and that is how most pull requests land on GitHub and
GitLab. It does not see branches whose upstream was deleted after the merge,
either. git-nanny finds both, and writes a journal before deleting, so any
branch can be brought back.

## Install

Homebrew (macOS and Linux):

    brew install art-ps/tap/git-nanny

Or from source:

    go install github.com/art-ps/git-nanny/cmd/git-nanny@latest

Prebuilt binaries for macOS and Linux (x86_64 and arm64) are in the
[releases](https://github.com/art-ps/git-nanny/releases).

The binary is called `git-nanny`, so git picks it up as a subcommand: both
`git-nanny` and `git nanny` work.

## Usage

Full reference: `git nanny --help` (Homebrew installs the man page alongside
the binary; with `go install`, `make install-man` puts it in place).

    git nanny                          # interactive list
    git nanny --merged --yes           # delete merged branches
    git nanny --all-but-default --yes  # delete everything except the default one
    git nanny --dry-run                # only show the plan
    git nanny --stale-days 30          # abandonment threshold (90 by default)
    git nanny restore                  # what was deleted and how to bring it back

Without `--merged` or `--all-but-default` a non-interactive run deletes
nothing: it prints the branch list and asks you to name a scope explicitly.

Never deleted: the current branch, the default branch, branches checked out in
another worktree, and anything covered by protection:

    git config --add nanny.protect 'release/*'
    git nanny --protect wip

If the default branch cannot be resolved unambiguously (no `origin/HEAD`, no
`main`/`master`, more than one branch), git-nanny deletes nothing and explains
how to name it: `git config nanny.defaultBranch <name>` or
`git nanny --default-branch <name>`.

`--force` applies to non-interactive runs: without it, branches with commits
the default branch does not have are skipped. In the interactive list you can
tick any branch that is not protected — a "has unique commits" note marks the
ones that are not merged anywhere, and the confirm screen tells you how many
of your picks are unmerged.

Git does not record when a branch was created, so every row ends with
`created 2006-01-02` — the date of the branch's first commit not present on
the default branch.

## What it does not do

It does not touch branches on the remote, does not merge, does not switch.
Local cleanup only.

## License

MIT. See [LICENSE](LICENSE).
