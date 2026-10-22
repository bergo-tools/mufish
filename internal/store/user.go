package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"mufish/internal/auth"
	"mufish/internal/model"
)

// ---------------------------------------------------------------------------
// 用户
// ---------------------------------------------------------------------------

const userCols = `u.id, u.username, u.password, u.name, u.phone, u.email, u.role_id,
	COALESCE(r.name,'') AS role_name, u.status, u.is_super, u.remark,
	u.last_login_at, u.created_at, u.updated_at`

// ListUsers 查询用户列表。
func (s *Store) ListUsers(ctx context.Context, keyword string, page, size int) (model.Page[model.User], error) {
	var out model.Page[model.User]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(keyword); kw != "" {
		where = append(where, "(u.username LIKE ? OR u.name LIKE ? OR u.phone LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	cond := strings.Join(where, " AND ")

	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM users u WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size = normPage(page, size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.User](ctx, s.db,
		`SELECT `+userCols+` FROM users u LEFT JOIN roles r ON r.id=u.role_id
		 WHERE `+cond+` ORDER BY u.id LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	out = model.Page[model.User]{List: list, Total: total, Page: page, Size: size}
	return out, nil
}

// GetUser 按 ID 获取用户。
func (s *Store) GetUser(ctx context.Context, id int64) (model.User, error) {
	u, ok, err := queryRow[model.User](ctx, s.db,
		`SELECT `+userCols+` FROM users u LEFT JOIN roles r ON r.id=u.role_id WHERE u.id=?`, id)
	if err != nil {
		return u, err
	}
	if !ok {
		return u, ErrNotFound
	}
	return u, nil
}

// GetUserByName 按登录名获取用户。
func (s *Store) GetUserByName(ctx context.Context, username string) (model.User, error) {
	u, ok, err := queryRow[model.User](ctx, s.db,
		`SELECT `+userCols+` FROM users u LEFT JOIN roles r ON r.id=u.role_id WHERE u.username=?`, username)
	if err != nil {
		return u, err
	}
	if !ok {
		return u, ErrNotFound
	}
	return u, nil
}

// UserInput 用户新增 / 修改入参。
type UserInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	RoleID   int64  `json:"role_id"`
	Status   int    `json:"status"`
	Remark   string `json:"remark"`
}

// CreateUser 新增用户。
func (s *Store) CreateUser(ctx context.Context, in UserInput) (int64, error) {
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		return 0, fmt.Errorf("%w: 登录名不能为空", ErrBadRequest)
	}
	if len(in.Password) < 6 {
		return 0, fmt.Errorf("%w: 口令至少 6 位", ErrBadRequest)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return 0, err
	}
	now := model.Now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(username, password, name, phone, email, role_id, status, remark, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		in.Username, hash, in.Name, in.Phone, in.Email, in.RoleID, normStatus(in.Status), in.Remark, now, now)
	if err != nil {
		return 0, wrapErr(err, "insert user")
	}
	return res.LastInsertId()
}

// UpdateUser 修改用户。
func (s *Store) UpdateUser(ctx context.Context, id int64, in UserInput) error {
	u, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if u.IsSuper && in.Status == 0 {
		return fmt.Errorf("%w: 内置超级管理员不可停用", ErrBadRequest)
	}
	now := model.Now()
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if strings.TrimSpace(in.Password) != "" {
			if len(in.Password) < 6 {
				return fmt.Errorf("%w: 口令至少 6 位", ErrBadRequest)
			}
			hash, err := auth.HashPassword(in.Password)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE users SET password=?, name=?, phone=?, email=?, role_id=?, status=?, remark=?, updated_at=? WHERE id=?`,
				hash, in.Name, in.Phone, in.Email, in.RoleID, normStatus(in.Status), in.Remark, now, id); err != nil {
				return wrapErr(err, "update user")
			}
			// 口令变化后必须让该用户所有旧会话失效；管理员在“编辑用户”里
			// 改口令与“重置口令”应保持同样的安全语义。
			if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, id); err != nil {
				return err
			}
			return nil
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET name=?, phone=?, email=?, role_id=?, status=?, remark=?, updated_at=? WHERE id=?`,
			in.Name, in.Phone, in.Email, in.RoleID, normStatus(in.Status), in.Remark, now, id); err != nil {
			return wrapErr(err, "update user")
		}
		return nil
	})
}

// DeleteUser 删除用户。
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	u, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if u.IsSuper {
		return fmt.Errorf("%w: 内置超级管理员不可删除", ErrBadRequest)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, id); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id); err != nil {
		return wrapErr(err, "delete user")
	}
	return nil
}

// UpdatePassword 修改口令。
func (s *Store) UpdatePassword(ctx context.Context, id int64, oldPwd, newPwd string) error {
	u, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if !auth.VerifyPassword(oldPwd, u.Password) {
		return fmt.Errorf("%w: 原口令不正确", ErrBadRequest)
	}
	if len(newPwd) < 6 {
		return fmt.Errorf("%w: 新口令至少 6 位", ErrBadRequest)
	}
	hash, err := auth.HashPassword(newPwd)
	if err != nil {
		return err
	}
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password=?, updated_at=? WHERE id=?`, hash, model.Now(), id); err != nil {
			return wrapErr(err, "update password")
		}
		// 口令变化后让所有已有会话失效，避免旧会话继续使用。
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, id); err != nil {
			return err
		}
		return nil
	})
}

