package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyIPK(t *testing.T) {
	distDir := filepath.Join("..", "dist")
	ipkPath := filepath.Join(distDir, "luci-app-sirin_1.4.0-1_all.ipk")

	f, err := os.Open(ipkPath)
	if err != nil {
		t.Fatalf("Open ipk: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	foundFiles := make(map[string]bool)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Tar next: %v", err)
		}
		foundFiles[hdr.Name] = true

		if hdr.Name == "control.tar.gz" {
			// Read and verify control
			subGz, err := gzip.NewReader(tr)
			if err != nil {
				t.Fatalf("Sub gz control: %v", err)
			}
			subTr := tar.NewReader(subGz)
			hasControl := false
			for {
				shdr, serr := subTr.Next()
				if serr == io.EOF {
					break
				}
				if serr != nil {
					t.Fatalf("Sub tar control: %v", serr)
				}
				if shdr.Name == "control" {
					hasControl = true
					data, _ := io.ReadAll(subTr)
					if !strings.Contains(string(data), "Package: luci-app-sirin") {
						t.Errorf("control does not contain Package: luci-app-sirin")
					}
				}
			}
			if !hasControl {
				t.Errorf("control file not found inside control.tar.gz")
			}
		}

		if hdr.Name == "data.tar.gz" {
			// Read and verify data files
			subGz, err := gzip.NewReader(tr)
			if err != nil {
				t.Fatalf("Sub gz data: %v", err)
			}
			subTr := tar.NewReader(subGz)
			hasMenu := false
			hasRPCD := false
			for {
				shdr, serr := subTr.Next()
				if serr == io.EOF {
					break
				}
				if serr != nil {
					t.Fatalf("Sub tar data: %v", serr)
				}
				if strings.Contains(shdr.Name, "menu.d/luci-app-sirin.json") {
					hasMenu = true
				}
				if strings.Contains(shdr.Name, "rpcd/sirin") {
					hasRPCD = true
					if shdr.Mode != 0755 {
						t.Errorf("Expected mode 0755 for rpcd/sirin, got %o", shdr.Mode)
					}
				}
			}
			if !hasMenu || !hasRPCD {
				t.Errorf("data.tar.gz missing expected files (hasMenu=%v, hasRPCD=%v)", hasMenu, hasRPCD)
			}
		}
	}

	if !foundFiles["debian-binary"] || !foundFiles["control.tar.gz"] || !foundFiles["data.tar.gz"] {
		t.Errorf("IPK missing standard sections: %v", foundFiles)
	}
}

func TestVerifyAPK(t *testing.T) {
	distDir := filepath.Join("..", "dist")
	apkPath := filepath.Join(distDir, "luci-app-sirin-1.4.0-r1.apk")

	f, err := os.Open(apkPath)
	if err != nil {
		t.Fatalf("Open apk: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	foundFiles := make(map[string]bool)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Tar next: %v", err)
		}
		foundFiles[hdr.Name] = true

		if hdr.Name == ".PKGINFO" {
			data, _ := io.ReadAll(tr)
			if !strings.Contains(string(data), "pkgname = luci-app-sirin") {
				t.Errorf(".PKGINFO missing pkgname = luci-app-sirin")
			}
		}
	}

	if !foundFiles[".PKGINFO"] || !foundFiles[".post-install"] {
		t.Errorf("APK missing metadata files: %v", foundFiles)
	}
}
