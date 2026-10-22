// 木鱼进销存 —— 面向贸易型小企业的单机进销存软件。
//
// 单文件运行：内嵌前端资源，数据保存在本地 SQLite 文件中。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"mufish/internal/api"
	"mufish/internal/store"
	"mufish/web"
)

// version 由构建脚本通过 -ldflags 注入。
var version = "0.11.0"

func main() {
	var (
		addr    = flag.String("addr", "127.0.0.1:8899", "监听地址，例如 0.0.0.0:8899")
		dataDir = flag.String("data", "data", "数据目录，存放 SQLite 数据库与备份")
		open    = flag.Bool("open", defaultOpenBrowser(runtime.GOOS), "启动后自动打开浏览器（Windows/macOS 默认开启，Linux 默认关闭）")
		showVer = flag.Bool("version", false, "显示版本号")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("木鱼进销存 v%s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[木鱼] ")

	if err := run(*addr, *dataDir, *open); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}

func run(addr, dataDir string, openBrowser bool) error {
	absData, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absData, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	dbPath := filepath.Join(absData, "mufish.db")

	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	assets, err := web.Dist()
	if err != nil {
		log.Printf("警告: 未能加载内嵌前端资源: %v", err)
	}

	handler := api.New(api.Options{
		Store:   st,
		Assets:  assets,
		Version: version,
		DataDir: absData,
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 定期清理过期会话
	stopClean := make(chan struct{})
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				st.CleanSessions(context.Background())
			case <-stopClean:
				return
			}
		}
	}()

	// 先完成监听，再打开浏览器；端口占用时直接报错，不会先弹出一个打不开的页面。
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		close(stopClean)
		return fmt.Errorf("监听地址 %s 失败: %w", addr, err)
	}
	defer ln.Close()

	url := displayURL(ln.Addr().String())
	printBanner(url, dbPath, absData)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	if openBrowser {
		go func() {
			// 给 Serve 一点点启动时间，避免浏览器抢先请求时页面空白。
			time.Sleep(300 * time.Millisecond)
			if err := openURL(url); err != nil {
				log.Printf("自动打开浏览器失败：%v", err)
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		close(stopClean)
		return err
	case <-sigCh:
		log.Println("正在关闭服务…")
		close(stopClean)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("关闭服务时出错: %v", err)
		}
		log.Println("已安全退出，数据已保存。")
	}
	return nil
}

// defaultOpenBrowser 按平台决定是否默认打开浏览器。
// Windows/macOS 的桌面用户双击即用；Linux 默认不主动弹窗，可用 -open 开启。
func defaultOpenBrowser(goos string) bool {
	return goos == "windows" || goos == "darwin"
}

func displayURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "http://" + host + ":" + port
}

func printBanner(url, dbPath, dataDir string) {
	fmt.Println()
	fmt.Println("  ┌──────────────────────────────────────────────┐")
	fmt.Println("  │            木鱼进销存  Mufish                │")
	fmt.Println("  └──────────────────────────────────────────────┘")
	fmt.Printf("   版本    : v%s\n", version)
	fmt.Printf("   访问地址: %s\n", url)
	fmt.Printf("   数据库  : %s\n", dbPath)
	fmt.Printf("   数据目录: %s\n", dataDir)
	fmt.Println("   默认账号: admin / admin123 （请登录后立即修改）")
	fmt.Println("   按 Ctrl+C 退出")
	fmt.Println()
}

// browserCommand 返回在指定平台打开 URL 的命令和参数，便于单元测试。
func browserCommand(goos, url string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		return "xdg-open", []string{url}
	}
}

func openURL(url string) error {
	name, args := browserCommand(runtime.GOOS, url)
	return exec.Command(name, args...).Start()
}
