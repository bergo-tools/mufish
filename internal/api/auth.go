package api

import (
	"net/http"
	"time"

	"mufish/internal/model"
	"mufish/internal/store"
)

// login 用户登录。
func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	u, perms, err := h.st.Authenticate(r.Context(), in.Username, in.Password)
	if err != nil {
		h.st.AddLog(r.Context(), 0, in.Username, "认证", "登录失败", err.Error(), clientIP(r))
		return store.NewError(http.StatusUnauthorized, err.Error())
	}
	hours := h.st.SettingInt(r.Context(), "session_hours", 72)
	if hours <= 0 || hours > 24*365 {
		hours = 72
	}
	token, exp, err := h.st.CreateSession(r.Context(), u.ID, clientIP(r), r.UserAgent(), time.Duration(hours)*time.Hour)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   hours * 3600,
	})
	h.st.AddLog(r.Context(), u.ID, u.Username, "认证", "登录成功", "", clientIP(r))
	writeOK(w, map[string]any{
		"token":       token,
		"expires_at":  exp.Format(model.TimeLayout),
		"user":        u,
		"permissions": perms.List(),
	})
	return nil
}

// logout 退出登录。
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	token := ""
	if c, err := r.Cookie(h.cookie); err == nil {
		token = c.Value
	}
	if token == "" {
		token = r.Header.Get("X-Auth-Token")
	}
	if token != "" {
		if u, ok := h.st.SessionUser(r.Context(), token); ok {
			h.st.AddLog(r.Context(), u.ID, u.Username, "认证", "退出登录", "", clientIP(r))
		}
		_ = h.st.DeleteSession(r.Context(), token)
	}
	http.SetCookie(w, &http.Cookie{
		Name: h.cookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
	})
	writeOK(w, nil)
	return nil
}

// me 当前登录用户信息。
func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	u := user(r)
	if u.ID == 0 {
		return store.NewError(http.StatusUnauthorized, "未登录")
	}
	perms, _ := r.Context().Value(ctxPerms).(model.PermSet)
	writeOK(w, map[string]any{
		"user":        u,
		"permissions": perms.List(),
	})
	return nil
}

// changePassword 修改自己的口令。
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	u := user(r)
	if err := h.st.UpdatePassword(r.Context(), u.ID, in.OldPassword, in.NewPassword); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), u.ID, u.Username, "认证", "修改口令", "", clientIP(r))
	writeOK(w, nil)
	return nil
}
