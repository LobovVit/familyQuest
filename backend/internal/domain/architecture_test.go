package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDependencyDirection(t *testing.T) {
	for _, layer := range []string{"domain", "application"} {
		files, err := os.ReadDir(filepath.Join("..", layer))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", layer, file.Name()), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if path == "net/http" || path == "database/sql" || path == "os" || strings.Contains(path, ".") && !(layer == "application" && path == "github.com/lobov/familyquest/backend/internal/domain") {
					t.Errorf("%s/%s imports outer dependency %s", layer, file.Name(), path)
				}
			}
		}
	}
}
