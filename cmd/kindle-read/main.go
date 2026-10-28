// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xinhang725-code/kindle-read/internal/device"
	"github.com/xinhang725-code/kindle-read/internal/library"
	"github.com/xinhang725-code/kindle-read/internal/web"
)

type session struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

func lock(path string) (*os.File, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("此应用已有运行中的进程，请稍后重试")
	}
	return f, nil
}
func lockApps(data string) (func(), error) {
	var held []*os.File
	release := func() {
		for _, f := range held {
			_ = f.Close()
		}
	}
	for _, source := range device.Sources {
		for _, suffix := range []string{"-launch.lock", ".lock"} {
			f, e := lock(filepath.Join(data, "run", source+suffix))
			if e != nil {
				release()
				return nil, errors.New("应用仍在运行，请退出三个应用或等待五分钟空闲结束后重试")
			}
			held = append(held, f)
		}
	}
	return release, nil
}
func serve(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	source := flags.String("source", "qidian", "qidian, changpei, miuread")
	data := flags.String("data", ".local-data", "private data directory")
	listen := flags.String("listen", "127.0.0.1:0", "loopback address")
	ready := flags.String("ready", "", "private session file")
	bootstrap := flags.String("bootstrap", "", "Mesquite bootstrap HTML")
	onDevice := flags.Bool("device", false, "enable Kindle home command")
	archive := flags.String("source-archive", "", "corresponding source archive")
	idle := flags.Duration("idle", 5*time.Minute, "idle shutdown timeout")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if !library.ValidSource(*source) {
		return errors.New("未知来源")
	}
	host, _, e := net.SplitHostPort(*listen)
	if e != nil || host != "127.0.0.1" {
		return errors.New("只能监听 127.0.0.1")
	}
	if *idle < time.Second {
		return errors.New("idle 必须至少一秒")
	}
	held, e := lock(filepath.Join(*data, "run", *source+".lock"))
	if e != nil {
		return e
	}
	defer held.Close()
	app, e := web.New(*data, *source)
	if e != nil {
		return e
	}
	app.SourceArchive = *archive
	listener, e := net.Listen("tcp", *listen)
	if e != nil {
		return e
	}
	defer listener.Close()
	app.Host = listener.Addr().String()
	link := "http://" + app.Host + app.Prefix
	server := &http.Server{Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	stop := make(chan struct{}, 1)
	app.Exit = func() {
		if *onDevice {
			_ = exec.Command("/usr/bin/lipc-set-prop", "com.lab126.appmgrd", "start", "app://com.lab126.booklet.home").Run()
		}
		select {
		case stop <- struct{}{}:
		default:
		}
	}
	if *bootstrap != "" {
		b := []byte("<!doctype html><meta charset=utf-8><meta http-equiv=\"refresh\" content=\"0;url=" + html.EscapeString(link) + "\"><p>正在打开阅读器…</p>")
		if e = library.AtomicWrite(*bootstrap, b); e != nil {
			return e
		}
	}
	if *ready != "" {
		b, _ := json.Marshal(session{URL: link, Source: *source})
		if e = library.AtomicWrite(*ready, b); e != nil {
			return e
		}
		defer os.Remove(*ready)
	}
	if !*onDevice {
		fmt.Println(link)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	running := true
	for running {
		select {
		case <-signals:
			running = false
		case <-stop:
			running = false
		case e := <-done:
			if !errors.Is(e, http.ErrServerClosed) {
				return e
			}
			running = false
		case <-ticker.C:
			if time.Since(time.Unix(app.LastSeen.Load(), 0)) > *idle {
				running = false
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
func healthy(path, source string) bool {
	raw, e := os.ReadFile(path)
	if e != nil || len(raw) > 4096 {
		return false
	}
	var value session
	if json.Unmarshal(raw, &value) != nil || value.Source != source {
		return false
	}
	u, e := url.Parse(value.URL)
	if e != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || !strings.HasPrefix(u.Path, "/s/") {
		return false
	}
	client := http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect rejected") }}
	req, e := http.NewRequest("GET", value.URL+"api/health", nil)
	if e != nil {
		return false
	}
	req.Header.Set("X-Kindle-Read", "1")
	res, e := client.Do(req)
	if e != nil {
		return false
	}
	defer res.Body.Close()
	var body struct {
		Source string `json:"source"`
	}
	return res.StatusCode == 200 && json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&body) == nil && body.Source == source
}
func launch(source string) error {
	if !library.ValidSource(source) {
		return errors.New("未知应用")
	}
	if e := (&device.Installer{Root: "/"}).Preflight(); e != nil {
		return e
	}
	data := device.Bundle + "/data"
	run := filepath.Join(data, "run")
	held, e := lock(filepath.Join(run, source+"-launch.lock"))
	if e != nil {
		return e
	}
	defer held.Close()
	ready := filepath.Join(run, source+".json")
	if !healthy(ready, source) {
		executable, e := os.Executable()
		if e != nil {
			return e
		}
		cmd := exec.Command(executable, "serve", "--source", source, "--data", data, "--ready", ready, "--bootstrap", "/var/local/mesquite/"+device.AppID(source)+"/index.html", "--source-archive", device.Bundle+"/source.tar.gz", "--device")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		log, e := os.OpenFile(filepath.Join(run, source+".log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if e != nil {
			return e
		}
		defer log.Close()
		cmd.Stdout = log
		cmd.Stderr = log
		if e = cmd.Start(); e != nil {
			return e
		}
		_ = cmd.Process.Release()
		available := false
		for attempt := 0; attempt < 40; attempt++ {
			if healthy(ready, source) {
				available = true
				break
			}
			time.Sleep(125 * time.Millisecond)
		}
		if !available {
			return errors.New("阅读服务启动失败，请查看本应用 data/run 下的日志；未终止其他进程")
		}
	}
	return exec.Command("/usr/bin/lipc-set-prop", "com.lab126.appmgrd", "start", "app://"+device.AppID(source)).Run()
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("用法：kindle-read serve [选项] | launch <应用> | preflight | install | uninstall")
	}
	switch args[0] {
	case "serve":
		return serve(args[1:])
	case "launch":
		if len(args) != 2 {
			return errors.New("launch 需要指定应用")
		}
		return launch(args[1])
	case "preflight", "install", "uninstall":
		source := ""
		if len(args) == 2 && library.ValidSource(args[1]) {
			source = args[1]
		} else if len(args) != 1 {
			return errors.New("可选参数为 qidian、changpei 或 miuread")
		}
		i := &device.Installer{Root: "/", Source: source}
		var e error
		if args[0] == "preflight" {
			e = i.Preflight()
		} else {
			var held *os.File
			held, e = lock(device.Bundle + "/data/run/install.lock")
			if e != nil {
				return e
			}
			defer held.Close()
			release, lockError := lockApps(device.Bundle + "/data")
			if lockError != nil {
				return lockError
			}
			defer release()
			if args[0] == "install" {
				e = i.Install()
			} else {
				e = i.Uninstall()
			}
		}
		if e == nil {
			fmt.Println("操作完成。运行组件检查不代表已验证 Vera 书架索引或真机兼容性。")
		}
		return e
	default:
		return errors.New("未知命令")
	}
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
