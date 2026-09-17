package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"mufish/internal/model"
)

//go:embed schema.sql
var schemaSQL string

// Store 封装数据库访问。
type Store struct {
	db   *sql.DB
	path string
}

// DB 返回底层连接。
func (s *Store) DB() *sql.DB { return s.db }

// Path 返回数据库文件路径。
func (s *Store) Path() string { return s.path }

// Open 打开（不存在则创建）数据库并完成建表与初始化数据。
func Open(path string) (*Store, error) {
	dsn := path
	if !strings.HasPrefix(path, "file:") {
		dsn = "file:" + path
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	// WAL 提升并发读性能；单机场景下写入串行化，避免 SQLITE_BUSY。
	dsn += sep + strings.Join([]string{
		"_pragma=busy_timeout(10000)",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(1)",
		"_pragma=temp_store(memory)",
	}, "&")

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	// 单机场景：串行化访问，杜绝锁冲突。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	s := &Store{db: db, path: path}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seed(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// BackupTo 使用 SQLite 在线备份能力导出数据库副本。
func (s *Store) BackupTo(ctx context.Context, dest string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, dest)
	return err
}

// RestoreFrom 用指定文件替换当前数据库并重新打开。
func (s *Store) RestoreFrom(ctx context.Context, src string) error {
	if err := s.db.Close(); err != nil {
		return err
	}
	// 保留一份回滚副本
	_ = copyFile(s.path, s.path+".before-restore")
	if err := copyFile(src, s.path); err != nil {
		reopened, rerr := Open(s.path)
		if rerr == nil {
			*s = *reopened
		}
		return err
	}
	// 清理 WAL / SHM，避免旧日志干扰
	_ = removeIfExists(s.path + "-wal")
	_ = removeIfExists(s.path + "-shm")

	reopened, err := Open(s.path)
	if err != nil {
		return fmt.Errorf("恢复后重新打开数据库失败: %w", err)
	}
	*s = *reopened
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("初始化数据库结构失败: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 通用查询助手
// ---------------------------------------------------------------------------

// scanAll 把查询结果按 db tag 映射到结构体切片。
func scanAll[T any](rows *sql.Rows) ([]T, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, 16)
	for rows.Next() {
		var item T
		if err := scanInto(rows, cols, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// scanOne 读取单行，无数据返回 (零值, false, nil)。
func scanOne[T any](rows *sql.Rows) (T, bool, error) {
	defer rows.Close()
	var item T
	if !rows.Next() {
		return item, false, rows.Err()
	}
	cols, err := rows.Columns()
	if err != nil {
		return item, false, err
	}
	if err := scanInto(rows, cols, &item); err != nil {
		return item, false, err
	}
	return item, true, rows.Err()
}

func scanInto(rows *sql.Rows, cols []string, dst any) error {
	rv := reflect.ValueOf(dst).Elem()
	rt := rv.Type()
	idx := make(map[string]int, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		if tag := rt.Field(i).Tag.Get("db"); tag != "" {
			idx[tag] = i
		}
	}
	var fixups []func()
	var discard any
	ptrs := make([]any, len(cols))
	for i, c := range cols {
		fi, ok := idx[c]
		if !ok {
			ptrs[i] = &discard
			continue
		}
		f := rv.Field(fi)
		if !f.CanAddr() {
			ptrs[i] = &discard
			continue
		}
		switch f.Kind() {
		case reflect.String:
			var tmp sql.NullString
			ptrs[i] = &tmp
			fv := f
			fixups = append(fixups, func() {
				if tmp.Valid {
					fv.SetString(tmp.String)
				}
			})
		case reflect.Bool:
			var tmp sql.NullInt64
			ptrs[i] = &tmp
			fv := f
			fixups = append(fixups, func() { fv.SetBool(tmp.Valid && tmp.Int64 != 0) })
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var tmp sql.NullInt64
			ptrs[i] = &tmp
			fv := f
			fixups = append(fixups, func() {
				if tmp.Valid {
					fv.SetInt(tmp.Int64)
				} else {
					fv.SetInt(0)
				}
			})
		case reflect.Float32, reflect.Float64:
			var tmp sql.NullFloat64
			ptrs[i] = &tmp
			fv := f
			fixups = append(fixups, func() {
				if tmp.Valid {
					fv.SetFloat(tmp.Float64)
				} else {
					fv.SetFloat(0)
				}
			})
		default:
			// 实现了 sql.Scanner 的自定义类型（Money / Price / Qty）
			ptrs[i] = f.Addr().Interface()
		}
	}
	if err := rows.Scan(ptrs...); err != nil {
		return err
	}
	for _, fn := range fixups {
		fn()
	}
	return nil
}

// query 执行查询并返回结构体切片。
func query[T any](ctx context.Context, db execer, sqlStr string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, wrapErr(err, sqlStr)
	}
	return scanAll[T](rows)
}

// queryRow 执行查询并返回单条记录。
func queryRow[T any](ctx context.Context, db execer, sqlStr string, args ...any) (T, bool, error) {
	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		var zero T
		return zero, false, wrapErr(err, sqlStr)
	}
	return scanOne[T](rows)
}

type execer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func wrapErr(err error, sqlStr string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("%w", ErrDuplicate)
	}
	return fmt.Errorf("%v [%s]", err, firstLine(sqlStr))
}

func firstLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 90 {
		return s[:90] + "…"
	}
	return s
}

// 常见业务错误。
var (
	ErrNotFound   = errors.New("记录不存在")
	ErrDuplicate  = errors.New("编码或名称已存在，请更换后重试")
	ErrInUse      = errors.New("该记录已被引用，无法删除")
	ErrBadRequest = errors.New("请求参数有误")
)

// Error 携带 HTTP 状态码的业务错误。
type Error struct {
	Status int
	Msg    string
}

func (e *Error) Error() string { return e.Msg }

// NewError 构造带状态码的错误。
func NewError(status int, msg string) error { return &Error{Status: status, Msg: msg} }

// ---------------------------------------------------------------------------
// 事务助手
// ---------------------------------------------------------------------------

// Tx 在事务中执行 fn，出错自动回滚。
func (s *Store) Tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// nextDocNo 生成单据编号，如 PO20250917-0001。
func nextDocNo(ctx context.Context, tx *sql.Tx, kind, prefix, day string) (string, error) {
	compact := strings.ReplaceAll(day, "-", "")
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO doc_seqs(kind, day, seq) VALUES(?, ?, 1)
		 ON CONFLICT(kind, day) DO UPDATE SET seq = seq + 1`, kind, day); err != nil {
		return "", err
	}
	var seq int
	if err := tx.QueryRowContext(ctx, `SELECT seq FROM doc_seqs WHERE kind=? AND day=?`, kind, day).Scan(&seq); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s-%04d", prefix, compact, seq), nil
}

// ---------------------------------------------------------------------------
// 设置
// ---------------------------------------------------------------------------

// Setting 读取单个配置项。
func (s *Store) Setting(ctx context.Context, key string) string {
	v, _, err := queryRow[settingRow](ctx, s.db, `SELECT value FROM settings WHERE key=?`, key)
	if err != nil {
		return ""
	}
	return v.Value
}

type settingRow struct {
	Value string `db:"value"`
}

// Settings 读取全部配置。
func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	rows, err := query[settingRow2](ctx, s.db, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

type settingRow2 struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

// SaveSettings 保存配置项。
func (s *Store) SaveSettings(ctx context.Context, kv map[string]string) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		for k, v := range kv {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO settings(key, value) VALUES(?, ?)
				 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

// SettingInt 读取整型配置。
func (s *Store) SettingInt(ctx context.Context, key string, def int) int {
	v := s.Setting(ctx, key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// SettingBool 读取布尔配置。
func (s *Store) SettingBool(ctx context.Context, key string, def bool) bool {
	v := s.Setting(ctx, key)
	if v == "" {
		return def
	}
	return v == "1" || strings.EqualFold(v, "true")
}

var _ = model.Money(0)

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("写入目标文件失败: %w", err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func removeIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return os.Remove(path)
}
