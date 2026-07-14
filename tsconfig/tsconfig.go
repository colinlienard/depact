package tsconfig

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strings"
)

type Config struct {
	BaseURL                    string
	Paths                      map[string][]string
	ModuleResolution           string
	AllowImportingTsExtensions bool
	References                 []string
}

type rawConfig struct {
	Extends         json.RawMessage `json:"extends"`
	CompilerOptions struct {
		BaseURL                    *string             `json:"baseUrl"`
		Paths                      map[string][]string `json:"paths"`
		ModuleResolution           *string             `json:"moduleResolution"`
		AllowImportingTsExtensions *bool               `json:"allowImportingTsExtensions"`
	} `json:"compilerOptions"`
	References []struct {
		Path string `json:"path"`
	} `json:"references"`
}

func Load(fsys fs.FS, p string) (*Config, error) {
	cfg := &Config{}
	if err := load(fsys, p, cfg, map[string]bool{}, true); err != nil {
		return nil, err
	}
	return cfg, nil
}

func load(fsys fs.FS, p string, cfg *Config, seen map[string]bool, root bool) error {
	if seen[p] {
		return errors.New("tsconfig: extends cycle detected at " + p)
	}
	seen[p] = true
	defer delete(seen, p)

	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		return err
	}
	data = stripJSONC(data)

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if len(raw.Extends) > 0 {
		targets, err := parseExtends(raw.Extends)
		if err != nil {
			return err
		}
		for _, target := range targets {
			if err := load(fsys, resolveExtends(p, target), cfg, seen, false); err != nil {
				return err
			}
		}
	}

	co := raw.CompilerOptions
	if co.BaseURL != nil {
		cfg.BaseURL = *co.BaseURL
	}
	if co.Paths != nil {
		cfg.Paths = co.Paths
	}
	if co.ModuleResolution != nil {
		cfg.ModuleResolution = *co.ModuleResolution
	}
	if co.AllowImportingTsExtensions != nil {
		cfg.AllowImportingTsExtensions = *co.AllowImportingTsExtensions
	}

	if root {
		for _, ref := range raw.References {
			cfg.References = append(cfg.References, resolveReference(p, ref.Path))
		}
	}

	return nil
}

func resolveReference(from, refPath string) string {
	resolved := path.Join(path.Dir(from), refPath)
	if !strings.HasSuffix(resolved, ".json") {
		resolved = path.Join(resolved, "tsconfig.json")
	}
	return resolved
}

func parseExtends(raw json.RawMessage) ([]string, error) {
	if raw[0] == '[' {
		var arr []string
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return []string{s}, nil
}

func resolveExtends(from, target string) string {
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") {
		resolved := path.Join(path.Dir(from), target)
		if !strings.HasSuffix(resolved, ".json") {
			resolved += ".json"
		}
		return resolved
	}

	pkg, subpath := splitPackageSpecifier(target)
	if subpath == "" {
		return path.Join("node_modules", pkg, "tsconfig.json")
	}
	resolved := path.Join("node_modules", pkg, subpath)
	if !strings.HasSuffix(resolved, ".json") {
		resolved += ".json"
	}
	return resolved
}

func splitPackageSpecifier(specifier string) (pkg, subpath string) {
	parts := strings.Split(specifier, "/")
	n := 1
	if strings.HasPrefix(specifier, "@") {
		n = 2
	}
	if len(parts) <= n {
		return specifier, ""
	}
	return strings.Join(parts[:n], "/"), strings.Join(parts[n:], "/")
}

func stripJSONC(data []byte) []byte {
	var out []byte
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			i, out = copyString(data, i, out)
			continue
		}
		if data[i] == '/' && i+1 < len(data) {
			if data[i+1] == '/' {
				i += 2
				for i < len(data) && data[i] != '\n' {
					i++
				}
				i-- // let the loop re-emit the newline
				continue
			}
			if data[i+1] == '*' {
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					i++
				}
				i++ // skip past the closing '/'
				continue
			}
		}
		out = append(out, data[i])
	}
	return stripTrailingCommas(out)
}

func stripTrailingCommas(data []byte) []byte {
	var out []byte
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			i, out = copyString(data, i, out)
			continue
		}
		if data[i] == ',' {
			j := i + 1
			for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue // drop the trailing comma
			}
		}
		out = append(out, data[i])
	}
	return out
}

func copyString(data []byte, i int, out []byte) (int, []byte) {
	out = append(out, data[i]) // opening quote
	for i++; i < len(data); i++ {
		out = append(out, data[i])
		if data[i] == '\\' && i+1 < len(data) {
			i++
			out = append(out, data[i])
			continue
		}
		if data[i] == '"' {
			break
		}
	}
	return i, out
}
