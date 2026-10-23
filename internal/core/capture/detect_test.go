package capture

import (
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
)

func TestDetectFolderName(t *testing.T) {
	val, ok, err := detectFolderName(nil, "some/path/acme-site")
	if err != nil || !ok || val != "acme-site" {
		t.Errorf("detectFolderName = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
	}
}

func TestDetectPackageJSON(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/package.json", `{"name": "acme-site", "version": "1.0.0"}`)

	val, ok, err := detectPackageJSON(fsys, "proj")
	if err != nil || !ok || val != "acme-site" {
		t.Errorf("detectPackageJSON = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
	}

	if _, ok, err := detectPackageJSON(fsys, "nowhere"); ok || err != nil {
		t.Errorf("detectPackageJSON(missing) = (_, %v, %v), want (false, nil)", ok, err)
	}
}

func TestDetectPackageJSON_Malformed(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/package.json", `not json`)
	if _, _, err := detectPackageJSON(fsys, "proj"); err == nil {
		t.Error("detectPackageJSON(malformed): want error, got nil")
	}
}

func TestDetectGoMod(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/go.mod", "module github.com/CodeOne45/acme-site\n\ngo 1.22\n")

	val, ok, err := detectGoMod(fsys, "proj")
	if err != nil || !ok || val != "acme-site" {
		t.Errorf("detectGoMod = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
	}
}

func TestDetectCargoToml(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/Cargo.toml", "[package]\nname = \"acme-site\"\nversion = \"0.1.0\"\n")

	val, ok, err := detectCargoToml(fsys, "proj")
	if err != nil || !ok || val != "acme-site" {
		t.Errorf("detectCargoToml = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
	}
}

func TestDetectPyProject(t *testing.T) {
	t.Run("PEP 621", func(t *testing.T) {
		fsys := memfs.New()
		write(t, fsys, "proj/pyproject.toml", "[project]\nname = \"acme-site\"\n")
		val, ok, err := detectPyProject(fsys, "proj")
		if err != nil || !ok || val != "acme-site" {
			t.Errorf("detectPyProject = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
		}
	})
	t.Run("poetry fallback", func(t *testing.T) {
		fsys := memfs.New()
		write(t, fsys, "proj/pyproject.toml", "[tool.poetry]\nname = \"acme-site\"\n")
		val, ok, err := detectPyProject(fsys, "proj")
		if err != nil || !ok || val != "acme-site" {
			t.Errorf("detectPyProject = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
		}
	})
}

func TestDetectGitRemote(t *testing.T) {
	fsys := memfs.New()
	config := `[core]
	bare = false
[remote "origin"]
	url = git@github.com:someuser/acme-site.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`
	write(t, fsys, "proj/.git/config", config)

	val, ok, err := detectGitRemote(fsys, "proj")
	if err != nil || !ok || val != "acme-site" {
		t.Errorf("detectGitRemote = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
	}
}

func TestDetectGitRemote_HTTPS(t *testing.T) {
	fsys := memfs.New()
	config := `[remote "origin"]
	url = https://github.com/someuser/acme-site.git
`
	write(t, fsys, "proj/.git/config", config)

	val, ok, err := detectGitRemote(fsys, "proj")
	if err != nil || !ok || val != "acme-site" {
		t.Errorf("detectGitRemote = (%q, %v, %v), want (acme-site, true, nil)", val, ok, err)
	}
}

func TestDetectGitRemote_NoRemote(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/.git/config", "[core]\n\tbare = false\n")
	if _, ok, err := detectGitRemote(fsys, "proj"); ok || err != nil {
		t.Errorf("detectGitRemote(no origin) = (_, %v, %v), want (false, nil)", ok, err)
	}
}

func TestDetectReadmeH1(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/README.md", "\n# Acme Site\n\nSome description.\n## Subheading\n")

	val, ok, err := detectReadmeH1(fsys, "proj")
	if err != nil || !ok || val != "Acme Site" {
		t.Errorf("detectReadmeH1 = (%q, %v, %v), want (Acme Site, true, nil)", val, ok, err)
	}
}

func TestDetectReadmeH1_Missing(t *testing.T) {
	fsys := memfs.New()
	if _, ok, err := detectReadmeH1(fsys, "proj"); ok || err != nil {
		t.Errorf("detectReadmeH1(missing) = (_, %v, %v), want (false, nil)", ok, err)
	}
}
