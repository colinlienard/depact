package resolver

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
)

type PkgJSON struct {
	Main    string          `json:"main"`
	Exports json.RawMessage `json:"exports"`
}

var ErrPkgNotFound = errors.New("resolver: package not found")
var ErrPkgNoEntries = errors.New("resolver: found no entries in package")

func (r *Resolver) resolvePkgEntry(specifier string) (string, error) {
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

	}

	if data.Main != "" {
		return path.Join("node_modules", specifier, data.Main), nil
	}

	return "", ErrPkgNoEntries
}
