package deploy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	deployEmail = "deploy@example.invalid"
	fakeDigest  = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
)

type deployRepo struct {
	origin string
	work   string
}

// newDeployRepo makes a bare origin with the prod overlay on master and a full clone of it.
func newDeployRepo(t *testing.T) deployRepo {
	t.Helper()

	root := t.TempDir()
	r := deployRepo{origin: filepath.Join(root, "origin.git"), work: filepath.Join(root, "work")}

	run(t, root, "git", "init", "--quiet", "--bare", "--initial-branch=master", r.origin)

	seed := filepath.Join(root, "seed")
	run(t, root, "git", "clone", "--quiet", r.origin, seed)
	writeFile(t, filepath.Join(seed, "deploy", "overlays", "prod", "kustomization.yaml"), readFile(t, "overlays/prod/kustomization.yaml"))
	commitAll(t, seed, "feat: first")
	run(t, seed, "git", "push", "--quiet", "origin", "HEAD:master")

	run(t, root, "git", "clone", "--quiet", r.origin, r.work)

	return r
}

// pushCommit lands a commit on origin's master from another clone and returns its sha.
func (r deployRepo) pushCommit(t *testing.T, file string) string {
	t.Helper()

	other := filepath.Join(t.TempDir(), "other")
	run(t, "", "git", "clone", "--quiet", r.origin, other)
	writeFile(t, filepath.Join(other, file), file)
	commitAll(t, other, "feat: "+file)
	run(t, other, "git", "push", "--quiet", "origin", "HEAD:master")

	return strings.TrimSpace(run(t, other, "git", "rev-parse", "HEAD"))
}

func (r deployRepo) deploy(t *testing.T, sha string) (string, error) {
	t.Helper()

	script, err := filepath.Abs("../.github/scripts/deploy-prod.sh")
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.CommandContext(t.Context(), script, sha, fakeDigest)
	cmd.Dir = r.work
	cmd.Env = append(os.Environ(), "DEPLOY_EMAIL="+deployEmail, "KUSTOMIZE="+kustomize(t))
	out, err := cmd.CombinedOutput()

	return string(out), err
}

func (r deployRepo) masterLog(t *testing.T) []string {
	t.Helper()

	return strings.Split(strings.TrimSpace(run(t, r.origin, "git", "log", "--format=%s|%ae", "master")), "\n")
}

func (r deployRepo) prodImage(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "check")
	run(t, "", "git", "clone", "--quiet", r.origin, dir)

	return strings.TrimSpace(run(t, filepath.Join(dir, "deploy", "overlays", "prod"), "grep", "-E", "newTag|digest", "kustomization.yaml"))
}

func TestDeployCommitsThePinnedImageToTheProdOverlayAsTheDeployIdentity(t *testing.T) {
	r := newDeployRepo(t)
	sha := r.pushCommit(t, "backend/feature.go")

	if out, err := r.deploy(t, sha); err != nil {
		t.Fatalf("deploy-prod.sh: %v\n%s", err, out)
	}

	if got, want := r.masterLog(t)[0], "deploy(prod): "+sha[:7]+"|"+deployEmail; got != want {
		t.Errorf("master head = %q, want %q", got, want)
	}

	image := r.prodImage(t)
	for _, want := range []string{"newTag: sha-" + sha, "digest: " + fakeDigest} {
		if !strings.Contains(image, want) {
			t.Errorf("prod overlay image fields:\n%s\nwant %q", image, want)
		}
	}
}

func TestDeployRebasesAndRetriesWhenMasterMovedDuringThePush(t *testing.T) {
	r := newDeployRepo(t)
	sha := r.pushCommit(t, "backend/feature.go")

	// The first push is rejected after another merge lands, as in a race with a PR merge.
	hook := filepath.Join(r.origin, "hooks", "pre-receive")
	marker := filepath.Join(t.TempDir(), "rejected-once")
	writeFile(t, hook, "#!/bin/sh\n[ -e '"+marker+"' ] && exit 0\ntouch '"+marker+"'\necho 'master moved' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}

	r.pushCommitBypassingHook(t, marker, "web/feature.ts")

	if out, err := r.deploy(t, sha); err != nil {
		t.Fatalf("deploy-prod.sh: %v\n%s", err, out)
	}

	log := r.masterLog(t)
	if len(log) < 2 || !strings.HasPrefix(log[0], "deploy(prod): "+sha[:7]) || log[1] != "feat: web/feature.ts|test@example.invalid" {
		t.Errorf("master log = %q, want the deploy commit on top of the moved master", log)
	}
}

func TestDeploySkipsWhenANewerCommitIsAlreadyDeployed(t *testing.T) {
	r := newDeployRepo(t)
	older := r.pushCommit(t, "backend/older.go")
	newer := r.pushCommit(t, "backend/newer.go")

	if out, err := r.deploy(t, newer); err != nil {
		t.Fatalf("deploy newer: %v\n%s", err, out)
	}

	if out, err := r.deploy(t, older); err != nil {
		t.Fatalf("deploy older: %v\n%s", err, out)
	}

	if got := r.masterLog(t)[0]; !strings.HasPrefix(got, "deploy(prod): "+newer[:7]) {
		t.Errorf("master head = %q, want the newer deploy to stay", got)
	}

	if image := r.prodImage(t); !strings.Contains(image, "sha-"+newer) {
		t.Errorf("prod overlay image fields:\n%s\nwant sha-%s", image, newer)
	}
}

func (r deployRepo) pushCommitBypassingHook(t *testing.T, marker, file string) {
	t.Helper()

	writeFile(t, marker, "")
	r.pushCommit(t, file)

	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
}

func kustomize(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "kustomize")
	run(t, "", "go", "build", "-o", bin, "sigs.k8s.io/kustomize/kustomize/v5")

	return bin
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()

	run(t, dir, "git", "add", "--all")
	run(t, dir, "git", "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "--message", msg)
}

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}

	return string(out)
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
