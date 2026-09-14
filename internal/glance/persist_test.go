package glance

import (
	"os"
	"testing"
)

func TestLoadExampleConfigAndEncode(t *testing.T) {
	contents, err := os.ReadFile("../../docs/glance.yml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}

	config, raw, err := loadConfigFromYAML(contents)
	if err != nil {
		t.Fatalf("load example config: %v", err)
	}
	if len(config.Pages) == 0 {
		t.Fatal("expected pages in example config")
	}

	encoded, err := encodeConfigYAML(raw)
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}

	reloaded, _, err := loadConfigFromYAML(encoded)
	if err != nil {
		t.Fatalf("reload encoded config: %v\n%s", err, encoded)
	}
	if len(reloaded.Pages) != len(config.Pages) {
		t.Fatalf("page count: got %d want %d", len(reloaded.Pages), len(config.Pages))
	}
	if reloaded.Pages[0].Title != config.Pages[0].Title {
		t.Fatalf("page title: got %q want %q", reloaded.Pages[0].Title, config.Pages[0].Title)
	}
}

func TestWidgetSchemaCoversRegisteredTypes(t *testing.T) {
	catalog := widgetSchemaByType()
	for _, widgetType := range []string{
		"calendar", "calendar-legacy", "clock", "weather", "bookmarks", "iframe", "html",
		"hacker-news", "releases", "videos", "markets", "reddit", "rss", "monitor",
		"twitch-top-games", "twitch-channels", "lobsters", "change-detection", "repository",
		"search", "extension", "group", "dns-stats", "split-column", "custom-api",
		"docker-containers", "server-stats", "to-do",
	} {
		if _, err := newWidget(widgetType); err != nil {
			t.Fatalf("newWidget(%s): %v", widgetType, err)
		}
		if _, ok := catalog[widgetType]; !ok {
			t.Fatalf("missing schema for widget type %s", widgetType)
		}
	}
}

func TestDefaultWidgetsInitialize(t *testing.T) {
	for _, schema := range widgetTypeCatalog() {
		page := defaultPageConfig("Schema")
		columns := sliceAny(page["columns"])
		full := mapStringAny(columns[1])
		full["widgets"] = []any{defaultWidgetConfig(schema.Type)}
		raw := map[string]any{"pages": []any{page}}
		encoded, err := encodeConfigYAML(raw)
		if err != nil {
			t.Fatalf("%s encode: %v", schema.Type, err)
		}
		if _, _, err := loadConfigFromYAML(encoded); err != nil {
			t.Fatalf("%s default widget is invalid: %v\n%s", schema.Type, err, encoded)
		}
	}
}

func TestRawPageSlug(t *testing.T) {
	if got := rawPageSlug(map[string]any{"name": "Home"}); got != "home" {
		t.Fatalf("slug from name: %q", got)
	}
	if got := rawPageSlug(map[string]any{"name": "Home", "slug": "start"}); got != "start" {
		t.Fatalf("explicit slug: %q", got)
	}
}
