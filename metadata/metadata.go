package metadata

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/go-errors/errors"

	opt "github.com/moznion/go-optional"
	"gopkg.in/yaml.v3"
)

type Role string

const (
	Import Role = "import"
	Export Role = "export"
)

type MetadataFile struct {
	ProtoMap *[]*struct {
		ProtoSpec `yaml:",inline"`
		Role Role
	} `yaml:"proto-map"`
}

type ProtoSpec struct {
	// The name of the protocol
	Name   string
	// The path of the protocol file
	Path   string
	// The go import path of the generated proto code
	Import string
}

type Metadata struct {
	ExportProto *ProtoSpec
	ImportProto opt.Option[*ProtoSpec]
}

func Parse(file string, absolutify bool) (*Metadata, error) {
	m := &MetadataFile{}

	f, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(f, m); err != nil {
		return nil, err
	}

	if m.ProtoMap == nil {
		return nil, errors.Errorf("no protocol maps defined when parsing %s", file)
	}

	if absolutify {
		for _, e := range *m.ProtoMap {
			fmt.Println(e.Path)
			if !path.IsAbs(e.Path) {
				fmt.Println(file)
				abs, err := filepath.Abs(file)
				if err != nil {
					return nil, err
				}
				fmt.Println(abs)
				abspath, err := filepath.Abs(filepath.Join(filepath.Dir(abs), e.Path))
				if err != nil {
					return nil, err
				}
				fmt.Println(abspath)
				e.Path = abspath
			}

			if _, err := os.Stat(e.Path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, errors.Errorf("protocol file %s does not exists", e.Path)
				} else {
					return nil, err
				}
			}
		}

	}

	protos := make(map[string]ProtoSpec)
	for _, e := range *m.ProtoMap {
		protos[string(e.Role)] = ProtoSpec{
			Name:   e.Name,
			Path:   e.Path,
			Import: e.Import,
		}
	}

	var importMap opt.Option[*ProtoSpec] = nil
	var exportMap *ProtoSpec
	if val, ok := protos["export"]; ok {
		exportMap = &val
	} else {
		return nil, errors.Errorf("protocol metadata does not define an export protocol")
	}

	if val, ok := protos["import"]; ok {
		importMap = opt.Some(&val)
	}

	return &Metadata{
		ImportProto: importMap,
		ExportProto: exportMap,
	}, nil
}
