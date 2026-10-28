// SPDX-License-Identifier: AGPL-3.0-only
package device

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xinhang725-code/kindle-read/internal/library"
)

var Sources = []string{"qidian", "changpei", "miuread"}

const Bundle = "/mnt/us/extensions/kindle-read"
const owner = "kindle-read managed application v1\n"

func AppID(source string) string { return "io.github.xinhang725.kindleread." + source }
func Name(source string) string {
	switch source {
	case "qidian":
		return "起点阅读"
	case "changpei":
		return "长佩阅读"
	default:
		return "MiuRead 独立版"
	}
}
func Scriptlet(source string) string {
	return "#!/bin/sh\n# Name: " + Name(source) + "\n# Author: kindle-read\n# DontUseFBInk\n# kindle-read managed entry v1\nexec " + Bundle + "/bin/kindle-read launch " + source + "\n"
}
func command(source string) string {
	return "/usr/bin/mesquite -l " + AppID(source) + " -c file:///var/local/mesquite/" + AppID(source) + "/"
}
func config(source string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<widget id="` + AppID(source) + `" version="0.2.0" viewmodes="application" xmlns="http://www.w3.org/ns/widgets" xmlns:kindle="http://kindle.amazon.com/ns/widget-extensions">
 <name xml:lang="zh">` + Name(source) + `</name><content src="index.html"/>
 <kindle:permissions><kindle:permission name="local-port-access"/></kindle:permissions>
 <kindle:chrome><kindle:asset key="configureSearchBar" value="none"/></kindle:chrome>
 <kindle:cookiejar><kindle:asset key="persistent" value="false"/><kindle:asset key="usePrivateCookies" value="true"/><kindle:asset key="useDeviceCookies" value="false"/><kindle:asset key="useAccessToken" value="false"/></kindle:cookiejar>
 <kindle:gestures><kindle:param name="tap" value="yes" properties="fire_on_tap:1 max_updown_delta:0"/></kindle:gestures>
 <feature name="http://kindle.amazon.com/apis" required="true"><param name="appmgr" value="yes"/><param name="device" value="yes"/><param name="gestures" value="yes"/></feature>
 <kindle:settings><kindle:setting name="internetRequired" value="no"/><kindle:setting name="saveContext" value="no"/></kindle:settings>
</widget>
`
}

type Installer struct {
	Source string // empty selects all legacy entries
	Root   string
	SQL    func(string) (string, error)
}

func (i *Installer) path(path string) string {
	return filepath.Join(i.Root, strings.TrimPrefix(path, "/"))
}
func (i *Installer) sql(query string) (string, error) {
	if i.SQL != nil {
		return i.SQL(query)
	}
	out, e := exec.Command(i.path("/usr/bin/sqlite3"), "-batch", i.path("/var/local/appreg.db"), query).CombinedOutput()
	if e != nil {
		return "", errors.New("appreg 数据库操作失败，未输出数据库内容")
	}
	return strings.TrimSpace(string(out)), nil
}
func (i *Installer) sources() []string {
	if i.Source != "" {
		return []string{i.Source}
	}
	return Sources
}
func (i *Installer) Preflight() error {
	if i.Source != "" && !library.ValidSource(i.Source) {
		return errors.New("未知应用，未修改入口")
	}
	for _, p := range []string{"/usr/bin/mesquite", "/usr/bin/sqlite3", "/usr/bin/lipc-set-prop", "/var/local/appreg.db", Bundle + "/bin/kindle-read"} {
		info, e := os.Stat(i.path(p))
		if e != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("缺少必需组件：%s；未安装任何入口", p)
		}
	}
	if _, e := i.sql("SELECT interface FROM interfaces LIMIT 0; SELECT handlerId FROM handlerIds LIMIT 0; SELECT handlerId,name,value FROM properties LIMIT 0;"); e != nil {
		return errors.New("appreg 表结构不兼容；未修改系统")
	}
	return nil
}
func (i *Installer) checkOwned(source string) error {
	dir := i.path("/var/local/mesquite/" + AppID(source))
	if info, e := os.Lstat(dir); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("应用目录不是普通目录")
		}
		b, e := os.ReadFile(filepath.Join(dir, ".kindle-read"))
		if e != nil || string(b) != owner {
			return fmt.Errorf("拒绝覆盖未归属本项目的目录：%s", dir)
		}
		entries, e := os.ReadDir(dir)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			if entry.Name() != ".kindle-read" && entry.Name() != "config.xml" && entry.Name() != "index.html" {
				return fmt.Errorf("应用目录含未知文件：%s", entry.Name())
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return errors.New("拒绝覆盖符号链接")
			}
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	entry := i.path("/mnt/us/documents/KindleRead-" + source + ".sh")
	if info, e := os.Lstat(entry); e == nil {
		b, e := os.ReadFile(entry)
		if !info.Mode().IsRegular() || e != nil || string(b) != Scriptlet(source) {
			return fmt.Errorf("书架脚本已存在或被修改，请先保留并人工处理：%s", entry)
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	id, e := i.sql("SELECT handlerId FROM handlerIds WHERE handlerId='" + AppID(source) + "';")
	if e != nil {
		return e
	}
	if id != "" {
		b, e := os.ReadFile(filepath.Join(dir, ".kindle-read"))
		if e != nil || string(b) != owner {
			return errors.New("应用 ID 已被其他程序注册")
		}
		cmd, e := i.sql("SELECT value FROM properties WHERE handlerId='" + AppID(source) + "' AND name='command';")
		if e != nil || cmd != command(source) {
			return errors.New("应用注册命令已被修改，未覆盖")
		}
	}
	return nil
}

type change struct {
	path    string
	old     []byte
	existed bool
	next    []byte
	mode    os.FileMode
}

func (i *Installer) apply(changes []change, sql string) error {
	for idx := range changes {
		b, e := os.ReadFile(changes[idx].path)
		if e == nil {
			changes[idx].old = b
			changes[idx].existed = true
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	rollback := func() {
		for _, c := range changes {
			if c.existed {
				_ = library.AtomicWrite(c.path, c.old)
				_ = os.Chmod(c.path, c.mode)
			} else {
				_ = os.Remove(c.path)
			}
		}
		for _, s := range i.sources() {
			_ = os.Remove(i.path("/var/local/mesquite/" + AppID(s)))
		}
	}
	for _, c := range changes {
		if e := os.MkdirAll(filepath.Dir(c.path), 0755); e != nil {
			rollback()
			return e
		}
		var e error
		if c.next == nil {
			e = os.Remove(c.path)
			if os.IsNotExist(e) {
				e = nil
			}
		} else {
			e = library.AtomicWrite(c.path, c.next)
			if e == nil {
				e = os.Chmod(c.path, c.mode)
			}
		}
		if e != nil {
			rollback()
			return e
		}
	}
	if _, e := i.sql(sql); e != nil {
		rollback()
		return e
	}
	return nil
}
func (i *Installer) Install() error {
	if e := i.Preflight(); e != nil {
		return e
	}
	for _, s := range i.sources() {
		if e := i.checkOwned(s); e != nil {
			return e
		}
	}
	var changes []change
	var sql strings.Builder
	sql.WriteString("BEGIN IMMEDIATE; INSERT OR IGNORE INTO interfaces(interface) VALUES('application');")
	for _, s := range i.sources() {
		dir := i.path("/var/local/mesquite/" + AppID(s))
		changes = append(changes, change{path: filepath.Join(dir, ".kindle-read"), next: []byte(owner), mode: 0600}, change{path: filepath.Join(dir, "config.xml"), next: []byte(config(s)), mode: 0644}, change{path: filepath.Join(dir, "index.html"), next: []byte("<!doctype html><meta charset=utf-8><p>请从原生书架重新打开应用。</p>"), mode: 0600}, change{path: i.path("/mnt/us/documents/KindleRead-" + s + ".sh"), next: []byte(Scriptlet(s)), mode: 0755})
		fmt.Fprintf(&sql, "INSERT OR IGNORE INTO handlerIds(handlerId) VALUES('%s');", AppID(s))
		for _, p := range [][2]string{{"lipcId", AppID(s)}, {"command", command(s)}, {"supportedOrientation", "U"}, {"default-chrome-style", "NH"}, {"searchbar-mode", "transient"}, {"unloadPolicy", "unloadOnPause"}} {
			fmt.Fprintf(&sql, "INSERT OR REPLACE INTO properties(handlerId,name,value) VALUES('%s','%s','%s');", AppID(s), p[0], p[1])
		}
	}
	sql.WriteString("COMMIT;")
	return i.apply(changes, sql.String())
}
func (i *Installer) Uninstall() error {
	if e := i.Preflight(); e != nil {
		return e
	}
	for _, s := range i.sources() {
		if e := i.checkOwned(s); e != nil {
			return e
		}
	}
	var changes []change
	sql := "BEGIN IMMEDIATE;"
	for _, s := range i.sources() {
		dir := i.path("/var/local/mesquite/" + AppID(s))
		for _, name := range []string{"config.xml", "index.html", ".kindle-read"} {
			changes = append(changes, change{path: filepath.Join(dir, name), mode: 0600})
		}
		changes = append(changes, change{path: i.path("/mnt/us/documents/KindleRead-" + s + ".sh"), mode: 0755})
		sql += "DELETE FROM properties WHERE handlerId='" + AppID(s) + "'; DELETE FROM handlerIds WHERE handlerId='" + AppID(s) + "';"
	}
	if e := i.apply(changes, sql+"COMMIT;"); e != nil {
		return e
	}
	for _, s := range i.sources() {
		_ = os.Remove(i.path("/var/local/mesquite/" + AppID(s)))
	}
	return nil
}