// ResetPassword 由管理员重置口令。
func (s *Store) ResetPassword(ctx context.Context, id int64, newPwd string) error {
	if len(newPwd) < 6 {
		return fmt.Errorf("%w: 新口令至少 6 位", ErrBadRequest)
	}
	hash, err := auth.HashPassword(newPwd)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET password=?, updated_at=? WHERE id=?`, hash, model.Now(), id)
	if err != nil {
		return wrapErr(err, "reset password")
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, id)
	return nil
}

// Authenticate 校验登录并返回用户与权限集合。
func (s *Store) Authenticate(ctx context.Context, username, password string) (model.User, model.PermSet, error) {
	u, err := s.GetUserByName(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return u, nil, fmt.Errorf("%w: 用户名或口令错误", ErrBadRequest)
		}
		return u, nil, err
	}
	if !auth.VerifyPassword(password, u.Password) {
		return u, nil, fmt.Errorf("%w: 用户名或口令错误", ErrBadRequest)
	}
	if u.Status != 1 {
		return u, nil, fmt.Errorf("%w: 该账号已被停用，请联系管理员", ErrBadRequest)
	}
	perms, err := s.UserPerms(ctx, u)
	if err != nil {
		return u, nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE users SET last_login_at=? WHERE id=?`, model.Now(), u.ID)
	return u, perms, nil
}

// UserPerms 计算用户拥有的权限集合。
func (s *Store) UserPerms(ctx context.Context, u model.User) (model.PermSet, error) {
	if u.IsSuper {
		return model.NewPermSet([]string{model.SuperPerm}), nil
	}
	if u.RoleID == 0 {
		return model.NewPermSet(nil), nil
	}
	rows, err := query[permRow](ctx, s.db, `SELECT perm FROM role_permissions WHERE role_id=?`, u.RoleID)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(rows))
	for _, r := range rows {
		codes = append(codes, r.Perm)
	}
	return model.NewPermSet(codes), nil
}

type permRow struct {
	Perm string `db:"perm"`
}

// ---------------------------------------------------------------------------
// 会话
// ---------------------------------------------------------------------------

// CreateSession 创建会话并返回令牌。
func (s *Store) CreateSession(ctx context.Context, userID int64, ip, ua string, ttl time.Duration) (string, time.Time, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	exp := time.Now().Add(ttl)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions(token, user_id, ip, user_agent, created_at, expires_at) VALUES(?,?,?,?,?,?)`,
		token, userID, ip, ua, model.Now(), exp.Format(model.TimeLayout)); err != nil {
		return "", time.Time{}, err
	}
	return token, exp, nil
}

// SessionUser 按令牌取用户。
func (s *Store) SessionUser(ctx context.Context, token string) (model.User, bool) {
	if token == "" {
		return model.User{}, false
	}
	var userID int64
	var expires string
	err := s.db.QueryRowContext(ctx, `SELECT user_id, expires_at FROM sessions WHERE token=?`, token).Scan(&userID, &expires)
	if err != nil {
		return model.User{}, false
	}
	if exp, err := time.ParseInLocation(model.TimeLayout, expires, time.Local); err == nil {
		if time.Now().After(exp) {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token=?`, token)
			return model.User{}, false
		}
	}
	u, err := s.GetUser(ctx, userID)
	if err != nil || u.Status != 1 {
		return model.User{}, false
	}
	return u, true
}

// DeleteSession 注销会话。
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token=?`, token)
	return err
}

// CleanSessions 清理过期会话。
func (s *Store) CleanSessions(ctx context.Context) {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, model.Now())
}

// ---------------------------------------------------------------------------
// 角色
// ---------------------------------------------------------------------------

// ListRoles 全部角色（含权限）。
func (s *Store) ListRoles(ctx context.Context) ([]model.Role, error) {
	roles, err := query[model.Role](ctx, s.db,
		`SELECT id, code, name, remark, is_system, created_at, updated_at,
		   (SELECT COUNT(*) FROM users u WHERE u.role_id = roles.id) AS user_count
		 FROM roles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		perms, err := query[permRow](ctx, s.db, `SELECT perm FROM role_permissions WHERE role_id=?`, roles[i].ID)
		if err != nil {
			return nil, err
		}
		roles[i].Perms = make([]string, 0, len(perms))
		for _, p := range perms {
			roles[i].Perms = append(roles[i].Perms, p.Perm)
		}
	}
	return roles, nil
}

