package device

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func prepare(t *testing.T) *Installer {
	t.Helper()
	root := t.TempDir()
	for _, p := range []string{"usr/bin/mesquite", "usr/bin/sqlite3", "usr/bin/lipc-set-prop", "var/local/appreg.db", "mnt/us/extensions/kindle-read/bin/kindle-read"} {
		path := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte("fixture"), 0755)
	}
	return &Installer{Root: root, SQL: func(query string) (string, error) {
		if strings.HasPrefix(query, "SELECT") {
			return "", nil
		}
		return "", nil
	}}
}
func TestInstallThreeIndependentEntriesAndPreserveUnknownFiles(t *testing.T) {
	i := prepare(t)
	if e := i.Install(); e != nil {
		t.Fatal(e)
	}
	for _, s := range Sources {
		b, e := os.ReadFile(i.path("/mnt/us/documents/KindleRead-" + s + ".sh"))
		if e != nil || !strings.Contains(string(b), "launch "+s) {
			t.Fatalf("%s %v", b, e)
		}
		if _, e = os.Stat(i.path("/var/local/mesquite/" + AppID(s) + "/config.xml")); e != nil {
			t.Fatal(e)
		}
	}
	if e := i.Install(); e != nil {
		t.Fatalf("upgrade %v", e)
	}
	file := i.path("/mnt/us/documents/KindleRead-qidian.sh")
	os.WriteFile(file, []byte("user file"), 0600)
	if e := i.Install(); e == nil {
		t.Fatal("overwrote user file")
	}
	b, _ := os.ReadFile(file)
	if string(b) != "user file" {
		t.Fatal("file changed")
	}
}
func TestSelectedInstallAndUninstallNeverTouchOtherApps(t *testing.T) {
	i := prepare(t)
	i.Source = "changpei"
	other := i.path("/mnt/us/documents/KindleRead-qidian.sh")
	os.MkdirAll(filepath.Dir(other), 0755)
	os.WriteFile(other, []byte("user-owned"), 0600)
	var queries []string
	i.SQL = func(q string) (string, error) { queries = append(queries, q); return "", nil }
	if e := i.Install(); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(i.path("/mnt/us/documents/KindleRead-changpei.sh")); e != nil || !strings.Contains(string(b), "launch changpei") {
		t.Fatal("missing Changpei", e)
	}
	if _, e := os.Stat(i.path("/mnt/us/documents/KindleRead-miuread.sh")); !os.IsNotExist(e) {
		t.Fatal("installed another app")
	}
	if e := i.Uninstall(); e != nil {
		t.Fatal(e)
	}
	if b, _ := os.ReadFile(other); string(b) != "user-owned" {
		t.Fatal("changed unrelated entry")
	}
	for _, q := range queries {
		if strings.Contains(q, AppID("qidian")) || strings.Contains(q, AppID("miuread")) {
			t.Fatal("changed unrelated registration")
		}
	}
	i.Source = "unknown"
	queries = nil
	if e := i.Install(); e == nil || len(queries) > 0 {
		t.Fatal("invalid source must fail before SQL")
	}
}
func TestFailedRegistrationRollsBackFiles(t *testing.T) {
	i := prepare(t)
	i.SQL = func(query string) (string, error) {
		if strings.HasPrefix(query, "SELECT") {
			return "", nil
		}
		return "", errors.New("disk full")
	}
	if e := i.Install(); e == nil {
		t.Fatal("registration failure ignored")
	}
	if _, e := os.Stat(i.path("/mnt/us/documents/KindleRead-qidian.sh")); !os.IsNotExist(e) {
		t.Fatal("partial scriptlet left")
	}
}
func TestMissingPrerequisitesDoNotMutateSystem(t *testing.T) {
	i := prepare(t)
	os.Remove(i.path("/usr/bin/mesquite"))
	if e := i.Install(); e == nil {
		t.Fatal("missing Mesquite accepted")
	}
	if _, e := os.Stat(i.path("/mnt/us/documents/KindleRead-qidian.sh")); !os.IsNotExist(e) {
		t.Fatal("partial install")
	}
}
func TestUninstallPreservesDataAndEditedEntry(t *testing.T) {
	i := prepare(t)
	if e := i.Install(); e != nil {
		t.Fatal(e)
	}
	data := i.path("/mnt/us/extensions/kindle-read/data/private.txt")
	os.MkdirAll(filepath.Dir(data), 0700)
	os.WriteFile(data, []byte("my data"), 0600)
	entry := i.path("/mnt/us/documents/KindleRead-qidian.sh")
	os.WriteFile(entry, []byte("edited by user"), 0600)
	if e := i.Uninstall(); e == nil {
		t.Fatal("edited entry should require manual handling before uninstall")
	}
	os.WriteFile(entry, []byte(Scriptlet("qidian")), 0600)
	if e := i.Uninstall(); e != nil {
		t.Fatal(e)
	}
	if b, _ := os.ReadFile(data); string(b) != "my data" {
		t.Fatal("user data removed")
	}
}
