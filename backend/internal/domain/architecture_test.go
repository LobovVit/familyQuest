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

func allowedDependency(layer, path string) bool {
	const module = "github.com/lobov/familyquest/backend/internal/"
	layers := map[string][]string{
		"domain": {"domain"}, "application": {"application", "domain"},
		"httpapi": {"httpapi", "application", "domain"}, "store": {"store", "application", "domain"},
		"auth": {"auth", "domain"}, "config": {"config"}, "buildinfo": {"buildinfo"},
		"identity": {"identity"}, "testoidc": {"testoidc"},
		// Access shares HTTP wire types and CSRF policy with httpapi. This is an
		// explicit adapter exception, not permission to import persistence/auth.
		"access": {"access", "domain", "httpapi", "identity"},
	}
	allowed, ok := layers[layer]
	if !ok {
		return false
	}
	if strings.HasPrefix(path, module) {
		destination := strings.Split(strings.TrimPrefix(path, module), "/")[0]
		return slices.Contains(allowed, destination)
	}
	if layer == "domain" || layer == "application" {
		if strings.Contains(path, ".") {
			return false
		}
		for _, forbidden := range []string{"database", "net", "os", "syscall", "plugin", "unsafe"} {
			if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
				return false
			}
		}
	}
	return true
}

func TestDependencyDirection(t *testing.T) {
	// Walk the whole internal tree so new packages cannot silently escape review.
	err := filepath.WalkDir("..", func(file string, entry os.DirEntry, walkErr error) error {
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
		rel, err := filepath.Rel("..", file)
		if err != nil {
			return err
		}
		layer := strings.Split(filepath.ToSlash(rel), "/")[0]
		if !allowedDependency(layer, "context") {
			t.Errorf("%s belongs to an unregistered layer %s", file, layer)
		}
		for _, imp := range parsed.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if !allowedDependency(layer, path) {
				t.Errorf("%s imports disallowed dependency %s", file, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDependencyPolicyCoversAdaptersAndIO(t *testing.T) {
	for _, tc := range []struct{ layer, path string }{
		{"domain", "net/smtp"}, {"application", "database/sql/driver"},
		{"application", "os/exec"}, {"domain", "unsafe"},
		{"access", "github.com/lobov/familyquest/backend/internal/store"},
		{"identity", "github.com/lobov/familyquest/backend/internal/application"},
		{"new-unreviewed-layer", "context"},
	} {
		if allowedDependency(tc.layer, tc.path) {
			t.Errorf("accepted %s -> %s", tc.layer, tc.path)
		}
	}
}
