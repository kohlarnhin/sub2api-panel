---
name: sub2api-panel-release
description: Project-specific workflow for /Users/zhujiangyong/Software/sub2api-panel. Use when Codex makes a new feature, UI change, bug fix, Docker/compose change, or release in this repo; when the user asks to start work, create a branch, merge to release, publish, tag, or says "可以上"; and when deciding branch names or release tag increments.
---

# Sub2API Panel Release Workflow

## Scope

Apply this workflow only inside `/Users/zhujiangyong/Software/sub2api-panel`.

Do not commit local runtime config such as root `config.yaml` unless the user explicitly asks. It is for local Docker Compose execution.

## Starting New Work

Always base new feature or modification branches on the latest local/remote `release` branch.

1. Check the current branch and worktree first:
   ```bash
   git status -sb
   git fetch origin release --tags
   ```
2. If there are uncommitted user changes, do not discard them. Work with them or ask if they block branch switching.
3. Create the branch from `release`.

Branch naming:

- First work branch of a day: `YYMMDD`
- Additional work branches after a release on the same day: `YYMMDD_1`, `YYMMDD_2`, etc.
- Use the current date from the environment, not memory.
- Example for May 27, 2026:
  - first branch: `260527`
  - next branch after a same-day release: `260527_1`
  - next one: `260527_2`

Pick the next suffix by checking local and remote branches:

```bash
git branch --list '260527*'
git ls-remote --heads origin '260527*'
```

## Local Development

For ordinary frontend/backend changes:

- Make focused edits.
- Do not start services, rebuild Docker images, or run tests unless the user asks or explicitly authorizes it.
- If the user asks to rebuild/restart locally, use:
  ```bash
  docker compose up -d --build
  ```
- Validate with the smallest non-destructive checks that fit the change, such as `git diff --check`, `docker compose config`, or reading diffs.

## Release Trigger

When the user says "可以上", "发布", "发版", "上", "tag", or otherwise approves release:

1. Commit the current work branch changes, excluding local-only files like `config.yaml`.
2. Switch to `release`.
3. Fast-forward or merge the work branch into `release`.
4. Compute the next release tag.
5. Create the tag on `release`.
6. Push `release` and the tag.
7. Watch GitHub Actions Docker Release until it succeeds or fails.

Prefer fast-forward when possible:

```bash
git switch release
git merge --ff-only <work-branch>
```

If fast-forward is not possible, inspect the graph before merging. Do not use destructive resets.

## Tag Increment Rule

Release tags are numeric semantic tags without a leading `v`.

Increment the patch number until it reaches `9`, then roll over:

- `0.0.1`
- `0.0.2`
- ...
- `0.0.9`
- `0.1.0`
- `0.1.1`

General rule:

- If patch `< 9`, next tag is `major.minor.(patch+1)`.
- If patch is `9`, next tag is `major.(minor+1).0`.

Find the latest tag from Git before choosing:

```bash
git fetch origin --tags
git tag --list --sort=-version:refname | head
```

Verify the chosen tag does not already exist locally or remotely:

```bash
git tag --list <tag>
git ls-remote --tags origin <tag>
```

## GitHub Actions

This repo publishes Docker images through `.github/workflows/docker-release.yaml`.

After pushing a release tag, monitor the release run:

```bash
gh run list --repo kohlarnhin/sub2api-panel --limit 5
gh run watch <run-id> --repo kohlarnhin/sub2api-panel --interval 5 --exit-status
```

Successful release publishes:

- `ghcr.io/kohlarnhin/sub2api-panel:<tag>`
- `ghcr.io/kohlarnhin/sub2api-panel:latest`

Report the commit, branch, tag, workflow status, and image names to the user.
