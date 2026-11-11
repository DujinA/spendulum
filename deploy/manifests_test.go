package deploy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

func build(t *testing.T, dir string) resmap.ResMap {
	t.Helper()

	m, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(filesys.MakeFsOnDisk(), dir)
	if err != nil {
		t.Fatalf("kustomize build %s: %v", dir, err)
	}

	return m
}

func ofKind(t *testing.T, m resmap.ResMap, kind string) [][]byte {
	t.Helper()

	var found [][]byte

	for _, r := range m.Resources() {
		if r.GetKind() != kind {
			continue
		}

		b, err := r.AsYAML()
		if err != nil {
			t.Fatal(err)
		}

		found = append(found, b)
	}

	return found
}

func decode(t *testing.T, m resmap.ResMap, kind string, out any) {
	t.Helper()

	found := ofKind(t, m, kind)
	if len(found) != 1 {
		t.Fatalf("want 1 %s, got %d", kind, len(found))
	}

	if err := yaml.UnmarshalStrict(found[0], out); err != nil {
		t.Fatalf("decode %s: %v", kind, err)
	}
}

func baseDeployment(t *testing.T) appsv1.Deployment {
	t.Helper()

	var d appsv1.Deployment
	decode(t, build(t, "base"), "Deployment", &d)

	if n := len(d.Spec.Template.Spec.Containers); n != 1 {
		t.Fatalf("want 1 container, got %d", n)
	}

	return d
}

func TestBaseDeploymentRunsOneHardenedReplica(t *testing.T) {
	d := baseDeployment(t)
	pod := d.Spec.Template.Spec
	c := pod.Containers[0]

	if d.Spec.Replicas == nil || *d.Spec.Replicas != 1 {
		t.Errorf("replicas = %v, want 1", d.Spec.Replicas)
	}

	if pod.SecurityContext == nil || pod.SecurityContext.RunAsNonRoot == nil || !*pod.SecurityContext.RunAsNonRoot {
		t.Error("pod must set runAsNonRoot: true")
	}

	if c.SecurityContext == nil || c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
		t.Error("container must set readOnlyRootFilesystem: true")
	}

	if got := c.Resources.Requests.Memory().String(); got != "128Mi" {
		t.Errorf("memory request = %s, want 128Mi", got)
	}

	if got := c.Resources.Limits.Memory().String(); got != "256Mi" {
		t.Errorf("memory limit = %s, want 256Mi", got)
	}
}

func TestBaseDeploymentProbesHealthz(t *testing.T) {
	c := baseDeployment(t).Spec.Template.Spec.Containers[0]

	for name, p := range map[string]*corev1.Probe{"liveness": c.LivenessProbe, "readiness": c.ReadinessProbe} {
		if p == nil || p.HTTPGet == nil {
			t.Errorf("%s probe must be an httpGet probe", name)
			continue
		}

		if p.HTTPGet.Path != "/healthz" || p.HTTPGet.Port.String() != "http" {
			t.Errorf("%s probe = %s on port %s, want /healthz on port http", name, p.HTTPGet.Path, p.HTTPGet.Port.String())
		}
	}
}

func TestBaseServiceIsNodePort30080(t *testing.T) {
	var s corev1.Service
	decode(t, build(t, "base"), "Service", &s)

	if s.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("type = %s, want NodePort", s.Spec.Type)
	}

	if len(s.Spec.Ports) != 1 {
		t.Fatalf("want 1 port, got %d", len(s.Spec.Ports))
	}

	p := s.Spec.Ports[0]
	if p.NodePort != 30080 || p.TargetPort.String() != "http" {
		t.Errorf("port = nodePort %d to %s, want nodePort 30080 to http", p.NodePort, p.TargetPort.String())
	}

	if s.Spec.Selector["app.kubernetes.io/name"] != "spendulum" {
		t.Errorf("selector = %v, want app.kubernetes.io/name: spendulum", s.Spec.Selector)
	}
}

func TestDatabaseURLComesFromCloudNativePGAppSecret(t *testing.T) {
	c := baseDeployment(t).Spec.Template.Spec.Containers[0]

	if len(c.Env) != 1 || c.Env[0].Name != "SPENDULUM_DATABASE_URL" {
		t.Fatalf("env = %v, want only SPENDULUM_DATABASE_URL; other settings belong in the ConfigMap", c.Env)
	}

	ref := c.Env[0].ValueFrom
	if ref == nil || ref.SecretKeyRef == nil {
		t.Fatal("SPENDULUM_DATABASE_URL must come from a secretKeyRef")
	}

	if ref.SecretKeyRef.Name != "spendulum-db-app" || ref.SecretKeyRef.Key != "uri" {
		t.Errorf("secretKeyRef = %s/%s, want spendulum-db-app/uri", ref.SecretKeyRef.Name, ref.SecretKeyRef.Key)
	}
}

