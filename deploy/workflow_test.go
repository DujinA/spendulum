package deploy_test

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type workflow struct {
	On          map[string]any `yaml:"on"`
	Env         map[string]string
	Permissions map[string]string
	Concurrency *concurrency
	Jobs        map[string]job
}

type concurrency struct {
	Group            string
	CancelInProgress any `yaml:"cancel-in-progress"`
}

type job struct {
	If             string
	Needs          stringList
	TimeoutMinutes int `yaml:"timeout-minutes"`
	Permissions    map[string]string
	Concurrency    *concurrency
	Outputs        map[string]string
	Env            map[string]string
	Steps          []step
}

type step struct {
	ID   string
	Name string
	If   string
	Uses string
	Run  string
	With map[string]any
	Env  map[string]string
}

type stringList []string

func (l *stringList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = []string{n.Value}

		return nil
	}

	var s []string
	if err := n.Decode(&s); err != nil {
		return err
	}

	*l = s

	return nil
}

func loadYAML(t *testing.T, path string, out any) {
	t.Helper()

	if err := yaml.Unmarshal([]byte(readFile(t, path)), out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

func ciWorkflow(t *testing.T) workflow {
	t.Helper()

	var w workflow
	loadYAML(t, "../.github/workflows/ci.yml", &w)

	return w
}

func (w workflow) job(t *testing.T, name string) job {
	t.Helper()

	j, ok := w.Jobs[name]
	if !ok {
		t.Fatalf("ci.yml has no %s job", name)
	}

	return j
}

func (j job) step(t *testing.T, id string) step {
	t.Helper()

	for _, s := range j.Steps {
		if s.ID == id {
			return s
		}
	}

	t.Fatalf("no step with id %s", id)

	return step{}
}

func (j job) runs() string {
	var b strings.Builder
	for _, s := range j.Steps {
		b.WriteString(s.Run + "\n")
	}

	return b.String()
}

func (j job) uses(action string) []step {
	var found []step

	for _, s := range j.Steps {
		if strings.HasPrefix(s.Uses, action+"@") || s.Uses == action {
			found = append(found, s)
		}
	}

	return found
}

func with(s step, key string) string {
	if v, ok := s.With[key]; ok {
		return fmt.Sprint(v)
	}

	return ""
}

func filteredJobs() []string {
	return []string{"contract", "backend", "migrations", "web", "manifests", "image"}
}

// runScript runs a step's shell script the way GitHub does and returns what it wrote to GITHUB_OUTPUT.
func runScript(t *testing.T, script string, env map[string]string) (map[string]string, string, error) {
	t.Helper()

	outputFile := filepath.Join(t.TempDir(), "output")
	writeFile(t, outputFile, "")

	cmd := exec.CommandContext(t.Context(), "bash", "-e", "-o", "pipefail", "-c", script)
	cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+outputFile)

	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	out, err := cmd.CombinedOutput()

	outputs := map[string]string{}

	for line := range strings.Lines(readFile(t, outputFile)) {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			outputs[k] = v
		}
	}

	return outputs, string(out), err
}

type pathFilters map[string][]string

func (w workflow) pathFilters(t *testing.T) pathFilters {
	t.Helper()

	filter := w.job(t, "changes").step(t, "filter")
	if !strings.HasPrefix(filter.Uses, "dorny/paths-filter@") {
		t.Fatalf("filter step uses %q, want dorny/paths-filter", filter.Uses)
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(with(filter, "filters")), &raw); err != nil {
		t.Fatalf("parse filters: %v", err)
	}

	filters := pathFilters{}
	for name, patterns := range raw {
		filters[name] = flatten(patterns)
	}

	return filters
}

// flatten resolves YAML anchors used inside a filter, which dorny/paths-filter flattens too.
func flatten(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, item := range v {
			out = append(out, flatten(item)...)
		}

		return out
	default:
		return nil
	}
}

// matches supports the two pattern forms the filters use: an exact path and a dir/** prefix.
func matches(t *testing.T, pattern, file string) bool {
	t.Helper()

	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok && !strings.ContainsAny(prefix, "*?[") {
		return strings.HasPrefix(file, prefix+"/")
	}

	if strings.ContainsAny(pattern, "*?[") {
		t.Fatalf("pattern %q is neither an exact path nor dir/**; extend matches in this test", pattern)
	}

	return pattern == file
}

