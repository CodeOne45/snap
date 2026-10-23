package capture

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/CodeOne45/snap/internal/ports"
)

// SeedDetector proposes one raw candidate string from a potential source of
// a project's identifying name. It returns ok=false, not an error, when its
// source simply isn't present — a missing package.json isn't a scan
// failure.
type SeedDetector struct {
	Name   string
	Detect func(fsys ports.FS, root string) (value string, ok bool, err error)
}

// SeedDetectors are run, in order, by BuildCandidates. Matches PLAN.md §5's
// seed list: project folder name, name in package.json / go.mod module /
// Cargo.toml / pyproject.toml, git remote repo name, README H1.
var SeedDetectors = []SeedDetector{
	{Name: "folder-name", Detect: detectFolderName},
	{Name: "package.json", Detect: detectPackageJSON},
	{Name: "go.mod", Detect: detectGoMod},
	{Name: "Cargo.toml", Detect: detectCargoToml},
	{Name: "pyproject.toml", Detect: detectPyProject},
	{Name: "git-remote", Detect: detectGitRemote},
	{Name: "readme-h1", Detect: detectReadmeH1},
}

func detectFolderName(_ ports.FS, root string) (string, bool, error) {
	name := path.Base(root)
	if name == "" || name == "." || name == "/" {
		return "", false, nil
	}
	return name, true, nil
}

func detectPackageJSON(fsys ports.FS, root string) (string, bool, error) {
	data, err := fsys.ReadFile(path.Join(root, "package.json"))
	if err != nil {
		return "", false, nil
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false, err
	}
	return pkg.Name, pkg.Name != "", nil
}

func detectGoMod(fsys ports.FS, root string) (string, bool, error) {
	data, err := fsys.ReadFile(path.Join(root, "go.mod"))
	if err != nil {
		return "", false, nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if mod, ok := strings.CutPrefix(line, "module "); ok {
			mod = strings.TrimSpace(mod)
			if mod == "" {
				return "", false, nil
			}
			parts := strings.Split(mod, "/")
			return parts[len(parts)-1], true, nil
		}
	}
	return "", false, nil
}

func detectCargoToml(fsys ports.FS, root string) (string, bool, error) {
	data, err := fsys.ReadFile(path.Join(root, "Cargo.toml"))
	if err != nil {
		return "", false, nil
	}
	var cargo struct {
		Package struct {
			Name string `toml:"name"`
		} `toml:"package"`
	}
	if err := toml.Unmarshal(data, &cargo); err != nil {
		return "", false, err
	}
	return cargo.Package.Name, cargo.Package.Name != "", nil
}

func detectPyProject(fsys ports.FS, root string) (string, bool, error) {
	data, err := fsys.ReadFile(path.Join(root, "pyproject.toml"))
	if err != nil {
		return "", false, nil
	}
	var py struct {
		Project struct {
			Name string `toml:"name"`
		} `toml:"project"`
		Tool struct {
			Poetry struct {
				Name string `toml:"name"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}
	if err := toml.Unmarshal(data, &py); err != nil {
		return "", false, err
	}
	if py.Project.Name != "" {
		return py.Project.Name, true, nil
	}
	return py.Tool.Poetry.Name, py.Tool.Poetry.Name != "", nil
}

// gitRemoteURL matches "url = ..." inside a [remote "origin"] section of a
// plain-text git config file.
var gitRemoteSection = regexp.MustCompile(`(?m)^\[remote\s+"origin"\]\s*$`)
var gitRemoteURL = regexp.MustCompile(`(?m)^\s*url\s*=\s*(\S+)\s*$`)

func detectGitRemote(fsys ports.FS, root string) (string, bool, error) {
	data, err := fsys.ReadFile(path.Join(root, ".git", "config"))
	if err != nil {
		return "", false, nil
	}
	content := string(data)
	loc := gitRemoteSection.FindStringIndex(content)
	if loc == nil {
		return "", false, nil
	}
	rest := content[loc[1]:]
	if next := gitRemoteURL.FindStringSubmatchIndex(rest); next != nil {
		url := rest[next[2]:next[3]]
		return repoNameFromURL(url), true, nil
	}
	return "", false, nil
}

func repoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")
	if i := strings.LastIndexAny(url, "/:"); i != -1 {
		return url[i+1:]
	}
	return url
}

var readmeH1 = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

func detectReadmeH1(fsys ports.FS, root string) (string, bool, error) {
	data, err := fsys.ReadFile(path.Join(root, "README.md"))
	if err != nil {
		return "", false, nil
	}
	m := readmeH1.FindStringSubmatch(string(data))
	if m == nil {
		return "", false, nil
	}
	return m[1], true, nil
}
