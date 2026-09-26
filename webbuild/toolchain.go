package webbuild

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// sdkVersion is the Go 1.21 distribution the GopherJS compiler requires
// as its GOROOT. GopherJS 1.21 compiles only a Go 1.21 GOROOT, so the
// bootstrap downloads this distribution when the user's Go is not 1.21.
const sdkVersion = "go1.21.13"

// sdkURL is the base URL of the Go distribution archives.
const sdkURL = "https://dl.google.com/go/"

// sdkHashes are the sha256 checksums of the sdkVersion archives, keyed
// by "<os>-<arch>".
var sdkHashes = map[string]string{
	"linux-amd64":   "502fc16d5910562461e6a6631fb6377de2322aad7304bf2bcd23500ba9dab4a7",
	"linux-arm64":   "2ca2d70dc9c84feef959eb31f2a5aac33eefd8c97fe48f1548886d737bffabd4",
	"darwin-amd64":  "796fd05e8741f6776c505eb201922864f2e32991679b639d9fcb524dbe300c0d",
	"darwin-arm64":  "c04ee7bdc0e65cf17133994c40ee9bdfa1b1dc9587b3baedaea39affdb8e5b49",
	"windows-amd64": "924655193634bfcdf7ec7a34589e0d73458741998a59e4155a929ce85f81af2d",
	"windows-arm64": "74fb3a74cdf0cf6cfea664d3746aea423a3e4a8952b749920f8013d735a59589",
}

// goInfo describes the Go toolchain on the user's PATH.
type goInfo struct {
	version string
	goroot  string
	gobin   string
	gopath  string
	binary  string
}

var (
	userGoOnce sync.Once
	userGoInfo *goInfo
	userGoErr  error
)

// userGo returns the Go toolchain on the user's PATH, resolving it once.
func userGo() (*goInfo, error) {
	userGoOnce.Do(func() {
		userGoInfo, userGoErr = detectUserGo()
	})
	return userGoInfo, userGoErr
}

// detectUserGo finds the go on PATH and reads its version and env.
func detectUserGo() (*goInfo, error) {
	binary, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("find go: %w", err)
	}
	vout, err := runCmd("", binary, "version")
	if err != nil {
		return nil, fmt.Errorf("go version: %w", err)
	}
	fields := strings.Fields(string(vout))
	if len(fields) < 3 {
		return nil, fmt.Errorf("unexpected go version output %q", vout)
	}
	gout, err := runCmd("", binary, "env", "-json")
	if err != nil {
		return nil, fmt.Errorf("go env: %w", err)
	}
	var env struct {
		GOROOT string
		GOBIN  string
		GOPATH string
	}
	if err := json.Unmarshal(gout, &env); err != nil {
		return nil, fmt.Errorf("parse go env: %w", err)
	}
	return &goInfo{
		version: fields[2],
		goroot:  env.GOROOT,
		gobin:   env.GOBIN,
		gopath:  env.GOPATH,
		binary:  binary,
	}, nil
}

// goMajorMinor parses the major and minor version from a Go version
// string like "go1.27.1".
func goMajorMinor(version string) (int, int, error) {
	v := strings.TrimPrefix(version, "go")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("bad Go version %q", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

// toolchain holds a ready GopherJS toolchain: the gopherjs CLI path and
// the GOROOT it compiles against.
type toolchain struct {
	cli    string
	goroot string
}

// ensureToolchain provisions a GopherJS toolchain for the client module
// in dir, returning the gopherjs CLI path and the GOROOT to compile
// against.
//
// The GOROOT is a Go 1.21 distribution: the user's GOROOT when their Go
// is 1.21, otherwise a downloaded SDK. The gopherjs CLI is built with a
// Go 1.21-1.26 toolchain: the user's Go when it is in that range,
// otherwise the SDK's Go.
func ensureToolchain(dir string) (*toolchain, error) {
	g, err := userGo()
	if err != nil {
		return nil, err
	}
	major, minor, err := goMajorMinor(g.version)
	if err != nil {
		return nil, err
	}
	version, err := gopherjsVersion(dir)
	if err != nil {
		return nil, err
	}

	needSDK := !(major == 1 && minor == 21)
	var sdk *sdkInfo
	if needSDK {
		sdk, err = ensureSDK()
		if err != nil {
			return nil, err
		}
	}

	var goroot string
	if needSDK {
		goroot = sdk.goroot
	} else {
		goroot = g.goroot
	}

	// The gopherjs CLI must be built with a Go 1.21-1.26 toolchain. Use
	// the user's Go when it is in that range, otherwise the SDK's Go.
	// The GOROOT is set to the chosen toolchain's GOROOT, because an
	// inherited GOROOT env var would otherwise point the toolchain at a
	// mismatched compiler.
	var cliGo, cliGoroot string
	if major == 1 && minor >= 21 && minor <= 26 {
		cliGo = g.binary
		cliGoroot = g.goroot
	} else {
		cliGo = sdk.goBinary
		cliGoroot = sdk.goroot
	}

	cli, err := ensureCLI(cliGo, cliGoroot, version)
	if err != nil {
		return nil, err
	}
	return &toolchain{cli: cli, goroot: goroot}, nil
}

// sdkInfo describes a downloaded Go SDK distribution.
type sdkInfo struct {
	goroot   string
	goBinary string
}

// ensureSDK returns the Go 1.21 SDK in the user cache directory,
// downloading and extracting it on first use.
func ensureSDK() (*sdkInfo, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	sdkDir := filepath.Join(cacheDir, "sheidan", sdkVersion)
	goroot := filepath.Join(sdkDir, "go")
	if _, err := os.Stat(filepath.Join(goroot, "VERSION")); err == nil {
		return &sdkInfo{goroot: goroot, goBinary: filepath.Join(goroot, "bin", goExe())}, nil
	}
	if err := downloadSDK(sdkDir); err != nil {
		return nil, err
	}
	return &sdkInfo{goroot: goroot, goBinary: filepath.Join(goroot, "bin", goExe())}, nil
}

// downloadSDK downloads the sdkVersion distribution, verifies its
// checksum, and extracts it to sdkDir.
func downloadSDK(sdkDir string) error {
	key := runtime.GOOS + "-" + runtime.GOARCH
	hash, ok := sdkHashes[key]
	if !ok {
		return fmt.Errorf("no Go %s distribution for %s", sdkVersion, key)
	}
	isZip := runtime.GOOS == "windows"
	ext := ".tar.gz"
	if isZip {
		ext = ".zip"
	}
	filename := sdkVersion + "." + key + ext
	url := sdkURL + filename

	tmp, err := os.CreateTemp("", "sheidan-go-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := downloadFile(url, tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := verifySha256(tmpName, hash); err != nil {
		return err
	}
	if err := os.RemoveAll(sdkDir); err != nil {
		return err
	}
	if err := os.MkdirAll(sdkDir, 0o755); err != nil {
		return err
	}
	f, err := os.Open(tmpName)
	if err != nil {
		return err
	}
	defer f.Close()
	if isZip {
		info, err := f.Stat()
		if err != nil {
			return err
		}
		return extractZip(f, info.Size(), sdkDir)
	}
	return extractTarGz(f, sdkDir)
}

// downloadFile writes the content of url to w.
func downloadFile(url string, w io.Writer) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// verifySha256 checks that the file at path has the want sha256.
func verifySha256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", path, got, want)
	}
	return nil
}

