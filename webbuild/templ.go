package webbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// generateTempl regenerates the *_templ.go files for any .templ files in
// dir, so a transpile picks up the latest templ output. It is a no-op
// when dir has no .templ files.
func generateTempl(dir string) error {
	hasTempl, err := hasTemplFiles(dir)
	if err != nil {
		return err
	}
	if !hasTempl {
		return nil
	}
	version, err := templVersion(dir)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "github.com/a-h/templ/cmd/templ@"+version, "generate")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("templ generate: %w", err)
	}
	return nil
}

// hasTemplFiles reports whether dir contains any .templ files.
func hasTemplFiles(dir string) (bool, error) {
	found := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".templ") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err == filepath.SkipAll {
		err = nil
	}
	return found, err
}

// templVersion returns the templ version in the client module's build
// list. The CLI version must match the templ library version the client
// uses, so the version comes from the client's build list.
func templVersion(dir string) (string, error) {
	out, err := runCmd(dir, "go", "list", "-m", "-f", "{{.Version}}", "github.com/a-h/templ")
	if err != nil {
		return "", fmt.Errorf("find templ version: %w", err)
	}
	if v := strings.TrimSpace(string(out)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("no templ version in %s", dir)
}
