package tsconfig

import "io/fs"

type Config struct {
	BaseURL                    string
	Paths                      map[string][]string
	ModuleResolution           string
	AllowImportingTsExtensions bool
}

func Load(fsys fs.FS, path string) (*Config, error)
