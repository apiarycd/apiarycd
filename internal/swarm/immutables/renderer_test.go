package immutables_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apiarycd/apiarycd/internal/swarm/immutables"
	"go.yaml.in/yaml/v3"
)

func TestRenderVersionedCompose_RenamesFileBackedResources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	compose := `services:
  app:
    image: nginx
    configs:
      - app_cfg
    secrets:
      - source: app_secret
        target: /run/secrets/app_secret
configs:
  app_cfg:
    file: ./cfg.txt
secrets:
  app_secret:
    file: ./secret.txt
`

	mustWrite(t, filepath.Join(dir, "compose.yml"), compose)
	mustWrite(t, filepath.Join(dir, "cfg.txt"), "alpha")
	mustWrite(t, filepath.Join(dir, "secret.txt"), "beta")

	r := immutables.NewRenderer()
	renderedPath, rotations, err := r.RenderVersionedCompose("mystack", filepath.Join(dir, "compose.yml"))
	if err != nil {
		t.Fatalf("RenderVersionedCompose returned error: %v", err)
	}

	if len(rotations) != 2 {
		t.Fatalf("expected 2 rotations, got %d", len(rotations))
	}

	raw, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("read rendered compose: %v", err)
	}

	var rendered map[string]any
	if yamlErr := yaml.Unmarshal(raw, &rendered); yamlErr != nil {
		t.Fatalf("unmarshal rendered compose: %v", yamlErr)
	}

	configs := rendered["configs"].(map[string]any)
	secrets := rendered["secrets"].(map[string]any)
	if len(configs) != 1 || len(secrets) != 1 {
		t.Fatalf("expected exactly one config and one secret")
	}

	services := rendered["services"].(map[string]any)
	app := services["app"].(map[string]any)
	cfgRefs := app["configs"].([]any)
	secretRefs := app["secrets"].([]any)

	cfgRef, ok := cfgRefs[0].(map[string]any)
	if !ok {
		t.Fatalf("expected config reference to be a map, got %#v", cfgRefs[0])
	}
	cfgSource := cfgRef["source"].(string)
	if !strings.HasPrefix(cfgSource, "mystack_app_cfg__") {
		t.Fatalf("unexpected config reference source: %#v", cfgSource)
	}
	// Verify target is preserved (implicit mount target equals original logical name)
	if cfgRef["target"] != "app_cfg" {
		t.Fatalf("expected config target to be 'app_cfg', got %#v", cfgRef["target"])
	}

	secretRef := secretRefs[0].(map[string]any)
	secretSource := secretRef["source"].(string)
	if !strings.HasPrefix(secretSource, "mystack_app_secret__") {
		t.Fatalf("unexpected secret reference source: %#v", secretRefs[0])
	}
	// Verify explicit target is preserved
	if secretRef["target"] != "/run/secrets/app_secret" {
		t.Fatalf("expected secret target to be '/run/secrets/app_secret', got %#v", secretRef["target"])
	}
}

func TestRenderVersionedCompose_LeavesExternalResourcesUntouched(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	compose := `services:
  app:
    image: nginx
    configs:
      - ext_cfg
configs:
  ext_cfg:
    external: true
`
	mustWrite(t, filepath.Join(dir, "compose.yml"), compose)

	r := immutables.NewRenderer()
	renderedPath, rotations, err := r.RenderVersionedCompose("mystack", filepath.Join(dir, "compose.yml"))
	if err != nil {
		t.Fatalf("RenderVersionedCompose returned error: %v", err)
	}
	if len(rotations) != 0 {
		t.Fatalf("expected no rotations for external resource, got %d", len(rotations))
	}

	raw, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("read rendered compose: %v", err)
	}
	if !strings.Contains(string(raw), "ext_cfg") {
		t.Fatalf("expected external resource name to stay unchanged")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
