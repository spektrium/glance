package glance

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var adminPageTemplate = mustParseTemplate("admin.html", "document.html", "footer.html")

func (a *application) registerAdminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", a.handleAdminPageRequest)
	mux.HandleFunc("GET /api/admin/overview", a.handleAdminOverview)
	mux.HandleFunc("GET /api/admin/users", a.handleAdminListUsers)
	mux.HandleFunc("POST /api/admin/users", a.handleAdminCreateUser)
	mux.HandleFunc("PATCH /api/admin/users/{username}", a.handleAdminUpdateUser)
	mux.HandleFunc("DELETE /api/admin/users/{username}", a.handleAdminDeleteUser)
	mux.HandleFunc("GET /api/admin/settings", a.handleAdminGetSettings)
	mux.HandleFunc("PUT /api/admin/settings", a.handleAdminSaveSettings)
	mux.HandleFunc("GET /api/admin/theme", a.handleAdminGetTheme)
	mux.HandleFunc("PUT /api/admin/theme", a.handleAdminSaveTheme)
	mux.HandleFunc("GET /api/admin/pages", a.handleAdminListPages)
	mux.HandleFunc("POST /api/admin/pages", a.handleAdminCreatePage)
	mux.HandleFunc("PATCH /api/admin/pages/{page}", a.handleAdminUpdatePage)
	mux.HandleFunc("DELETE /api/admin/pages/{page}", a.handleAdminDeletePage)
	mux.HandleFunc("PUT /api/admin/pages/order", a.handleAdminReorderPages)
}

func (a *application) handleAdminPageRequest(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, redirectToLogin) {
		return
	}

	data := templateData{App: a, Request: templateRequestData{IsAdminUI: true}}
	a.populateTemplateRequestData(&data.Request, r)

	var responseBytes []byte
	var buf strings.Builder
	if err := adminPageTemplate.Execute(&buf, data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	responseBytes = []byte(buf.String())
	w.Write(responseBytes)
}

func (a *application) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"username":     a.currentUsername(r),
		"requiresAuth": a.RequiresAuth,
		"canManage":    a.canManage(r),
		"pages":        len(a.Config.Pages),
		"users":        len(a.Config.Auth.Users),
		"theme":        a.Config.Theme.Key,
		"branding":     a.Config.Branding.AppName,
		"configPath":   a.configPath,
	})
}

func (a *application) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	users := make([]map[string]any, 0, len(a.Config.Auth.Users))
	for username, user := range a.Config.Auth.Users {
		users = append(users, map[string]any{
			"username": username,
			"admin":    user.Admin,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users":        users,
		"requiresAuth": a.RequiresAuth,
		"hasSecretKey": a.Config.Auth.SecretKey != "",
	})
}

