# Repository rules for `master`

`master.json` is the ruleset for `master`: a pull request and the `ci-ok` check are required, with no approval; only squash merges; linear history; no force pushes or deletion. The deploy key is the only bypass actor, so the `deploy` job can push its `deploy(prod)` commit. `integration_id` 15368 is GitHub Actions, so only a `ci-ok` reported by Actions counts.

The web UI may not list deploy keys as bypass actors, so apply the ruleset through the API. Run these from the repository root.

Create the ruleset:

```sh
gh api --method POST repos/DujinA/spendulum/rulesets --input .github/rulesets/master.json
```

Update it after a change to `master.json`:

```sh
id=$(gh api repos/DujinA/spendulum/rulesets --jq '.[] | select(.name == "master") | .id')
gh api --method PUT "repos/DujinA/spendulum/rulesets/$id" --input .github/rulesets/master.json
```

A ruleset cannot delete branches or switch off merge methods, so set those on the repository. `allow_auto_merge` lets Renovate merge on a green `ci-ok`.

```sh
gh api --method PATCH repos/DujinA/spendulum \
  -F allow_squash_merge=true -F allow_merge_commit=false -F allow_rebase_merge=false \
  -F delete_branch_on_merge=true -F allow_auto_merge=true
```

If the API refuses `DeployKey` as a bypass actor, the fallback is a GitHub App token as the bypass actor.
