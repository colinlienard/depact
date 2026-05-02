package resolver

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strings"
)

type PkgJSON struct {
	Main    string          `json:"main"`
	Exports json.RawMessage `json:"exports"`
}

var ErrPkgNotFound = errors.New("resolver: package not found")
var ErrPkgNoEntries = errors.New("resolver: found no entries in package")

func (r *Resolver) resolvePkgEntry(specifier string) (string, error) {
	subpath := ""
	numberOfSlash := strings.Count(specifier, "/")
	isScopedPkg := specifier[0] == '@'
	if !isScopedPkg && numberOfSlash > 0 {
		specifier, subpath, _ = strings.Cut(specifier, "/")
	} else if isScopedPkg && numberOfSlash > 1 {
		splited := strings.Split(specifier, "/")
		specifier = strings.Join(splited[0:2], "/")
		subpath = strings.Join(splited[2:], "/")
	}

	pkgPath := path.Join("node_modules", specifier, "package.json")

	_, err := r.stat(pkgPath)
	if err != nil {
		return "", ErrPkgNotFound
	}

	content, err := fs.ReadFile(r.fs, pkgPath)
	if err != nil {
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
			return strings.Replace(target, "*", match, 1), nil
		}
		return "", ErrPkgNotFound
	}
	if subpath != "" {
		return "", ErrPkgNotFound
	}
	return r.resolveConditions(exports)
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

var conditions = []string{"types", "import", "default"}

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

	for _, cond := range conditions {
		if entry, ok := obj[cond]; ok {
			return r.resolveConditions(entry)
		}
	}
	return "", ErrPkgNotFound
}
