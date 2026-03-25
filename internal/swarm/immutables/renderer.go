package immutables

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const digestLen = 8

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) RenderVersionedCompose(stackName, composePath string) (string, []string, error) {
	raw, err := os.ReadFile(composePath)
	if err != nil {
		return "", nil, fmt.Errorf("read compose file: %w", err)
	}

	var compose map[string]any
	if yamlErr := yaml.Unmarshal(raw, &compose); yamlErr != nil {
		return "", nil, fmt.Errorf("unmarshal compose file: %w", yamlErr)
	}

	composeDir := filepath.Dir(composePath)

	configRenames, err := rotateSection(compose, "configs", stackName, composeDir)
	if err != nil {
		return "", nil, err
	}
	secretRenames, err := rotateSection(compose, "secrets", stackName, composeDir)
	if err != nil {
		return "", nil, err
	}

	rewriteServiceRefs(compose, "configs", configRenames)
	rewriteServiceRefs(compose, "secrets", secretRenames)

	out, err := yaml.Marshal(compose)
	if err != nil {
		return "", nil, fmt.Errorf("marshal rendered compose file: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(composePath), ".apiarycd-compose-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary compose file: %w", err)
	}
	defer tmp.Close()

	if _, writeErr := tmp.Write(out); writeErr != nil {
		_ = os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("write temporary compose file: %w", writeErr)
	}

	rotated := make([]string, 0, len(configRenames)+len(secretRenames))
	for from, to := range configRenames {
		rotated = append(rotated, fmt.Sprintf("config %s -> %s", from, to))
	}
	for from, to := range secretRenames {
		rotated = append(rotated, fmt.Sprintf("secret %s -> %s", from, to))
	}

	return tmp.Name(), rotated, nil
}

func rotateSection(compose map[string]any, section, stackName, composeDir string) (map[string]string, error) {
	rawSection, ok := compose[section]
	if !ok {
		return map[string]string{}, nil
	}

	resources, ok := rawSection.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidComposeSectionType, section)
	}

	renames := make(map[string]string)
	updated := make(map[string]any, len(resources))
	for logicalName, rawDef := range resources {
		def, isMap := rawDef.(map[string]any)
		if !isMap {
			updated[logicalName] = rawDef
			continue
		}

		if isExternal(def) {
			updated[logicalName] = rawDef
			continue
		}

		filePath, isString := def["file"].(string)
		if !isString || strings.TrimSpace(filePath) == "" {
			updated[logicalName] = rawDef
			continue
		}

		content, err := os.ReadFile(filepath.Clean(filepath.Join(composeDir, filePath)))
		if err != nil {
			return nil, fmt.Errorf("read %s %q file %q: %w", section, logicalName, filePath, err)
		}

		versioned := versionedName(stackName, logicalName, content)
		renames[logicalName] = versioned

		updatedDef := cloneMap(def)
		updatedDef["name"] = versioned

		if _, exists := updated[versioned]; exists {
			return nil, fmt.Errorf(
				"%w in %s: generated %q from %q conflicts with existing key",
				ErrResourceNameCollision,
				section, versioned, logicalName,
			)
		}

		updated[versioned] = updatedDef
	}

	compose[section] = updated
	return renames, nil
}

func rewriteServiceRefs(compose map[string]any, section string, renames map[string]string) {
	if len(renames) == 0 {
		return
	}

	services, ok := getServices(compose)
	if !ok {
		return
	}

	for serviceName, rawSvc := range services {
		updatedSvc := rewriteServiceRef(rawSvc, section, renames)
		if updatedSvc != nil {
			services[serviceName] = updatedSvc
		}
	}

	compose["services"] = services
}

func getServices(compose map[string]any) (map[string]any, bool) {
	rawServices, ok := compose["services"]
	if !ok {
		return nil, false
	}

	services, ok := rawServices.(map[string]any)
	if !ok {
		return nil, false
	}

	return services, true
}

func rewriteServiceRef(rawSvc any, section string, renames map[string]string) any {
	svc, isMap := rawSvc.(map[string]any)
	if !isMap {
		return nil
	}

	rawRefs, ok := svc[section]
	if !ok {
		return rawSvc
	}

	refs, ok := rawRefs.([]any)
	if !ok {
		return rawSvc
	}

	updatedRefs := rewriteRefs(refs, renames)
	if updatedRefs == nil {
		return rawSvc
	}

	updatedSvc := cloneMap(svc)
	updatedSvc[section] = updatedRefs
	return updatedSvc
}

func rewriteRefs(refs []any, renames map[string]string) []any {
	changed := false
	updated := make([]any, len(refs))
	for i, item := range refs {
		updatedItem, wasChanged := rewriteRef(item, renames)
		updated[i] = updatedItem
		if wasChanged {
			changed = true
		}
	}

	if changed {
		return updated
	}
	return nil
}

func rewriteRef(item any, renames map[string]string) (any, bool) {
	switch v := item.(type) {
	case string:
		if to, hasRename := renames[v]; hasRename {
			// Return long-syntax map preserving implicit mount target
			return map[string]any{
				"source": to,
				"target": v,
			}, true
		}
		return v, false
	case map[string]any:
		source, _ := v["source"].(string)
		if to, hasSource := renames[source]; hasSource {
			updated := cloneMap(v)
			updated["source"] = to
			// Ensure target is set to original source if missing or empty
			if target, ok := updated["target"]; !ok || target == "" {
				updated["target"] = source
			}
			return updated, true
		}
		return v, false
	default:
		return item, false
	}
}

func isExternal(def map[string]any) bool {
	v, ok := def["external"]
	if !ok {
		return false
	}

	switch x := v.(type) {
	case bool:
		return x
	case map[string]any:
		// Any non-empty map indicates an external resource
		return len(x) > 0
	default:
		// Fail-safe: unknown types are not external
		return false
	}
}

func versionedName(stackName, logicalName string, content []byte) string {
	h := sha256.Sum256(content)
	digest := hex.EncodeToString(h[:])
	return fmt.Sprintf("%s_%s__%s", stackName, logicalName, digest[:digestLen])
}

func cloneMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