func (a *application) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Admin    bool   `json:"admin"`
	}
	if err := decodeJSONBody(r, &payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	username := strings.TrimSpace(payload.Username)
	if len(username) < 3 {
		writeJSONError(w, http.StatusBadRequest, "Username must be at least 3 characters")
		return
	}
	if len(payload.Password) < 6 {
		writeJSONError(w, http.StatusBadRequest, "Password must be at least 6 characters")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Could not hash password")
		return
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		auth := mapStringAny(raw["auth"])
		if auth == nil {
			auth = map[string]any{}
			raw["auth"] = auth
		}
		if asString(auth["secret-key"]) == "" {
			key, err := makeAuthSecretKey(AUTH_SECRET_KEY_LENGTH)
			if err != nil {
				return err
			}
			auth["secret-key"] = key
		}

		users := mapStringAny(auth["users"])
		if users == nil {
			users = map[string]any{}
			auth["users"] = users
		}
		if _, exists := users[username]; exists {
			return fmt.Errorf("user %s already exists", username)
		}

		user := map[string]any{"password-hash": string(hashed)}
		if payload.Admin {
			user["admin"] = true
		}
		users[username] = user
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	app := a.holder.current()
	if !a.RequiresAuth && app != nil && app.RequiresAuth {
		token, err := generateSessionToken(username, app.authSecretKey, time.Now())
		if err == nil {
			app.setAuthSessionCookie(w, r, token, time.Now().Add(AUTH_TOKEN_VALID_PERIOD))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *application) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	username := r.PathValue("username")
	var payload struct {
		Password *string `json:"password"`
		Admin    *bool   `json:"admin"`
	}
	if err := decodeJSONBody(r, &payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	var hashed []byte
	if payload.Password != nil {
		if len(*payload.Password) < 6 {
			writeJSONError(w, http.StatusBadRequest, "Password must be at least 6 characters")
			return
		}
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(*payload.Password), bcrypt.DefaultCost)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not hash password")
			return
		}
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		user := userMap(raw, username)
		if user == nil {
			return fmt.Errorf("user %s not found", username)
		}
		if hashed != nil {
			user["password-hash"] = string(hashed)
			delete(user, "password")
		}
		if payload.Admin != nil {
			if *payload.Admin {
				user["admin"] = true
			} else {
				delete(user, "admin")
			}
		}
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *application) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	username := r.PathValue("username")
	if err := a.holder.saveRaw(func(raw map[string]any) error {
		users := usersMap(raw)
		if users == nil {
			return fmt.Errorf("user %s not found", username)
		}
		if _, exists := users[username]; !exists {
			return fmt.Errorf("user %s not found", username)
		}
		if len(users) == 1 {
			return fmt.Errorf("cannot delete the last user")
		}
		delete(users, username)
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *application) handleAdminGetSettings(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	raw := a.rawConfig
	writeJSON(w, http.StatusOK, map[string]any{
		"server":   valueOrEmptyMap(raw["server"]),
		"branding": valueOrEmptyMap(raw["branding"]),
		"document": valueOrEmptyMap(raw["document"]),
	})
}

func (a *application) handleAdminSaveSettings(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	payload, err := readJSONRaw(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		if server := mapStringAny(payload["server"]); server != nil {
			raw["server"] = server
		}
		if branding := mapStringAny(payload["branding"]); branding != nil {
			raw["branding"] = branding
		}
		if document := mapStringAny(payload["document"]); document != nil {
			raw["document"] = document
		}
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *application) handleAdminGetTheme(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	rawTheme := valueOrEmptyMap(a.rawConfig["theme"])
	writeJSON(w, http.StatusOK, map[string]any{
		"theme":            rawTheme,
		"communityPresets": communityThemePresets(),
	})
}

func (a *application) handleAdminSaveTheme(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	payload, err := readJSONRaw(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	theme := mapStringAny(payload["theme"])
	if theme == nil {
		theme = payload
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		raw["theme"] = stripInternalKeys(theme)
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *application) handleAdminListPages(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	pages := make([]map[string]any, 0, len(rawPages(a.rawConfig)))
	for _, item := range rawPages(a.rawConfig) {
		page := mapStringAny(item)
		if page == nil {
			continue
		}
		pages = append(pages, map[string]any{
			"name":    asString(page["name"]),
			"slug":    rawPageSlug(page),
			"width":   asString(page["width"]),
			"columns": len(sliceAny(page["columns"])),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"pages": pages})
}

func (a *application) handleAdminCreatePage(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	payload, err := readJSONRaw(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	name := strings.TrimSpace(asString(payload["name"]))
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "Page name is required")
		return
	}

	page := defaultPageConfig(name)
	if slug := strings.TrimSpace(asString(payload["slug"])); slug != "" {
		page["slug"] = slug
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		slug := rawPageSlug(page)
		if slicesContainsString(reservedPageSlugs, slug) {
			return fmt.Errorf("page slug %q is reserved", slug)
		}
		if findRawPageIndex(raw, slug) >= 0 {
			return fmt.Errorf("page slug %q already exists", slug)
		}
		raw["pages"] = append(rawPages(raw), page)
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "slug": rawPageSlug(page)})
}

func (a *application) handleAdminUpdatePage(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	slug := r.PathValue("page")
	payload, err := readJSONRaw(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		index := findRawPageIndex(raw, slug)
		if index < 0 {
			return fmt.Errorf("page not found")
		}

		pages := rawPages(raw)
		page := mapStringAny(pages[index])
		if page == nil {
			return fmt.Errorf("page not found")
		}

		for _, key := range []string{"name", "slug", "width", "desktop-navigation-width"} {
			if value, exists := payload[key]; exists {
				if asString(value) == "" {
					delete(page, key)
				} else {
					page[key] = value
				}
			}
		}
		for _, key := range []string{"center-vertically", "hide-desktop-navigation", "show-mobile-header"} {
			if value, exists := payload[key]; exists {
				if asBool(value) {
					page[key] = true
				} else {
					delete(page, key)
				}
			}
		}

		nextSlug := rawPageSlug(page)
		if nextSlug != slug {
			if slicesContainsString(reservedPageSlugs, nextSlug) {
				return fmt.Errorf("page slug %q is reserved", nextSlug)
			}
			if findRawPageIndex(raw, nextSlug) >= 0 {
				return fmt.Errorf("page slug %q already exists", nextSlug)
			}
		}

		pages[index] = page
		raw["pages"] = pages
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *application) handleAdminDeletePage(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	slug := r.PathValue("page")
	if err := a.holder.saveRaw(func(raw map[string]any) error {
		pages := rawPages(raw)
		if len(pages) <= 1 {
			return fmt.Errorf("cannot delete the last page")
		}
		index := findRawPageIndex(raw, slug)
		if index < 0 {
			return fmt.Errorf("page not found")
		}
		raw["pages"] = append(pages[:index], pages[index+1:]...)
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *application) handleAdminReorderPages(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	var payload struct {
		Slugs []string `json:"slugs"`
	}
	if err := decodeJSONBody(r, &payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		current := rawPages(raw)
		if len(payload.Slugs) != len(current) {
			return fmt.Errorf("page list is out of date")
		}

		bySlug := make(map[string]any, len(current))
		for _, item := range current {
			page := mapStringAny(item)
			if page == nil {
				continue
			}
			bySlug[rawPageSlug(page)] = page
		}

		ordered := make([]any, 0, len(payload.Slugs))
		for _, slug := range payload.Slugs {
			page, exists := bySlug[slug]
			if !exists {
				return fmt.Errorf("unknown page %q", slug)
			}
			ordered = append(ordered, page)
		}
		raw["pages"] = ordered
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func usersMap(raw map[string]any) map[string]any {
	auth := mapStringAny(raw["auth"])
	if auth == nil {
		return nil
	}
	return mapStringAny(auth["users"])
}

func userMap(raw map[string]any, username string) map[string]any {
	users := usersMap(raw)
	if users == nil {
		return nil
	}
	return mapStringAny(users[username])
}

func valueOrEmptyMap(v any) map[string]any {
	if m := mapStringAny(v); m != nil {
		return m
	}
	return map[string]any{}
}

func communityThemePresets() []map[string]any {
	return []map[string]any{
		{"key": "teal-city", "label": "Teal City", "background-color": "225 14 15", "primary-color": "157 47 65", "contrast-multiplier": 1.1},
		{"key": "catppuccin-frappe", "label": "Catppuccin Frappe", "background-color": "229 19 23", "contrast-multiplier": 1.2, "primary-color": "222 74 74", "positive-color": "96 44 68", "negative-color": "359 68 71"},
		{"key": "catppuccin-macchiato", "label": "Catppuccin Macchiato", "background-color": "232 23 18", "contrast-multiplier": 1.2, "primary-color": "220 83 75", "positive-color": "105 48 72", "negative-color": "351 74 73"},
		{"key": "catppuccin-mocha", "label": "Catppuccin Mocha", "background-color": "240 21 15", "contrast-multiplier": 1.2, "primary-color": "217 92 83", "positive-color": "115 54 76", "negative-color": "347 70 65"},
		{"key": "camouflage", "label": "Camouflage", "background-color": "186 21 20", "contrast-multiplier": 1.2, "primary-color": "97 13 80"},
		{"key": "gruvbox-dark", "label": "Gruvbox Dark", "background-color": "0 0 16", "primary-color": "43 59 81", "positive-color": "61 66 44", "negative-color": "6 96 59"},
		{"key": "kanagawa-dark", "label": "Kanagawa Dark", "background-color": "240 13 14", "primary-color": "51 33 68", "negative-color": "358 100 68", "contrast-multiplier": 1.2},
		{"key": "tucan", "label": "Tucan", "background-color": "50 1 6", "primary-color": "24 97 58", "negative-color": "209 88 54"},
		{"key": "dracula", "label": "Dracula", "background-color": "231 15 21", "primary-color": "265 89 79", "contrast-multiplier": 1.2, "positive-color": "135 94 66", "negative-color": "0 100 67"},
		{"key": "shades-of-purple", "label": "Shades of Purple", "background-color": "243 33 25", "contrast-multiplier": 1.2, "primary-color": "50 100 49", "positive-color": "98 82 71", "negative-color": "12 77 52"},
		{"key": "neon-pink", "label": "Neon Pink", "background-color": "240 27 11", "contrast-multiplier": 1.5, "primary-color": "321 100 71", "positive-color": "165 78 51", "negative-color": "360 100 71"},
		{"key": "catppuccin-latte", "label": "Catppuccin Latte", "light": true, "background-color": "220 23 95", "contrast-multiplier": 1.0, "primary-color": "220 91 54", "positive-color": "109 58 40", "negative-color": "347 87 44"},
		{"key": "peachy", "label": "Peachy", "light": true, "background-color": "28 40 77", "primary-color": "155 100 20", "negative-color": "0 100 60", "contrast-multiplier": 1.1, "text-saturation-multiplier": 0.5},
		{"key": "zebra", "label": "Zebra", "light": true, "background-color": "0 0 95", "primary-color": "0 0 10", "negative-color": "0 90 50"},
	}
}
