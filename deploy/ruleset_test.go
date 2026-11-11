package deploy_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

type ruleset struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
	Enforcement string `json:"enforcement"`
	Conditions  struct {
		RefName struct {
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"ref_name"`
	} `json:"conditions"`
	BypassActors []struct {
		ActorID    *int   `json:"actor_id"`
		ActorType  string `json:"actor_type"`
		BypassMode string `json:"bypass_mode"`
	} `json:"bypass_actors"`
	Rules []struct {
		Type       string          `json:"type"`
		Parameters json.RawMessage `json:"parameters"`
	} `json:"rules"`
}

func masterRuleset(t *testing.T) ruleset {
	t.Helper()

	dec := json.NewDecoder(strings.NewReader(readFile(t, "../.github/rulesets/master.json")))
	dec.DisallowUnknownFields()

	var r ruleset
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("parse master.json: %v", err)
	}

	return r
}

func (r ruleset) findRule(t *testing.T, ruleType string, params any) bool {
	t.Helper()

	for _, rule := range r.Rules {
		if rule.Type != ruleType {
			continue
		}

		if params != nil {
			if err := json.Unmarshal(rule.Parameters, params); err != nil {
				t.Fatalf("parse %s parameters: %v", ruleType, err)
			}
		}

		return true
	}

	return false
}

func rulesetChecks(t *testing.T) []string {
	t.Helper()

	var params struct {
		Checks []struct {
			Context string `json:"context"`
		} `json:"required_status_checks"`
	}
	masterRuleset(t).findRule(t, "required_status_checks", &params)

	checks := make([]string, 0, len(params.Checks))
	for _, c := range params.Checks {
		checks = append(checks, c.Context)
	}

	return checks
}

func TestRulesetTargetsMasterAndIsActive(t *testing.T) {
	r := masterRuleset(t)

	if r.Target != "branch" || r.Enforcement != "active" || !slices.Equal(r.Conditions.RefName.Include, []string{"refs/heads/master"}) {
		t.Errorf("ruleset = target %q, enforcement %q, include %v; want an active branch ruleset on refs/heads/master",
			r.Target, r.Enforcement, r.Conditions.RefName.Include)
	}
}

func TestRulesetRequiresAPRAndCIOKWithoutApproval(t *testing.T) {
	r := masterRuleset(t)

	var pr struct {
		Approvals *int `json:"required_approving_review_count"`
	}
	if !r.findRule(t, "pull_request", &pr) {
		t.Fatal("the ruleset does not require a pull request")
	}

	if pr.Approvals == nil || *pr.Approvals != 0 {
		t.Errorf("required_approving_review_count = %v, want 0", pr.Approvals)
	}

	if checks := rulesetChecks(t); !slices.Equal(checks, []string{"ci-ok"}) {
		t.Errorf("required checks = %v, want only ci-ok", checks)
	}

	var status struct {
		Strict *bool `json:"strict_required_status_checks_policy"`
	}
	r.findRule(t, "required_status_checks", &status)

	if status.Strict == nil || *status.Strict {
		t.Errorf("strict_required_status_checks_policy = %v, want false (no up-to-date branch)", status.Strict)
	}
}

func TestRulesetAllowsOnlySquashMergesWithLinearHistory(t *testing.T) {
	r := masterRuleset(t)

	var pr struct {
		AllowedMergeMethods []string `json:"allowed_merge_methods"`
	}
	r.findRule(t, "pull_request", &pr)

	if !slices.Equal(pr.AllowedMergeMethods, []string{"squash"}) {
		t.Errorf("allowed_merge_methods = %v, want [squash]", pr.AllowedMergeMethods)
	}

	for _, rule := range []string{"required_linear_history", "non_fast_forward", "deletion"} {
		if !r.findRule(t, rule, nil) {
			t.Errorf("the ruleset has no %s rule", rule)
		}
	}
}

func TestRulesetIsBypassedOnlyByTheDeployKey(t *testing.T) {
	r := masterRuleset(t)

	if len(r.BypassActors) != 1 {
		t.Fatalf("bypass actors = %+v, want only the deploy key", r.BypassActors)
	}

	a := r.BypassActors[0]
	if a.ActorType != "DeployKey" || a.BypassMode != "always" || a.ActorID != nil {
		t.Errorf("bypass actor = %+v, want actor_type DeployKey, bypass_mode always, actor_id null", a)
	}
}

func TestRulesetREADMEDocumentsTheGhAPICommands(t *testing.T) {
	readme := readFile(t, "../.github/rulesets/README.md")

	for _, want := range []string{
		"gh api --method POST repos/DujinA/spendulum/rulesets --input .github/rulesets/master.json",
		"delete_branch_on_merge=true",
		"allow_merge_commit=false",
		"allow_rebase_merge=false",
		"allow_auto_merge=true",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not document %q", want)
		}
	}
}