// selectedJobs simulates the changes job: its scope step, then the path filters, then its outputs.
func (w workflow) selectedJobs(t *testing.T, event, authorEmail string, files ...string) []string {
	t.Helper()

	changes := w.job(t, "changes")
	scope, out, err := runScript(t, changes.step(t, "scope").Run, map[string]string{
		"EVENT":        event,
		"AUTHOR_EMAIL": authorEmail,
		"DEPLOY_EMAIL": w.Env["DEPLOY_EMAIL"],
	})
	if err != nil {
		t.Fatalf("scope step: %v\n%s", err, out)
	}

	filters := w.pathFilters(t)

	var selected []string

	for _, name := range filteredJobs() {
		want := fmt.Sprintf("${{ steps.scope.outputs.all == 'true' || steps.filter.outputs.%s == 'true' }}", name)
		if got := changes.Outputs[name]; got != want {
			t.Fatalf("changes output %s = %q, want %q", name, got, want)
		}

		hit := scope["all"] == "true"
		for _, f := range files {
			for _, p := range filters[name] {
				hit = hit || matches(t, p, f)
			}
		}

		if hit {
			selected = append(selected, name)
		}
	}

	return selected
}

func TestChangesJobSelectsJobsPerThePathFilterTable(t *testing.T) {
	w := ciWorkflow(t)
	everything := filteredJobs()

	for _, tc := range []struct {
		files []string
		want  []string
	}{
		{[]string{"backend/internal/httpapi/probes.go"}, []string{"contract", "backend", "migrations", "image"}},
		{[]string{"backend/migrations/00002_entries.sql"}, []string{"contract", "backend", "migrations", "image"}},
		{[]string{"web/src/main.tsx"}, []string{"contract", "web", "image"}},
		{[]string{"pnpm-lock.yaml"}, []string{"contract", "web", "image"}},
		{[]string{"package.json"}, []string{"contract", "web", "image"}},
		{[]string{"e2e/tests/smoke.spec.ts"}, []string{"web", "image"}},
		{[]string{"api/openapi.yaml"}, []string{"contract", "backend", "web", "image"}},
		{[]string{"Dockerfile"}, []string{"image"}},
		{[]string{".dockerignore"}, []string{"image"}},
		{[]string{"deploy/overlays/prod/kustomization.yaml"}, []string{"manifests"}},
		{[]string{"backend/tools/go.mod"}, []string{"contract", "backend", "migrations", "manifests", "image"}},
		{[]string{"renovate.json"}, []string{"manifests"}},
		{[]string{".github/workflows/ci.yml"}, everything},
		{[]string{".github/scripts/deploy-prod.sh"}, everything},
		{[]string{"Taskfile.yml"}, everything},
	} {
		t.Run(strings.Join(tc.files, ","), func(t *testing.T) {
			if got := w.selectedJobs(t, "pull_request", "", tc.files...); !slices.Equal(got, tc.want) {
				t.Errorf("jobs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDocsOnlyChangeSelectsNoJob(t *testing.T) {
	w := ciWorkflow(t)

	got := w.selectedJobs(t, "pull_request", "", "docs/adr/0008-forward-only-additive-migrations.md", "README.md", "GLOSSARY.md", "AGENTS.md")
	if len(got) != 0 {
		t.Errorf("a docs-only change runs %v, want only ci-ok", got)
	}
}

func TestDocsOnlyRunStartsNoRunnerWorkBeforeCIOK(t *testing.T) {
	w := ciWorkflow(t)

	// On a pull request paths-filter reads the changed files through the API, so no checkout is needed.
	for _, s := range w.job(t, "changes").uses("actions/checkout") {
		if s.If != "github.event_name == 'push'" {
			t.Errorf("changes checks out the repository on pull requests (if: %q); it should only for pushes", s.If)
		}
	}

	for _, s := range w.job(t, "ci-ok").Steps {
		if s.Uses != "" {
			t.Errorf("ci-ok uses %s; it should only read the needs results", s.Uses)
		}
	}
}

func TestPushToMasterRunsEveryJob(t *testing.T) {
	w := ciWorkflow(t)

	if got := w.selectedJobs(t, "push", "dev@example.invalid", "README.md"); !slices.Equal(got, filteredJobs()) {
		t.Errorf("push to master runs %v, want every job %v", got, filteredJobs())
	}
}

func TestDeployCommitRunsOnlyManifestsAndSkipsDeploy(t *testing.T) {
	w := ciWorkflow(t)

	if w.Env["DEPLOY_EMAIL"] == "" {
		t.Fatal("ci.yml sets no workflow-level DEPLOY_EMAIL")
	}

	got := w.selectedJobs(t, "push", w.Env["DEPLOY_EMAIL"], "deploy/overlays/prod/kustomization.yaml")
	if !slices.Equal(got, []string{"manifests"}) {
		t.Errorf("a deploy commit runs %v, want only manifests", got)
	}

	scope, out, err := runScript(t, w.job(t, "changes").step(t, "scope").Run, map[string]string{
		"EVENT": "push", "AUTHOR_EMAIL": w.Env["DEPLOY_EMAIL"], "DEPLOY_EMAIL": w.Env["DEPLOY_EMAIL"],
	})
	if err != nil || scope["deploy-commit"] != "true" {
		t.Fatalf("scope outputs = %v, err %v, want deploy-commit=true\n%s", scope, err, out)
	}

	changes := w.job(t, "changes")
	if got := changes.Outputs["deploy-commit"]; got != "${{ steps.scope.outputs.deploy-commit }}" {
		t.Errorf("changes output deploy-commit = %q", got)
	}

	if s := changes.step(t, "scope"); s.Env["AUTHOR_EMAIL"] != "${{ github.event.head_commit.author.email }}" || s.Env["EVENT"] != "${{ github.event_name }}" {
		t.Errorf("scope step env = %v, want EVENT and AUTHOR_EMAIL from the event", s.Env)
	}

	if deploy := w.job(t, "deploy"); !strings.Contains(deploy.If, "needs.changes.outputs.deploy-commit != 'true'") {
		t.Errorf("deploy if = %q, want it to skip deploy commits", deploy.If)
	}
}

func TestFilteredJobsRunOnlyWhenTheirFilterMatches(t *testing.T) {
	w := ciWorkflow(t)

	for _, name := range filteredJobs() {
		j := w.job(t, name)

		if !slices.Contains(j.Needs, "changes") {
			t.Errorf("%s does not need changes", name)
		}

		if want := fmt.Sprintf("needs.changes.outputs.%s == 'true'", name); j.If != want {
			t.Errorf("%s if = %q, want %q", name, j.If, want)
		}
	}
}

func TestCIOKNeedsEveryCheckAndAlwaysRuns(t *testing.T) {
	w := ciWorkflow(t)
	ok := w.job(t, "ci-ok")

	if ok.If != "always()" {
		t.Errorf("ci-ok if = %q, want always()", ok.If)
	}

	want := append([]string{"changes"}, filteredJobs()...)
	if got := slices.Sorted(slices.Values(ok.Needs)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Errorf("ci-ok needs %v, want %v", got, want)
	}

	all := slices.Sorted(maps.Keys(w.Jobs))

	if wantAll := slices.Sorted(slices.Values(append(want, "ci-ok", "deploy"))); !slices.Equal(all, wantAll) {
		t.Errorf("jobs = %v, want %v; a new job must be added to ci-ok's needs", all, wantAll)
	}
}

func TestCIOKFailsOnAFailedOrCancelledJobAndPassesOnASkippedOne(t *testing.T) {
	w := ciWorkflow(t)
	ok := w.job(t, "ci-ok")

	if len(ok.Steps) != 1 || ok.Steps[0].Env["RESULTS"] != "${{ join(needs.*.result, ' ') }}" {
		t.Fatalf("ci-ok steps = %+v, want one step reading RESULTS from needs.*.result", ok.Steps)
	}

	for results, wantPass := range map[string]bool{
		"success":                 true,
		"success skipped":         true,
		"success failure skipped": false,
		"success cancelled":       false,
		"":                        true,
	} {
		_, out, err := runScript(t, ok.Steps[0].Run, map[string]string{"RESULTS": results})
		if pass := err == nil; pass != wantPass {
			t.Errorf("RESULTS=%q: pass = %v, want %v\n%s", results, pass, wantPass, out)
		}
	}
}

func assertRunsInOrder(t *testing.T, j job, name string, commands ...string) {
	t.Helper()

	runs := j.runs()
	at := 0

	for _, c := range commands {
		i := strings.Index(runs[at:], c)
		if i < 0 {
			t.Errorf("%s does not run %q after the previous command; runs:\n%s", name, c, runs)

			return
		}

		at += i + len(c)
	}
}

func TestEachCheckJobRunsItsTaskTargets(t *testing.T) {
	w := ciWorkflow(t)

	assertRunsInOrder(t, w.job(t, "contract"), "contract", "task lint:api", "task gen", "git diff --exit-code")
	assertRunsInOrder(t, w.job(t, "backend"), "backend", "task lint:go", "task vulncheck", "task test:go")
	assertRunsInOrder(t, w.job(t, "migrations"), "migrations", "task migrations")
	assertRunsInOrder(t, w.job(t, "web"), "web", "task lint:web", "task test:web", "task build:web")
	assertRunsInOrder(t, w.job(t, "manifests"), "manifests", "task manifests")
}

func TestMigrationsJobComparesWithTheMasterCommitBeforeTheChange(t *testing.T) {
	m := ciWorkflow(t).job(t, "migrations")

	if got := m.Env["MIGRATIONS_BASE_REF"]; got != "HEAD^1" {
		t.Errorf("MIGRATIONS_BASE_REF = %q, want HEAD^1 (the PR merge commit's master parent)", got)
	}

	checkout := m.uses("actions/checkout")
	if len(checkout) != 1 || with(checkout[0], "fetch-depth") != "2" {
		t.Errorf("migrations checkout = %+v, want fetch-depth 2 so HEAD^1 exists", checkout)
	}
}

type task struct {
	Cmds []any
	Deps []any
}

func taskCommands(t *testing.T, name string) string {
	t.Helper()

	var tf struct{ Tasks map[string]task }
	loadYAML(t, "../Taskfile.yml", &tf)

	tk, ok := tf.Tasks[name]
	if !ok {
		t.Fatalf("Taskfile.yml has no %s task", name)
	}

	b, err := yaml.Marshal(tk)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func TestTaskTargetsRunTheDecidedTools(t *testing.T) {
	for name, want := range map[string][]string{
		"lint:api":   {"redocly lint", "api/openapi.yaml"},
		"lint:go":    {"golangci-lint run"},
		"vulncheck":  {"govulncheck ./..."},
		"test:go":    {"go test -race ./..."},
		"migrations": {"go test -race", "./migrations/..."},
		"lint:web":   {"@spendulum/web lint", "@spendulum/web typecheck", "@spendulum/e2e typecheck"},
		"test:web":   {"@spendulum/web test"},
		"build:web":  {"@spendulum/web build"},
	} {
		cmds := taskCommands(t, name)
		for _, w := range want {
			if !strings.Contains(cmds, w) {
				t.Errorf("task %s does not run %q:\n%s", name, w, cmds)
			}
		}
	}
}

func TestImageJobBuildsScansAndRunsE2EOnTheLoadedImage(t *testing.T) {
	img := ciWorkflow(t).job(t, "image")

	if len(img.uses("docker/setup-buildx-action")) != 1 {
		t.Error("image does not set up buildx")
	}

	build := img.uses("docker/build-push-action")
	if len(build) != 1 {
		t.Fatalf("image has %d build steps, want 1", len(build))
	}

	for key, want := range map[string]string{
		"platforms":  "linux/amd64",
		"load":       "true",
		"push":       "false",
		"tags":       "spendulum:e2e",
		"cache-from": "type=gha",
		"cache-to":   "type=gha,mode=max",
	} {
		if got := with(build[0], key); got != want {
			t.Errorf("build %s = %q, want %q", key, got, want)
		}
	}

	trivy := img.uses("aquasecurity/trivy-action")
	if len(trivy) != 1 {
		t.Fatalf("image has %d Trivy steps, want 1", len(trivy))
	}

	for key, want := range map[string]string{
		"image-ref":      "spendulum:e2e",
		"severity":       "CRITICAL,HIGH",
		"ignore-unfixed": "true",
		"exit-code":      "1",
	} {
		if got := with(trivy[0], key); got != want {
			t.Errorf("trivy %s = %q, want %q", key, got, want)
		}
	}

	e2e := img.step(t, "e2e")
	if !strings.Contains(e2e.Run, "task e2e") || e2e.Env["E2E_IMAGE"] != "spendulum:e2e" {
		t.Errorf("e2e step = %+v, want task e2e with E2E_IMAGE=spendulum:e2e", e2e)
	}

	upload := img.uses("actions/upload-artifact")
	if len(upload) != 1 || upload[0].If != "failure() && steps.e2e.outcome == 'failure'" || !strings.Contains(with(upload[0], "path"), "e2e/test-results") {
		t.Errorf("upload steps = %+v, want one that uploads e2e/test-results only when e2e failed", upload)
	}

	if !strings.Contains(readFile(t, "../e2e/playwright.config.ts"), "retries: process.env.CI ? 1 : 0") {
		t.Error("playwright.config.ts does not retry once in CI")
	}
}

func TestE2ETaskUsesAGivenImageInsteadOfBuildingOne(t *testing.T) {
	cmds := taskCommands(t, "e2e")

	if !strings.Contains(cmds, "E2E_IMAGE") {
		t.Errorf("task e2e ignores E2E_IMAGE:\n%s", cmds)
	}
}

func TestMasterPushPublishesTheTestedImageBySHAWithoutLatest(t *testing.T) {
	w := ciWorkflow(t)
	img := w.job(t, "image")

	push := img.step(t, "push")
	if push.If != "github.event_name == 'push' && github.ref == 'refs/heads/master'" {
		t.Errorf("push if = %q, want only pushes to master", push.If)
	}

	for _, want := range []string{`ref="ghcr.io/dujina/spendulum:sha-${GITHUB_SHA}"`, `docker tag spendulum:e2e "$ref"`, `docker push "$ref"`, "digest="} {
		if !strings.Contains(push.Run, want) {
			t.Errorf("push step does not contain %q:\n%s", want, push.Run)
		}
	}

	if img.Outputs["digest"] != "${{ steps.push.outputs.digest }}" {
		t.Errorf("image outputs = %v, want digest from the push step", img.Outputs)
	}

	if img.Permissions["packages"] != "write" {
		t.Errorf("image permissions = %v, want packages: write", img.Permissions)
	}

	if strings.Contains(readFile(t, "../.github/workflows/ci.yml"), ":latest") {
		t.Error("ci.yml mentions a latest tag")
	}

	pushAt := slices.IndexFunc(img.Steps, func(s step) bool { return s.ID == "push" })
	e2eAt := slices.IndexFunc(img.Steps, func(s step) bool { return s.ID == "e2e" })
	if pushAt < e2eAt {
		t.Error("the image is pushed before e2e passes")
	}
}

func TestDeployJobCommitsTheDigestSeriallyWithTheDeployKey(t *testing.T) {
	w := ciWorkflow(t)
	d := w.job(t, "deploy")

	for _, need := range append([]string{"changes"}, filteredJobs()...) {
		if !slices.Contains(d.Needs, need) {
			t.Errorf("deploy does not need %s", need)
		}
	}

	if !strings.Contains(d.If, "github.event_name == 'push'") {
		t.Errorf("deploy if = %q, want only pushes", d.If)
	}

	if d.Concurrency == nil || d.Concurrency.Group != "deploy-prod" || fmt.Sprint(d.Concurrency.CancelInProgress) != "false" {
		t.Errorf("deploy concurrency = %+v, want group deploy-prod without cancel-in-progress", d.Concurrency)
	}

	checkout := d.uses("actions/checkout")
	if len(checkout) != 1 || with(checkout[0], "ssh-key") != "${{ secrets.DEPLOY_KEY }}" || with(checkout[0], "fetch-depth") != "0" {
		t.Errorf("deploy checkout = %+v, want the deploy key and full history", checkout)
	}

	commit := d.step(t, "commit")
	if !strings.Contains(commit.Run, ".github/scripts/deploy-prod.sh") || commit.Env["DIGEST"] != "${{ needs.image.outputs.digest }}" {
		t.Errorf("commit step = %+v, want deploy-prod.sh with the image digest", commit)
	}

	if !strings.Contains(readFile(t, "../.github/scripts/deploy-prod.sh"), `edit set image "$image:sha-$sha@$digest"`) {
		t.Error("deploy-prod.sh does not pin the image by tag and digest")
	}
}

func TestWorkflowRunsOnPullRequestsAndPushesToMaster(t *testing.T) {
	w := ciWorkflow(t)

	if _, ok := w.On["pull_request"]; !ok {
		t.Error("ci.yml does not run on pull_request")
	}

	push, _ := w.On["push"].(map[string]any)
	if fmt.Sprint(push["branches"]) != "[master]" {
		t.Errorf("on.push = %v, want branches [master]", w.On["push"])
	}

	if w.Concurrency == nil || w.Concurrency.CancelInProgress != "${{ github.event_name == 'pull_request' }}" {
		t.Errorf("concurrency = %+v, want superseded PR runs cancelled and master runs kept", w.Concurrency)
	}

	if len(w.Permissions) != 0 {
		t.Errorf("workflow permissions = %v, want none by default", w.Permissions)
	}
}

var pinnedAction = regexp.MustCompile(`^[\w.-]+/[\w./-]+@[0-9a-f]{40}$`)

func actionFiles(t *testing.T) []string {
	t.Helper()

	files, err := filepath.Glob("../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}

	actions, err := filepath.Glob("../.github/actions/*/action.yml")
	if err != nil {
		t.Fatal(err)
	}

	return append(files, actions...)
}

var usesLine = regexp.MustCompile(`(?m)^\s*(?:-\s*)?uses:\s*(\S+)(.*)$`)

func TestEveryActionIsPinnedByCommitSHA(t *testing.T) {
	for _, file := range actionFiles(t) {
		for _, m := range usesLine.FindAllStringSubmatch(readFile(t, file), -1) {
			ref, comment := m[1], strings.TrimSpace(m[2])
			if strings.HasPrefix(ref, "./") {
				continue
			}

			if !pinnedAction.MatchString(ref) {
				t.Errorf("%s: %s is not pinned by a full commit SHA", file, ref)
			}

			if !strings.HasPrefix(comment, "# v") {
				t.Errorf("%s: %s has no '# vX.Y.Z' comment naming the pinned release", file, ref)
			}
		}
	}
}

func TestEveryJobTimesOutAfter12Minutes(t *testing.T) {
	files, err := filepath.Glob("../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		var w workflow
		loadYAML(t, file, &w)

		for name, j := range w.Jobs {
			if j.TimeoutMinutes != 12 {
				t.Errorf("%s: job %s timeout-minutes = %d, want 12", file, name, j.TimeoutMinutes)
			}
		}
	}
}

type compositeAction struct {
	Runs struct {
		Steps []step
	}
}

func TestToolchainsComeFromTheRepositoryWithTheirCaches(t *testing.T) {
	var setup compositeAction
	loadYAML(t, "../.github/actions/setup/action.yml", &setup)
	j := job{Steps: setup.Runs.Steps}

	goStep := j.uses("actions/setup-go")
	if len(goStep) != 1 || with(goStep[0], "go-version-file") != "backend/go.mod" || !strings.Contains(with(goStep[0], "cache-dependency-path"), "backend/go.sum") {
		t.Errorf("setup-go = %+v, want go-version-file backend/go.mod and the go.sum caches", goStep)
	}

	nodeStep := j.uses("actions/setup-node")
	if len(nodeStep) != 1 || with(nodeStep[0], "node-version-file") != "web/.nvmrc" {
		t.Errorf("setup-node = %+v, want node-version-file web/.nvmrc", nodeStep)
	}

	if !strings.Contains(j.runs(), "corepack enable") {
		t.Error("pnpm does not come from corepack")
	}

	cache := j.uses("actions/cache")
	if len(cache) != 1 || !strings.Contains(with(cache[0], "key"), "hashFiles('pnpm-lock.yaml')") {
		t.Errorf("pnpm store cache = %+v, want one keyed on pnpm-lock.yaml", cache)
	}

	if len(j.uses("arduino/setup-task")) != 1 {
		t.Error("Task does not come from arduino/setup-task")
	}
}

func TestPRAgentIsPinnedAdvisoryAndSkipsDraftsAndBots(t *testing.T) {
	var w workflow
	loadYAML(t, "../.github/workflows/pr_agent.yml", &w)

	if len(w.Jobs) != 1 {
		t.Fatalf("pr_agent.yml has %d jobs, want 1", len(w.Jobs))
	}

	for _, j := range w.Jobs {
		for _, want := range []string{
			"github.event.pull_request.draft", "github.event.issue.draft",
			"github.event.pull_request.user.type != 'Bot'", "github.event.issue.user.type != 'Bot'",
		} {
			if !strings.Contains(j.If, want) {
				t.Errorf("pr_agent if = %q, want it to check %q", j.If, want)
			}
		}
	}

	for _, check := range rulesetChecks(t) {
		if check != "ci-ok" {
			t.Errorf("the ruleset requires %q; only ci-ok is required", check)
		}
	}
}

func TestWorkflowsPassActionlint(t *testing.T) {
	files, err := filepath.Glob("../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}

	// shellcheck and pyflakes are off so the result does not depend on what the machine has installed.
	cmd := exec.CommandContext(t.Context(), "go", append([]string{"tool", "actionlint", "-shellcheck=", "-pyflakes="}, files...)...)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("actionlint: %v\n%s", err, out)
	}
}
