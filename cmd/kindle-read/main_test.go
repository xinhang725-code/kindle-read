package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSingleInstanceLockIsReleasedWithoutKillingOtherProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	a, e := lock(path)
	if e != nil {
		t.Fatal(e)
	}
	if b, e := lock(path); e == nil {
		b.Close()
		t.Fatal("duplicate instance admitted")
	}
	a.Close()
	b, e := lock(path)
	if e != nil {
		t.Fatal(e)
	}
	b.Close()
}
func TestSessionDiscoveryRejectsExternalURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.json")
	for _, raw := range []string{`{"source":"qidian","url":"https://example.com/s/token/"}`, `{"source":"qidian","url":"http://u:p@127.0.0.1:1/s/token/"}`, `{"source":"miuread","url":"http://127.0.0.1:1/s/token/"}`} {
		os.WriteFile(path, []byte(raw), 0600)
		if healthy(path, "qidian") {
			t.Fatal("untrusted discovery accepted")
		}
	}
}

func TestInstallLocksRefuseRunningAppAndReleaseEarlierLocks(t *testing.T) {
	data := t.TempDir()
	active, e := lock(filepath.Join(data, "run", "changpei.lock"))
	if e != nil {
		t.Fatal(e)
	}
	if release, e := lockApps(data); e == nil {
		release()
		t.Fatal("installation allowed during reading")
	}
	probe, e := lock(filepath.Join(data, "run", "qidian.lock"))
	if e != nil {
		t.Fatal("failed lock acquisition leaked earlier lock")
	}
	probe.Close()
	active.Close()
	release, e := lockApps(data)
	if e != nil {
		t.Fatal(e)
	}
	release()
}
