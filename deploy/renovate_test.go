package deploy_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

type renovateRule struct {
	MatchManagers     []string `json:"matchManagers"`
	MatchDepTypes     []string `json:"matchDepTypes"`
	MatchUpdateTypes  []string `json:"matchUpdateTypes"`
	MatchFileNames    []string `json:"matchFileNames"`
	MatchPackageNames []string `json:"matchPackageNames"`
	GroupName         string   `json:"groupName"`
	Automerge         *bool    `json:"automerge"`
}

type renovateConfig struct {
	Schedule     []string            `json:"schedule"`
	Automerge    *bool               `json:"automerge"`
	PackageRules []renovateRule      `json:"packageRules"`
	ArgoCD       map[string][]string `json:"argocd"`
}

type update struct {
	manager, depType, updateType, file, pkg string
}

func renovate(t *testing.T) renovateConfig {
	t.Helper()

	var c renovateConfig
	if err := json.Unmarshal([]byte(readFile(t, "../renovate.json")), &c); err != nil {
		t.Fatalf("parse renovate.json: %v", err)
	}

	return c
}

func matchesAny(values []string, v string) bool {
	return len(values) == 0 || slices.Contains(values, v)
}

// resolve applies packageRules in order, as Renovate does, for the matchers this config uses.
func (c renovateConfig) resolve(u update) (group string, automerge bool) {
	if c.Automerge != nil {
		automerge = *c.Automerge
	}

	for _, r := range c.PackageRules {
		if !matchesAny(r.MatchManagers, u.manager) || !matchesAny(r.MatchDepTypes, u.depType) ||
			!matchesAny(r.MatchUpdateTypes, u.updateType) || !matchesAny(r.MatchFileNames, u.file) ||
			!matchesAny(r.MatchPackageNames, u.pkg) {
			continue
		}

		if r.GroupName != "" {
			group = r.GroupName
		}

		if r.Automerge != nil {
			automerge = *r.Automerge
		}
	}

	return group, automerge
}

func TestRenovateRunsWeekly(t *testing.T) {
	c := renovate(t)

	if len(c.Schedule) != 1 || !strings.Contains(c.Schedule[0], "on monday") {
		t.Errorf("schedule = %v, want one weekly window", c.Schedule)
	}
}

func TestRenovateGroupsUpdatesPerEcosystem(t *testing.T) {
	c := renovate(t)

	for _, tc := range []struct {
		u    update
		want string
	}{
		{update{"gomod", "require", "minor", "backend/go.mod", "github.com/jackc/pgx/v5"}, "Go modules"},
		{update{"gomod", "require", "minor", "backend/tools/go.mod", "github.com/sqlc-dev/sqlc"}, "Go tools"},
		{update{"gomod", "require", "patch", "deploy/go.mod", "sigs.k8s.io/kustomize/api"}, "Go tools"},
		{update{"npm", "dependencies", "minor", "web/package.json", "react"}, "npm dependencies"},
		{update{"npm", "devDependencies", "minor", "web/package.json", "vite"}, "npm dev dependencies"},
		{update{"dockerfile", "final", "minor", "Dockerfile", "golang"}, "Docker images"},
		{update{"github-actions", "action", "minor", ".github/workflows/ci.yml", "actions/checkout"}, "GitHub Actions"},
		{update{"argocd", "", "minor", "deploy/argocd/cert-manager.yaml", "cert-manager"}, "Helm charts"},
		{update{"npm", "devDependencies", "minor", "e2e/package.json", "@playwright/test"}, "Playwright"},
		{update{"docker-compose", "", "minor", "e2e/compose.e2e.yaml", "mcr.microsoft.com/playwright"}, "Playwright"},
	} {
		if got, _ := c.resolve(tc.u); got != tc.want {
			t.Errorf("%s %s in %s: group = %q, want %q", tc.u.manager, tc.u.pkg, tc.u.file, got, tc.want)
		}
	}
}

func TestRenovateAutomergesOnlyDevDependencyAndActionPatchesAndMinors(t *testing.T) {
	c := renovate(t)

	for _, tc := range []struct {
		u    update
		want bool
	}{
		{update{"npm", "devDependencies", "patch", "web/package.json", "vite"}, true},
		{update{"npm", "devDependencies", "minor", "e2e/package.json", "@playwright/test"}, true},
		{update{"gomod", "require", "minor", "backend/tools/go.mod", "github.com/sqlc-dev/sqlc"}, true},
		{update{"github-actions", "action", "patch", ".github/workflows/ci.yml", "actions/checkout"}, true},
		{update{"github-actions", "action", "digest", ".github/workflows/ci.yml", "actions/checkout"}, true},
		{update{"docker-compose", "", "minor", "e2e/compose.e2e.yaml", "mcr.microsoft.com/playwright"}, true},
		{update{"npm", "devDependencies", "major", "web/package.json", "vite"}, false},
		{update{"github-actions", "action", "major", ".github/workflows/ci.yml", "actions/checkout"}, false},
		{update{"npm", "dependencies", "minor", "web/package.json", "react"}, false},
		{update{"npm", "dependencies", "major", "web/package.json", "react"}, false},
		{update{"gomod", "require", "patch", "backend/go.mod", "github.com/jackc/pgx/v5"}, false},
		{update{"gomod", "require", "major", "backend/go.mod", "github.com/jackc/pgx/v5"}, false},
		{update{"dockerfile", "final", "patch", "Dockerfile", "golang"}, false},
		{update{"argocd", "", "patch", "deploy/argocd/cert-manager.yaml", "cert-manager"}, false},
	} {
		if _, got := c.resolve(tc.u); got != tc.want {
			t.Errorf("%s %s %s in %s: automerge = %v, want %v", tc.u.manager, tc.u.updateType, tc.u.pkg, tc.u.file, got, tc.want)
		}
	}
}

func TestRenovateReadsHelmChartsFromTheArgoCDApplications(t *testing.T) {
	c := renovate(t)

	patterns := c.ArgoCD["managerFilePatterns"]
	if len(patterns) != 1 || !strings.Contains(patterns[0], "deploy/argocd/") {
		t.Errorf("argocd.managerFilePatterns = %v, want deploy/argocd/", patterns)
	}
}