// extractTarGz extracts a .tar.gz archive to dest.
func extractTarGz(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractZip extracts a .zip archive to dest.
func extractZip(r *os.File, size int64, dest string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	for _, zf := range zr.File {
		target, err := safeJoin(dest, zf.Name)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, zf.FileInfo().Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

// safeJoin joins dest and name, rejecting paths that escape dest.
func safeJoin(dest, name string) (string, error) {
	dest = filepath.Clean(dest)
	target := filepath.Join(dest, name)
	if target != dest && !strings.HasPrefix(target, dest+string(filepath.Separator)) {
		return "", fmt.Errorf("illegal file path %q", name)
	}
	return target, nil
}

// goExe returns the name of the go executable for the current OS.
func goExe() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

// defaultGopherJSVersion is the GopherJS version the bootstrap installs
// when it cannot determine the client module's version.
const defaultGopherJSVersion = "v1.21.0"

// gopherjsVersion returns the GopherJS version in the client module's
// build list, or defaultGopherJSVersion when it cannot be determined.
func gopherjsVersion(dir string) (string, error) {
	out, err := runCmd(dir, "go", "list", "-m", "-f", "{{.Version}}", "github.com/gopherjs/gopherjs")
	if err != nil {
		return defaultGopherJSVersion, nil
	}
	if v := strings.TrimSpace(string(out)); v != "" {
		return v, nil
	}
	return defaultGopherJSVersion, nil
}

// cliDir returns the directory the gopherjs CLI is installed to: GOBIN
// when set, otherwise GOPATH/bin.
func cliDir() (string, error) {
	g, err := userGo()
	if err != nil {
		return "", err
	}
	if g.gobin != "" {
		return g.gobin, nil
	}
	return filepath.Join(g.gopath, "bin"), nil
}

// markerPath returns the path of the file recording the GopherJS version
// the installed CLI was built with.
func markerPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "sheidan", "gopherjs-version"), nil
}

// ensureCLI returns the path to the gopherjs CLI, building it with
// goBinary when it is missing or was built with a different GopherJS
// version. goroot is the GOROOT of goBinary, set explicitly so an
// inherited GOROOT env var cannot point the toolchain at a mismatched
// compiler.
func ensureCLI(goBinary, goroot, version string) (string, error) {
	dir, err := cliDir()
	if err != nil {
		return "", err
	}
	cli := filepath.Join(dir, cliName())
	marker, err := markerPath()
	if err != nil {
		return "", err
	}
	if content, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(content)) == version {
		if _, err := os.Stat(cli); err == nil {
			return cli, nil
		}
	}
	env := append(os.Environ(), "GOTOOLCHAIN=local", "GOBIN="+dir, "GOROOT="+goroot)
	cmd := exec.Command(goBinary, "install", "github.com/gopherjs/gopherjs@"+version)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build gopherjs CLI with %s: %w", goBinary, err)
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(marker, []byte(version), 0o644); err != nil {
		return "", err
	}
	return cli, nil
}

// cliName returns the name of the gopherjs executable for the current OS.
func cliName() string {
	if runtime.GOOS == "windows" {
		return "gopherjs.exe"
	}
	return "gopherjs"
}

// runCmd runs name with args in dir, returning stdout. Stderr goes to
// the terminal, so the user sees toolchain output.
func runCmd(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return stdout.Bytes(), err
}
