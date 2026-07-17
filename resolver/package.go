package resolver

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
)

type PkgJSON struct {
	Main    string          `json:"main"`
	Exports json.RawMessage `json:"exports"`
	Imports json.RawMessage `json:"imports"`
}

var ErrPkgNotFound = errors.New("resolver: package not found")
var ErrPkgNoEntries = errors.New("resolver: found no entries in package")

func (r *Resolver) resolvePkgEntry(specifier string) (string, error) {
	if specifier == "" {
		return "", ErrPkgNotFound
	}

	specifier, subpath := splitPkg(specifier)

	pkgPath := path.Join("node_modules", specifier, "package.json")

	content, err := fs.ReadFile(r.fs, pkgPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", ErrPkgNotFound
		}
		return "", err
	}

	var data PkgJSON
	if err := json.Unmarshal(content, &data); err != nil {
		return "", err
	}

	if len(data.Exports) > 0 {
		export, err := r.resolvePkgExports(data.Exports, subpath)
		if err != nil {
			return "", err
		}
		return path.Join("node_modules", specifier, export), nil
	}

	if data.Main != "" {
		return path.Join("node_modules", specifier, data.Main), nil
	}

	return "", ErrPkgNoEntries
}

func (r *Resolver) isWorkspaceLink(name string) bool {
	r.mu.Lock()
	if v, ok := r.links[name]; ok {
		r.mu.Unlock()
		return v
	}
	r.mu.Unlock()

	v := r.resolveWorkspaceLink(name)

	r.mu.Lock()
	r.links[name] = v
	r.mu.Unlock()
	return v
}

func (r *Resolver) resolveWorkspaceLink(name string) bool {
	root := path.Join("node_modules", name)
	target, err := fs.ReadLink(r.fs, root)
	if err != nil {
		return false
	}
	if !path.IsAbs(target) {
		target = path.Join(path.Dir(root), target)
	}
	if slices.Contains(strings.Split(path.Clean(target), "/"), "node_modules") {
		return false
	}
	return true
}

func splitPkg(specifier string) (name, subpath string) {
	isScoped := specifier != "" && specifier[0] == '@'
	slashes := strings.Count(specifier, "/")
	if !isScoped && slashes > 0 {
		name, subpath, _ = strings.Cut(specifier, "/")
		return name, subpath
	}
	if isScoped && slashes > 1 {
		parts := strings.Split(specifier, "/")
		return strings.Join(parts[:2], "/"), strings.Join(parts[2:], "/")
	}
	return specifier, ""
}

func pkgName(specifier string) string {
	name, _ := splitPkg(specifier)
	return name
}

func (r *Resolver) resolvePkgExports(exports json.RawMessage, subpath string) (string, error) {
	if exports[0] == '"' {
		if subpath != "" {
			return "", ErrPkgNotFound
		}
		var s string
		if err := json.Unmarshal(exports, &s); err != nil {
			return "", err
		}
		return s, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(exports, &obj); err != nil {
		return "", err
	}
	if isSubpathMap(obj) {
		key := subpath
		if key == "" {
			key = "."
		} else {
			key = "./" + key
		}
		if entry, ok := obj[key]; ok {
			return r.resolveConditions(entry)
		}
		if entry, match, ok := matchPattern(obj, key); ok {
			target, err := r.resolveConditions(entry)
			if err != nil {
				return "", err
			}
			return strings.ReplaceAll(target, "*", match), nil
		}
		return "", ErrPkgNotFound
	}
	if subpath != "" {
		return "", ErrPkgNotFound
	}
	return r.resolveConditions(exports)
}

func (r *Resolver) resolvePkgImports(from, specifier string) (string, error) {
	pkgDir, pkg, err := r.findEnclosingPkg(from)
	if err != nil {
		return "", err
	}
	if len(pkg.Imports) == 0 {
		return "", ErrPkgNotFound
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(pkg.Imports, &obj); err != nil {
		return "", err
	}

	if entry, ok := obj[specifier]; ok {
		target, err := r.resolveConditions(entry)
		if err != nil {
			return "", err
		}
		return path.Join(pkgDir, target), nil
	}

	if entry, match, ok := matchPattern(obj, specifier); ok {
		target, err := r.resolveConditions(entry)
		if err != nil {
			return "", err
		}
		return path.Join(pkgDir, strings.ReplaceAll(target, "*", match)), nil
	}

	return "", ErrPkgNotFound
}

func (r *Resolver) findEnclosingPkg(from string) (string, *PkgJSON, error) {
	dir := path.Dir(from)
	for {
		p := path.Join(dir, "package.json")
		if r.exists(p) {
			content, err := fs.ReadFile(r.fs, p)
			if err != nil {
				return "", nil, err
			}
			var pkg PkgJSON
			if err := json.Unmarshal(content, &pkg); err != nil {
				return "", nil, err
			}
			return dir, &pkg, nil
		}
		if dir == "." || dir == "/" {
			return "", nil, ErrPkgNotFound
		}
		dir = path.Dir(dir)
	}
}

func matchPattern(obj map[string]json.RawMessage, key string) (json.RawMessage, string, bool) {
	var bestKey, bestMatch string
	var bestEntry json.RawMessage
	found := false
	for k, v := range obj {
		prefix, suffix, ok := strings.Cut(k, "*")
		if !ok {
			continue
		}
		if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
			continue
		}
		if len(key) < len(prefix)+len(suffix) {
			continue
		}
		if found && len(prefix) <= len(bestKey) {
			continue
		}
		bestKey = prefix
		bestMatch = key[len(prefix) : len(key)-len(suffix)]
		bestEntry = v
		found = true
	}
	return bestEntry, bestMatch, found
}

func isSubpathMap(obj map[string]json.RawMessage) bool {
	for k := range obj {
		return strings.HasPrefix(k, ".")
	}
	return false
}

var runtimeConditions = []string{"import", "require", "default", "types"}
var typeConditions = []string{"types", "import", "require", "default"}

func (r *Resolver) resolveConditions(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", ErrPkgNotFound
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", err
	}

	for _, cond := range r.conditions() {
		if entry, ok := obj[cond]; ok {
			return r.resolveConditions(entry)
		}
	}
	return "", ErrPkgNotFound
}

func (r *Resolver) conditions() []string {
	if r.IncludeTypes {
		return typeConditions
	}
	return runtimeConditions
}