// RoleInput 角色入参。
type RoleInput struct {
	Code   string   `json:"code"`
	Name   string   `json:"name"`
	Remark string   `json:"remark"`
	Perms  []string `json:"perms"`
}

// GetRole 按 ID 取角色。
func (s *Store) GetRole(ctx context.Context, id int64) (model.Role, error) {
	r, ok, err := queryRow[model.Role](ctx, s.db,
		`SELECT id, code, name, remark, is_system, created_at, updated_at FROM roles WHERE id=?`, id)
	if err != nil {
		return r, err
	}
	if !ok {
		return r, ErrNotFound
	}
	perms, err := query[permRow](ctx, s.db, `SELECT perm FROM role_permissions WHERE role_id=?`, id)
	if err != nil {
		return r, err
	}
	for _, p := range perms {
		r.Perms = append(r.Perms, p.Perm)
	}
	return r, nil
}

// CreateRole 新增角色。
func (s *Store) CreateRole(ctx context.Context, in RoleInput) (int64, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return 0, fmt.Errorf("%w: 角色编码与名称不能为空", ErrBadRequest)
	}
	now := model.Now()
	var id int64
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO roles(code, name, remark, is_system, created_at, updated_at) VALUES(?,?,?,0,?,?)`,
			in.Code, in.Name, in.Remark, now, now)
		if err != nil {
			return wrapErr(err, "insert role")
		}
		id, _ = res.LastInsertId()
		return saveRolePerms(ctx, tx, id, in.Perms)
	})
	return id, err
}

// UpdateRole 修改角色。
func (s *Store) UpdateRole(ctx context.Context, id int64, in RoleInput) error {
	r, err := s.GetRole(ctx, id)
	if err != nil {
		return err
	}
	if r.Code == "admin" {
		return fmt.Errorf("%w: 内置管理员角色不可修改", ErrBadRequest)
	}
	now := model.Now()
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE roles SET name=?, remark=?, updated_at=? WHERE id=?`, in.Name, in.Remark, now, id); err != nil {
			return wrapErr(err, "update role")
		}
		return saveRolePerms(ctx, tx, id, in.Perms)
	})
}

// DeleteRole 删除角色。
func (s *Store) DeleteRole(ctx context.Context, id int64) error {
	r, err := s.GetRole(ctx, id)
	if err != nil {
		return err
	}
	if r.IsSystem {
		return fmt.Errorf("%w: 内置角色不可删除", ErrBadRequest)
	}
	var used int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role_id=?`, id).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("%w: 仍有 %d 个用户使用该角色", ErrInUse, used)
	}
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM roles WHERE id=?`, id)
		return err
	})
}

func saveRolePerms(ctx context.Context, tx *sql.Tx, roleID int64, perms []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=?`, roleID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, p := range perms {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] || !model.IsValidPerm(p) {
			continue
		}
		seen[p] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO role_permissions(role_id, perm) VALUES(?,?)`, roleID, p); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 操作日志
// ---------------------------------------------------------------------------

// AddLog 记录操作日志。
func (s *Store) AddLog(ctx context.Context, userID int64, username, module, action, detail, ip string) {
	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO audit_logs(user_id, username, module, action, detail, ip, created_at) VALUES(?,?,?,?,?,?,?)`,
		userID, username, module, action, detail, ip, model.Now())
}

// LogQuery 日志查询条件。
type LogQuery struct {
	Keyword string
	Module  string
	From    string
	To      string
	Page    int
	Size    int
}

// ListLogs 查询操作日志。
func (s *Store) ListLogs(ctx context.Context, q LogQuery) (model.Page[model.AuditLog], error) {
	var out model.Page[model.AuditLog]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(username LIKE ? OR detail LIKE ? OR action LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	if q.Module != "" {
		where = append(where, "module=?")
		args = append(args, q.Module)
	}
	if q.From != "" {
		where = append(where, "created_at >= ?")
		args = append(args, q.From+" 00:00:00")
	}
	if q.To != "" {
		where = append(where, "created_at <= ?")
		args = append(args, q.To+" 23:59:59")
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM audit_logs WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.AuditLog](ctx, s.db,
		`SELECT id, user_id, username, module, action, detail, ip, created_at
		 FROM audit_logs WHERE `+cond+` ORDER BY id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.AuditLog]{List: list, Total: total, Page: page, Size: size}, nil
}

// LogModules 返回出现过的模块列表。
func (s *Store) LogModules(ctx context.Context) ([]string, error) {
	rows, err := query[settingRow](ctx, s.db, `SELECT DISTINCT module AS value FROM audit_logs ORDER BY module`)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Value)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

func normPage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 500 {
		size = 500
	}
	return page, size
}

func normStatus(v int) int {
	if v == 0 {
		return 0
	}
	return 1
}

func countOf(ctx context.Context, db execer, sqlStr string, args ...any) (int64, error) {
	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return 0, wrapErr(err, sqlStr)
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var n int64
	if err := rows.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
