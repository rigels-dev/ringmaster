# Publishing

[← README](../../README.md)

## Pushing

Tasks are merged into your local `main`; nothing leaves your machine until you push. Give the project a remote once, then push from the command line or from the Branches view of the web UI:

```bash
ringmaster remote set git@github.com:you/project.git
ringmaster remote        # the remote, and which branches still need pushing
ringmaster push          # main, every task branch (merged or not) and the release tags
ringmaster push --main-only
ringmaster prune         # delete the task branches already merged into main
ringmaster prune --remote  # and delete them on origin too
```

Only `main`, the task branches and the steps' release tags are pushed, each named explicitly from Ringmaster's own record; your other branches and tags are left alone, even ones with similar names. A release tagged without pushing (**Tag only**, or `ringmaster release` without `--push`) shows as *not pushed* in Branches and goes with the next push. A push is never forced: if the remote has commits this project does not, that branch is rejected and reported. Pushing uses your own git login (an SSH key or a credential helper) and never prompts, so a missing login fails at once. Ringmaster refuses a URL with credentials in it, since it would be stored in `.git/config`.

A task branch merged into `main` does not need pushing: its commits go with `main`. So once `main` (and any unfinished task branch) is on the remote, the Branches view says everything is pushed. Merged task branches can then be deleted with **Delete merged branches…** or `ringmaster prune`. The work stays in `main`'s history; only the branch names go. `main`, unfinished task branches, branches checked out in a worktree and your own branches are never deleted. With `--remote` (or the checkbox in the web UI) they are deleted on origin too, unless someone has moved the branch there since this project last saw it.

## Steps

A finished plan becomes a *step* of the project: Step 1, Step 2, and so on, each ending at the commit on `main` where its last task was merged. The Tasks view says the step is done and links to Branches, where each step can be named, released, downloaded, or finished without a release. From the command line:

```bash
ringmaster steps                          # every step, and its release
ringmaster steps name 1 "First version"
ringmaster release                        # the last release, and suggested next versions
ringmaster release v0.2.0 --step 2        # tag where step 2 ended
ringmaster release v0.2.0 --step 2 --push # and push main and that tag to origin
ringmaster release notes --step 2 --ai     # draft the release notes, with a summary by AI
ringmaster release v0.2.0 --step 2 --notes-file notes.md --push --github-release
ringmaster release page v0.1.0            # a GitHub release page for a tag pushed earlier
ringmaster steps close 2                  # or: finished without a release
ringmaster archive v0.2.0                 # .ringmaster/archives/<project>-v0.2.0.zip
```

Steps are kept in `.ringmaster/steps.json`, so they are never committed or pushed. Their releases are read from git tags, so a tag you make by hand on a step's last commit counts too. Without `--step`, `ringmaster release` tags the tip of `main`.

## Releases

A release is an annotated git tag on a commit of `main`, made in your name. The version is yours to choose, in any scheme. Ringmaster only suggests: from `v0.1.0` it offers `v0.1.1` (fixes only), `v0.2.0` (new features) and `v1.0.0` (breaking changes), and marks the one that fits what was built; with no tag yet it offers `v0.1.0`. A tag that is not a version number gets no suggestion. A version that exists is never moved or replaced, and a release is refused while tasks of the current plan are unfinished (`--force` overrides). An archive holds only the files git tracks at that tag, so Ringmaster's state and untracked files stay out; `--format tar.gz` is also available.

## Release notes

The Release form starts with notes drafted from the step's tasks, one line each under *Features*, *Fixes*, *Documentation* or *Tests* (from the kind its branch names), and a place for a summary. Write the summary yourself, or press **Write summary with AI**: the planner model reads the tasks' specs and the files they changed and writes a sentence or two. That is one model call on your provider's quota, made only when you press it; the answers you gave the automatons are not sent. The notes go into the tag's message and are kept in `.ringmaster/releases/`.

When the remote is on GitHub and the [GitHub CLI](https://cli.github.com) (`gh`) is installed and logged in, **Tag and push** also creates the release's page on GitHub with those notes. A release pushed without a page gets a **Create GitHub release page…** button in Branches (or `ringmaster release page <version>`). If the page cannot be made, the tag and the push still stand, and the reason is shown.
