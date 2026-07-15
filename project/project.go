package project

import (
	"errors"
	"io/fs"
	"os"
	"path"

	"depact/resolver"
	"depact/tsconfig"
	"depact/walker"
)

type Project struct {
	FS       fs.FS
	Config   *tsconfig.Config
	Resolver *resolver.Resolver
	Walker   *walker.Walker
}

func Load(fsys fs.FS, tsconfigPath string) (*Project, error) {
	cfgPath := tsconfigPath
	if cfgPath == "" {
		cfgPath = "tsconfig.json"
	}
	cfg, err := tsconfig.Load(fsys, cfgPath)
	if err != nil {
		if tsconfigPath != "" || !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		cfg = &tsconfig.Config{}
	}
	r := resolver.New(fsys, rebasePaths(cfgPath, cfg))
	return &Project{FS: fsys, Config: cfg, Resolver: r, Walker: walker.New(fsys, r)}, nil
}

func Open(dir, tsconfigPath string) (*Project, error) {
	return Load(os.DirFS(dir), tsconfigPath)
}

func rebasePaths(cfgPath string, cfg *tsconfig.Config) map[string][]string {
	if len(cfg.Paths) == 0 && cfg.BaseURL == "" {
		return nil
	}
	base := path.Join(path.Dir(cfgPath), cfg.BaseURL)
	out := make(map[string][]string, len(cfg.Paths)+1)
	for pattern, subs := range cfg.Paths {
		rebased := make([]string, len(subs))
		for i, sub := range subs {
			rebased[i] = path.Join(base, sub)
		}
		out[pattern] = rebased
	}
	if cfg.BaseURL != "" {
		if _, ok := out["*"]; !ok {
			out["*"] = []string{path.Join(base, "*")}
		}
	}
	return out
}
