package glance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

type appHolder struct {
	configPath string

	mu  sync.RWMutex
	app *application
	mux http.Handler

	skipWatch atomic.Bool
}

func (h *appHolder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	mux := h.mux
	h.mu.RUnlock()

	if mux == nil {
		http.Error(w, "Glance is starting", http.StatusServiceUnavailable)
		return
	}

	mux.ServeHTTP(w, r)
}

func (h *appHolder) current() *application {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.app
}

func (h *appHolder) swap(app *application, mux http.Handler) {
	h.mu.Lock()
	h.app = app
	h.mux = mux
	h.mu.Unlock()
}

func (h *appHolder) consumeSkip() bool {
	return h.skipWatch.CompareAndSwap(true, false)
}

func (h *appHolder) loadFromYAML(contents []byte) (*application, error) {
	config, raw, err := loadConfigFromYAML(contents)
	if err != nil {
		return nil, err
	}

	app, err := newApplication(config)
	if err != nil {
		return nil, err
	}

	app.holder = h
	app.configPath = h.configPath
	app.rawConfig = raw
	return app, nil
}

func (h *appHolder) saveRaw(mutate func(map[string]any) error) error {
	current := h.current()
	if current == nil {
		return fmt.Errorf("application is not ready")
	}

	raw, err := cloneRawMap(current.rawConfig)
	if err != nil {
		return fmt.Errorf("cloning config: %w", err)
	}

	if raw == nil {
		raw = map[string]any{}
	}

	if err := mutate(raw); err != nil {
		return err
	}

	raw = omitEmptyDeep(raw).(map[string]any)
	yamlBytes, err := encodeConfigYAML(raw)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if _, _, err := loadConfigFromYAML(yamlBytes); err != nil {
		return err
	}

	app, err := h.loadFromYAML(yamlBytes)
	if err != nil {
		return err
	}

	h.skipWatch.Store(true)
	if err := os.WriteFile(h.configPath, yamlBytes, 0644); err != nil {
		h.skipWatch.Store(false)
		return fmt.Errorf("writing config file: %w", err)
	}

	h.swap(app, app.routes())
	log.Println("Configuration saved")
	return nil
}

func cloneRawMap(in map[string]any) (map[string]any, error) {
	if in == nil {
		return map[string]any{}, nil
	}

	encoded, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}

	var out map[string]any
	if err := yaml.Unmarshal(encoded, &out); err != nil {
		return nil, err
	}

	if out == nil {
		out = map[string]any{}
	}

	return out, nil
}

type persistDoc struct {
	Server   any `yaml:"server,omitempty"`
	Auth     any `yaml:"auth,omitempty"`
	Document any `yaml:"document,omitempty"`
	Theme    any `yaml:"theme,omitempty"`
	Branding any `yaml:"branding,omitempty"`
	Pages    any `yaml:"pages"`
}

func encodeConfigYAML(raw map[string]any) ([]byte, error) {
	doc := persistDoc{
		Server:   raw["server"],
		Auth:     raw["auth"],
		Document: raw["document"],
		Theme:    raw["theme"],
		Branding: raw["branding"],
		Pages:    raw["pages"],
	}

	var buf bytes.Buffer
	buf.WriteString("# Managed by Glance admin and visual editor.\n")

	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func omitEmptyDeep(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for key, item := range val {
			if strings.HasPrefix(key, "_") {
				continue
			}
			cleaned := omitEmptyDeep(item)
			if isOmitEmptyValue(cleaned) {
				continue
			}
			out[key] = cleaned
		}
		return out
	case []any:
		out := make([]any, 0, len(val))
		for _, item := range val {
			out = append(out, omitEmptyDeep(item))
		}
		return out
	case float64:
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	default:
		return v
	}
}

func isOmitEmptyValue(v any) bool {
	if v == nil {
		return true
	}

	if s, ok := v.(string); ok {
		return s == ""
	}

	if m, ok := v.(map[string]any); ok {
		return len(m) == 0
	}

	return false
}

func mapStringAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func sliceAny(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch n := v.(type) {
	case string:
		return n
	case fmt.Stringer:
		return n.String()
	default:
		return fmt.Sprintf("%v", n)
	}
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func rawPages(raw map[string]any) []any {
	pages := sliceAny(raw["pages"])
	if pages == nil {
		return []any{}
	}
	return pages
}

func rawPageSlug(page map[string]any) string {
	if slug := strings.TrimSpace(asString(page["slug"])); slug != "" {
		return slug
	}

	return titleToSlug(asString(page["name"]))
}

func findRawPageIndex(raw map[string]any, slug string) int {
	for i, item := range rawPages(raw) {
		page := mapStringAny(item)
		if page == nil {
			continue
		}
		if rawPageSlug(page) == slug {
			return i
		}
	}
	return -1
}

func stripInternalKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for key, item := range val {
			if strings.HasPrefix(key, "_") {
				continue
			}
			out[key] = stripInternalKeys(item)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = stripInternalKeys(item)
		}
		return out
	default:
		return v
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSONBody(r *http.Request, dest any) error {
	defer r.Body.Close()
	limited := http.MaxBytesReader(nil, r.Body, 2*1024*1024)
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return nil
}

func readJSONRaw(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	limited := http.MaxBytesReader(nil, r.Body, 2*1024*1024)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}
