package device

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Real SQLite transaction behavior; Python is only a host test harness, never a device dependency.
func TestSQLiteRegistrationTransactionAndUninstall(t *testing.T) {
	if _, e := exec.LookPath("python3"); e != nil {
		t.Skip("Python3 SQLite harness unavailable")
	}
	i := prepare(t)
	db := i.path("/var/local/appreg.db")
	os.Remove(db)
	i.SQL = func(query string) (string, error) {
		cmd := exec.Command("python3", "-c", `import sqlite3,sys
c=sqlite3.connect(sys.argv[1])
q=sys.stdin.read()
try:
 if q.startswith('SELECT') and q.count(';') == 1:
  for row in c.execute(q): print(row[0])
 else: c.executescript(q)
finally: c.close()
`, db)
		cmd.Stdin = strings.NewReader(query)
		out, e := cmd.Output()
		return strings.TrimSpace(string(out)), e
	}
	_, e := i.SQL("CREATE TABLE interfaces(interface TEXT PRIMARY KEY); CREATE TABLE handlerIds(handlerId TEXT PRIMARY KEY); CREATE TABLE properties(handlerId TEXT,name TEXT,value TEXT,PRIMARY KEY(handlerId,name)); INSERT INTO handlerIds VALUES('unrelated');")
	if e != nil {
		t.Fatal(e)
	}
	if e = i.Install(); e != nil {
		t.Fatal(e)
	}
	count, _ := i.SQL("SELECT count(*) FROM handlerIds;")
	if count != "4" {
		t.Fatalf("registration count %s", count)
	}
	if e = i.Install(); e != nil {
		t.Fatal(e)
	}
	if e = i.Uninstall(); e != nil {
		t.Fatal(e)
	}
	count, _ = i.SQL("SELECT count(*) FROM handlerIds;")
	if count != "1" {
		t.Fatal("unrelated registration deleted")
	}
	_, e = i.SQL("CREATE TRIGGER fail_install BEFORE INSERT ON properties WHEN NEW.handlerId='" + AppID("changpei") + "' BEGIN SELECT RAISE(ABORT,'test full disk'); END;")
	if e != nil {
		t.Fatal(e)
	}
	if e = i.Install(); e == nil {
		t.Fatal("trigger failure ignored")
	}
	count, _ = i.SQL("SELECT count(*) FROM handlerIds;")
	if count != "1" {
		t.Fatal("failed transaction partly committed")
	}
}
