package deploy_test

import (
	"encoding/json"
	"regexp"
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
	PackageRules []renovateRule      `json:"packageRules"`
	ArgoCD       map[string][]string `json:"argocd"`
	Custom       []struct {
		FilePatterns []string `json:"managerFilePatterns"`
		MatchStrings []string `json:"matchStrings"`
	} `json:"customManagers"`
}

type renovateUpdate struct {
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
func (c renovateConfig) resolve(u renovateUpdate) (group string, automerge bool) {
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
		u    renovateUpdate
		want string
	}{
		{renovateUpdate{"gomod", "require", "minor", "backend/go.mod", "github.com/jackc/pgx/v5"}, "Go modules"},
		{renovateUpdate{"gomod", "require", "minor", "backend/tools/go.mod", "github.com/sqlc-dev/sqlc"}, "Go tools"},
		{renovateUpdate{"gomod", "require", "patch", "deploy/go.mod", "sigs.k8s.io/kustomize/api"}, "Go tools"},
		{renovateUpdate{"npm", "dependencies", "minor", "web/package.json", "react"}, "npm dependencies"},
		{renovateUpdate{"npm", "devDependencies", "minor", "web/package.json", "vite"}, "npm dev dependencies"},
		{renovateUpdate{"dockerfile", "final", "minor", "Dockerfile", "golang"}, "Docker images"},
		{renovateUpdate{"github-actions", "action", "minor", ".github/workflows/ci.yml", "actions/checkout"}, "GitHub Actions"},
		{renovateUpdate{"argocd", "", "minor", "deploy/argocd/cert-manager.yaml", "cert-manager"}, "Helm charts"},
		{renovateUpdate{"npm", "devDependencies", "minor", "e2e/package.json", "@playwright/test"}, "Playwright"},
		{renovateUpdate{"docker-compose", "", "minor", "e2e/compose.e2e.yaml", "mcr.microsoft.com/playwright"}, "Playwright"},
	} {
		if got, _ := c.resolve(tc.u); got != tc.want {
			t.Errorf("%s %s in %s: group = %q, want %q", tc.u.manager, tc.u.pkg, tc.u.file, got, tc.want)
		}
	}
}

func TestRenovateAutomergesOnlyDevDependencyAndActionPatchesAndMinors(t *testing.T) {
	c := renovate(t)

	for _, tc := range []struct {
		u    renovateUpdate
		want bool
	}{
		{renovateUpdate{"npm", "devDependencies", "patch", "web/package.json", "vite"}, true},
		{renovateUpdate{"npm", "devDependencies", "minor", "e2e/package.json", "@playwright/test"}, true},
		{renovateUpdate{"gomod", "require", "minor", "backend/tools/go.mod", "github.com/sqlc-dev/sqlc"}, true},
		{renovateUpdate{"github-actions", "action", "patch", ".github/workflows/ci.yml", "actions/checkout"}, true},
		{renovateUpdate{"github-actions", "action", "digest", ".github/workflows/ci.yml", "actions/checkout"}, true},
		{renovateUpdate{"docker-compose", "", "minor", "e2e/compose.e2e.yaml", "mcr.microsoft.com/playwright"}, true},
		{renovateUpdate{"npm", "devDependencies", "major", "web/package.json", "vite"}, false},
		{renovateUpdate{"github-actions", "action", "major", ".github/workflows/ci.yml", "actions/checkout"}, false},
		{renovateUpdate{"npm", "dependencies", "minor", "web/package.json", "react"}, false},
		{renovateUpdate{"npm", "dependencies", "major", "web/package.json", "react"}, false},
		{renovateUpdate{"gomod", "require", "patch", "backend/go.mod", "github.com/jackc/pgx/v5"}, false},
		{renovateUpdate{"gomod", "require", "major", "backend/go.mod", "github.com/jackc/pgx/v5"}, false},
		{renovateUpdate{"dockerfile", "final", "patch", "Dockerfile", "golang"}, false},
		{renovateUpdate{"argocd", "", "patch", "deploy/argocd/cert-manager.yaml", "cert-manager"}, false},
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

// customDeps applies the custom regex managers to a file, as Renovate does, and returns depName → currentValue.
func (c renovateConfig) customDeps(t *testing.T, file string) map[string]string {
	t.Helper()

	deps := map[string]string{}
	content := readFile(t, "../"+file)

	for _, m := range c.Custom {
		applies := false

		for _, p := range m.FilePatterns {
			applies = applies || regexp.MustCompile(strings.Trim(p, "/")).MatchString(file)
		}

		if !applies {
			continue
		}

		for _, ms := range m.MatchStrings {
			re := regexp.MustCompile(ms)
			for _, match := range re.FindAllStringSubmatch(content, -1) {
				deps[match[re.SubexpIndex("depName")]] = match[re.SubexpIndex("currentValue")]
			}
		}
	}

	return deps
}

func TestRenovateUpdatesThePnpmInTheDockerfile(t *testing.T) {
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal([]byte(readFile(t, "../package.json")), &pkg); err != nil {
		t.Fatal(err)
	}

	want, ok := strings.CutPrefix(pkg.PackageManager, "pnpm@")
	if !ok {
		t.Fatalf("packageManager = %q, want pnpm@<version>", pkg.PackageManager)
	}

	if got := renovate(t).customDeps(t, "Dockerfile")["pnpm"]; got != want {
		t.Errorf("Renovate reads pnpm %q from the Dockerfile, want %q to match packageManager", got, want)
	}
}

func TestRenovateUpdatesBothPnpmPinsInOnePR(t *testing.T) {
	c := renovate(t)
	packageManager := renovateUpdate{"npm", "packageManager", "minor", "package.json", "pnpm"}
	dockerfile := renovateUpdate{"custom.regex", "", "minor", "Dockerfile", "pnpm"}

	for _, u := range []renovateUpdate{packageManager, dockerfile} {
		if group, automerge := c.resolve(u); group != "pnpm" || !automerge {
			t.Errorf("%s in %s minor: group %q, automerge %v; want pnpm, true", u.manager, u.file, group, automerge)
		}

		u.updateType = "major"
		if group, automerge := c.resolve(u); group != "pnpm" || automerge {
			t.Errorf("%s in %s major: group %q, automerge %v; want pnpm, false", u.manager, u.file, group, automerge)
		}
	}
}