func TestOtherSettingsComeFromConfigMap(t *testing.T) {
	m := build(t, "base")

	var d appsv1.Deployment
	decode(t, m, "Deployment", &d)

	var cm corev1.ConfigMap
	decode(t, m, "ConfigMap", &cm)

	from := d.Spec.Template.Spec.Containers[0].EnvFrom
	if len(from) != 1 || from[0].ConfigMapRef == nil || from[0].ConfigMapRef.Name != cm.Name {
		t.Fatalf("envFrom = %v, want the ConfigMap %s", from, cm.Name)
	}

	if cm.Data["SPENDULUM_HTTP_ADDR"] != ":8080" {
		t.Errorf("SPENDULUM_HTTP_ADDR = %q, want :8080 to match the container port", cm.Data["SPENDULUM_HTTP_ADDR"])
	}

	if _, ok := cm.Data["SPENDULUM_DATABASE_URL"]; ok {
		t.Error("the ConfigMap must not hold SPENDULUM_DATABASE_URL")
	}
}

func TestProdOverlayTargetsSpendulumNamespaceAndGHCRImage(t *testing.T) {
	m := build(t, "overlays/prod")

	for _, r := range m.Resources() {
		if ns := r.GetNamespace(); ns != "spendulum" {
			t.Errorf("%s/%s namespace = %q, want spendulum", r.GetKind(), r.GetName(), ns)
		}
	}

	var d appsv1.Deployment
	decode(t, m, "Deployment", &d)

	if got := d.Spec.Template.Spec.Containers[0].Image; got != "ghcr.io/dujina/spendulum:placeholder" {
		t.Errorf("image = %s, want ghcr.io/dujina/spendulum:placeholder", got)
	}
}

func TestProdImageIsReplacedByKustomizeEditSetImage(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(".")); err != nil {
		t.Fatal(err)
	}

	const image = "ghcr.io/dujina/spendulum:sha-0123456789abcdef0123456789abcdef01234567@sha256:" +
		"0000000000000000000000000000000000000000000000000000000000000000"

	cmd := exec.CommandContext(t.Context(), "go", "tool", "kustomize", "edit", "set", "image", image)
	cmd.Dir = filepath.Join(root, "overlays", "prod")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("kustomize edit set image: %v\n%s", err, out)
	}

	var d appsv1.Deployment
	decode(t, build(t, filepath.Join(root, "overlays", "prod")), "Deployment", &d)

	if got := d.Spec.Template.Spec.Containers[0].Image; got != image {
		t.Errorf("image = %s, want %s", got, image)
	}
}

type application struct {
	Metadata struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Source struct {
			RepoURL        string `json:"repoURL"`
			TargetRevision string `json:"targetRevision"`
			Path           string `json:"path"`
		} `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
		SyncPolicy struct {
			Automated *struct {
				Prune    bool `json:"prune"`
				SelfHeal bool `json:"selfHeal"`
			} `json:"automated"`
		} `json:"syncPolicy"`
	} `json:"spec"`
}

func applications(t *testing.T) map[string]application {
	t.Helper()

	m := build(t, "argocd")
	apps := map[string]application{}

	for _, b := range ofKind(t, m, "Application") {
		var a application
		if err := yaml.Unmarshal(b, &a); err != nil {
			t.Fatal(err)
		}

		apps[a.Metadata.Name] = a
	}

	if len(apps) != m.Size() {
		t.Errorf("deploy/argocd renders %d resources but %d Applications; it should hold only Applications", m.Size(), len(apps))
	}

	return apps
}

func TestArgoCDHasRootAndProdAppWithAutomatedSync(t *testing.T) {
	apps := applications(t)

	for name, path := range map[string]string{"root": "deploy/argocd", "spendulum": "deploy/overlays/prod"} {
		a, ok := apps[name]
		if !ok {
			t.Errorf("no Application %s", name)
			continue
		}

		if a.Spec.Source.Path != path {
			t.Errorf("%s path = %s, want %s", name, a.Spec.Source.Path, path)
		}

		if a.Metadata.Namespace != "argocd" {
			t.Errorf("%s namespace = %q, want argocd", a.Metadata.Name, a.Metadata.Namespace)
		}

		if a.Spec.Source.RepoURL != "https://github.com/DujinA/spendulum.git" || a.Spec.Source.TargetRevision != "master" {
			t.Errorf("%s source = %s@%s, want the spendulum repository at master", a.Metadata.Name, a.Spec.Source.RepoURL, a.Spec.Source.TargetRevision)
		}

		if auto := a.Spec.SyncPolicy.Automated; auto == nil || !auto.Prune || !auto.SelfHeal {
			t.Errorf("%s must sync automatically with prune and selfHeal", a.Metadata.Name)
		}
	}

	if ns := apps["spendulum"].Spec.Destination.Namespace; ns != "spendulum" {
		t.Errorf("spendulum destination namespace = %q, want spendulum", ns)
	}
}

func TestArgoCDChildApplicationsHaveSyncWaves(t *testing.T) {
	for name, a := range applications(t) {
		if name == "root" {
			continue
		}

		w, ok := a.Metadata.Annotations["argocd.argoproj.io/sync-wave"]
		if !ok {
			t.Errorf("%s has no sync-wave annotation", a.Metadata.Name)
			continue
		}

		if _, err := strconv.Atoi(w); err != nil {
			t.Errorf("%s sync-wave = %q, want an integer", a.Metadata.Name, w)
		}
	}
}

func TestNoKustomizationRendersASecret(t *testing.T) {
	for _, dir := range []string{"base", "overlays/prod", "argocd"} {
		for _, r := range build(t, dir).Resources() {
			if r.GetKind() == "Secret" {
				t.Errorf("%s renders Secret %s; secrets come from CloudNativePG or a SealedSecret", dir, r.GetName())
			}
		}
	}
}
