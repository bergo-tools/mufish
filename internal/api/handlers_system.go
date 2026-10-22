package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"mufish/internal/model"
	"mufish/internal/store"
)

// ---------------------------------------------------------------------------
// 用户
// ---------------------------------------------------------------------------

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListUsers(r.Context(), qs(r, "keyword"), qi(r, "page", 1), qi(r, "size", 20))
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) error {
	var in store.UserInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.st.CreateUser(r.Context(), in)
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "用户管理", "新增", in.Username, clientIP(r))
	writeOK(w, map[string]any{"id": id})
	return nil
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	target, err := h.st.GetUser(r.Context(), id)
	if err != nil {
		return err
	}
	if target.IsSuper && !user(r).IsSuper {
		return store.NewError(http.StatusForbidden, "只有超级管理员可以修改超级管理员账号")
	}
	var in store.UserInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.st.UpdateUser(r.Context(), id, in); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "用户管理", "修改", in.Username, clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if id == user(r).ID {
		return store.NewError(400, "不能删除当前登录账号")
	}
	target, _ := h.st.GetUser(r.Context(), id)
	if err := h.st.DeleteUser(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "用户管理", "删除", target.Username, clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	target, err := h.st.GetUser(r.Context(), id)
	if err != nil {
		return err
	}
	if target.IsSuper && !user(r).IsSuper {
		return store.NewError(http.StatusForbidden, "只有超级管理员可以重置超级管理员口令")
	}
	if err := h.st.ResetPassword(r.Context(), id, in.Password); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "用户管理", "重置口令", target.Username, clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 角色与权限
// ---------------------------------------------------------------------------

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.ListRoles(r.Context())
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) permissions(w http.ResponseWriter, r *http.Request) error {
	writeOK(w, map[string]any{
		"catalog": model.PermCatalog,
		"all":     model.AllPermCodes(),
		"super":   model.SuperPerm,
	})
	return nil
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) error {
	var in store.RoleInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.st.CreateRole(r.Context(), in)
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "角色权限", "新增", in.Name, clientIP(r))
	writeOK(w, map[string]any{"id": id})
	return nil
}

func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in store.RoleInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.st.UpdateRole(r.Context(), id, in); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "角色权限", "修改", in.Name, clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	role, _ := h.st.GetRole(r.Context(), id)
	if err := h.st.DeleteRole(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "角色权限", "删除", role.Name, clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 操作日志
// ---------------------------------------------------------------------------

func (h *Handler) listLogs(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListLogs(r.Context(), store.LogQuery{
		Keyword: qs(r, "keyword"),
		Module:  qs(r, "module"),
		From:    qs(r, "from"),
		To:      qs(r, "to"),
		Page:    qi(r, "page", 1),
		Size:    qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) logModules(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.LogModules(r.Context())
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

// ---------------------------------------------------------------------------
// 系统参数
// ---------------------------------------------------------------------------

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) error {
	m, err := h.st.Settings(r.Context())
	if err != nil {
		return err
	}
	writeOK(w, m)
	return nil
}

func (h *Handler) saveSettings(w http.ResponseWriter, r *http.Request) error {
	var in map[string]string
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	if v, ok := in["session_hours"]; ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n <= 0 || n > 24*365 {
			return store.NewError(http.StatusBadRequest, "登录保持小时数必须是 1～8760 之间的整数")
		}
	}
	if err := h.st.SaveSettings(r.Context(), in); err != nil {
		return err
	}
	detail, _ := json.Marshal(in)
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "系统参数", "修改", string(detail), clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 系统信息与备份
// ---------------------------------------------------------------------------

func (h *Handler) systemInfo(w http.ResponseWriter, r *http.Request) error {
	dbPath := h.st.Path()
	var size int64
	if fi, err := os.Stat(dbPath); err == nil {
		size = fi.Size()
	}
	var walSize int64
	if fi, err := os.Stat(dbPath + "-wal"); err == nil {
		walSize = fi.Size()
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	backups, _ := h.listBackupFiles()
	writeOK(w, map[string]any{
		"version":      h.version,
		"go_version":   runtime.Version(),
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"db_path":      dbPath,
		"db_size":      size,
		"wal_size":     walSize,
		"data_dir":     h.dataDir,
		"uptime":       int64(time.Since(h.started).Seconds()),
		"started_at":   h.started.Format(model.TimeLayout),
		"goroutines":   runtime.NumGoroutine(),
		"memory_alloc": int64(m.Alloc),
		"backups":      backups,
		"now":          model.Now(),
	})
	return nil
}

// BackupFile 备份文件信息。
type BackupFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func (h *Handler) backupDir() string {
	return filepath.Join(h.dataDir, "backups")
}

func (h *Handler) listBackupFiles() ([]BackupFile, error) {
	dir := h.backupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupFile{}, nil
		}
		return nil, err
	}
	out := make([]BackupFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupFile{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(model.TimeLayout),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

func (h *Handler) listBackups(w http.ResponseWriter, r *http.Request) error {
	files, err := h.listBackupFiles()
	if err != nil {
		return err
	}
	writeOK(w, files)
	return nil
}

// backup 生成数据库备份文件。
func (h *Handler) backup(w http.ResponseWriter, r *http.Request) error {
	dir := h.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}
	name := "mufish-" + nowStamp() + ".db"
	dest := filepath.Join(dir, name)
	if err := h.st.BackupTo(r.Context(), dest); err != nil {
		return fmt.Errorf("备份失败: %w", err)
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "数据备份", "备份", name, clientIP(r))
	info, _ := os.Stat(dest)
	var size int64
	if info != nil {
		size = info.Size()
	}
	writeOK(w, map[string]any{"name": name, "size": size, "path": dest})
	return nil
}

// restore 从备份文件恢复数据库。
func (h *Handler) restore(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	name := filepath.Base(strings.TrimSpace(in.Name))
	if name == "" || !strings.HasSuffix(name, ".db") {
		return store.NewError(400, "请选择有效的备份文件")
	}
	src := filepath.Join(h.backupDir(), name)
	if _, err := os.Stat(src); err != nil {
		return store.NewError(404, "备份文件不存在")
	}
	u := user(r)
	if err := h.st.RestoreFrom(r.Context(), src); err != nil {
		return fmt.Errorf("恢复失败: %w", err)
	}
	h.st.AddLog(r.Context(), u.ID, u.Username, "数据备份", "恢复", name, clientIP(r))
	writeOK(w, map[string]any{"restored": name})
	return nil
}
