package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestDependencyDirection(t *testing.T) {
	const module = "github.com/lobov/familyquest/backend/internal/"
	layers := map[string][]string{
		"domain": {"domain"}, "application": {"application", "domain"},
		"httpapi": {"httpapi", "application", "domain"}, "store": {"store", "application", "domain"},
		"auth": {"auth", "domain"}, "config": {"config"},
	}
	for layer, allowed := range layers {
		err := filepath.WalkDir(filepath.Join("..", layer), func(file string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range parsed.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if strings.HasPrefix(path, module) {
					destination := strings.Split(strings.TrimPrefix(path, module), "/")[0]
					if !slices.Contains(allowed, destination) {
						t.Errorf("%s imports disallowed layer %s", file, path)
					}
				} else if layer == "domain" || layer == "application" {
					if strings.Contains(path, ".") || path == "net" || path == "database/sql" || path == "os" || strings.HasPrefix(path, "os/") || strings.HasPrefix(path, "net/http") || path == "net/rpc" {
						t.Errorf("%s imports outer dependency %s", file, path)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
