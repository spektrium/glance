package glance

type fieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type fieldSchema struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        string        `json:"type"`
	Help        string        `json:"help,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Options     []fieldOption `json:"options,omitempty"`
	Fields      []fieldSchema `json:"fields,omitempty"`
	ItemType    string        `json:"itemType,omitempty"`
	Secret      bool          `json:"secret,omitempty"`
	Wide        bool          `json:"wide,omitempty"`
}

type widgetTypeSchema struct {
	Type        string         `json:"type"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Defaults    map[string]any `json:"defaults"`
	Fields      []fieldSchema  `json:"fields"`
}

func strField(key, label string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "string"}
}

func textField(key, label, help string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "text", Help: help, Wide: true}
}

func numField(key, label, help string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "number", Help: help}
}

func boolField(key, label string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "bool"}
}

func secretField(key, label, help string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "string", Help: help, Secret: true}
}

func selectField(key, label string, options ...string) fieldSchema {
	opts := make([]fieldOption, 0, len(options))
	for i := 0; i < len(options); i += 2 {
		opts = append(opts, fieldOption{Value: options[i], Label: options[i+1]})
	}
	return fieldSchema{Key: key, Label: label, Type: "select", Options: opts}
}

func stringListField(key, label, help string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "string-list", Help: help, ItemType: "string"}
}

func kvField(key, label, help string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "kv", Help: help}
}

func objectListField(key, label string, fields ...fieldSchema) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "object-list", Fields: fields, Wide: true}
}

func objectField(key, label string, fields ...fieldSchema) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "object", Fields: fields, Wide: true}
}

func namedMapField(key, label, nameLabel string, fields ...fieldSchema) fieldSchema {
	return fieldSchema{
		Key:      key,
		Label:    label,
		Type:     "named-map",
		Help:     nameLabel,
		Fields:   fields,
		Wide:     true,
		ItemType: "object",
	}
}

func durationFieldSchema(key, label string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "duration", Help: "Examples: 30s, 15m, 1h, 12h, 1d"}
}

func hslField(key, label string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "hsl", Help: "HSL as hue saturation lightness, e.g. 240 8 9"}
}

func widgetsField(key, label string) fieldSchema {
	return fieldSchema{Key: key, Label: label, Type: "widgets", Help: "Nested widgets can also be edited on the dashboard", Wide: true}
}

func sharedWidgetFields() []fieldSchema {
	return []fieldSchema{
		strField("title", "Title"),
		fieldSchema{Key: "title-url", Label: "Title URL", Type: "string", Placeholder: "https://"},
		boolField("hide-header", "Hide header"),
		durationFieldSchema("cache", "Cache"),
		strField("css-class", "CSS class"),
	}
}

func withShared(fields ...fieldSchema) []fieldSchema {
	return append(sharedWidgetFields(), fields...)
}

func hourFormatField() fieldSchema {
	return selectField("hour-format", "Hour format", "12h", "12 hour", "24h", "24 hour")
}

func targetField() fieldSchema {
	return selectField("target", "Link target", "_blank", "New tab", "_self", "Same tab", "_parent", "Parent", "_top", "Top")
}

func collapseAfterField() fieldSchema {
	return numField("collapse-after", "Collapse after", "Visible items before Show more. Use -1 to never collapse.")
}

func limitField(defHelp string) fieldSchema {
	return numField("limit", "Limit", defHelp)
}

func widgetTypeCatalog() []widgetTypeSchema {
	headers := kvField("headers", "Headers", "Request headers")
	basicAuth := objectField("basic-auth", "Basic auth",
		strField("username", "Username"),
		secretField("password", "Password", ""),
	)
	proxy := objectField("proxy", "Proxy",
		strField("url", "URL"),
		boolField("allow-insecure", "Allow insecure"),
		durationFieldSchema("timeout", "Timeout"),
	)

	return []widgetTypeSchema{
		{
			Type: "rss", Label: "RSS", Category: "Feeds",
			Description: "Articles from RSS and Atom feeds",
			Defaults:    map[string]any{"type": "rss", "limit": 10, "collapse-after": 3, "feeds": []any{map[string]any{"url": "https://"}}},
			Fields: withShared(
				limitField("Maximum number of articles"),
				collapseAfterField(),
				boolField("preserve-order", "Preserve feed order"),
				boolField("single-line-titles", "Single-line titles"),
				selectField("style", "Style",
					"vertical-list", "Vertical list",
					"detailed-list", "Detailed list",
					"horizontal-cards", "Horizontal cards",
					"horizontal-cards-2", "Horizontal cards 2",
				),
				numField("thumbnail-height", "Thumbnail height", "Used by horizontal-cards, in rem"),
				numField("card-height", "Card height", "Used by horizontal-cards-2, in rem"),
				objectListField("feeds", "Feeds",
					fieldSchema{Key: "url", Label: "URL", Type: "string", Placeholder: "https://"},
					strField("title", "Title"),
					boolField("hide-categories", "Hide categories"),
					boolField("hide-description", "Hide description"),
					numField("limit", "Limit", ""),
					strField("item-link-prefix", "Item link prefix"),
					strField("thumbnail-link-prefix", "Thumbnail link prefix"),
					headers,
				),
			),
		},
		{
			Type: "videos", Label: "Videos", Category: "Feeds",
			Description: "Latest videos from YouTube channels or playlists",
			Defaults:    map[string]any{"type": "videos", "channels": []any{}},
			Fields: withShared(
				stringListField("channels", "Channel IDs", "YouTube channel IDs"),
				stringListField("playlists", "Playlists", "YouTube playlist IDs"),
				limitField(""),
				selectField("style", "Style",
					"horizontal-cards", "Horizontal cards",
					"vertical-list", "Vertical list",
					"grid-cards", "Grid cards",
				),
				collapseAfterField(),
				numField("collapse-after-rows", "Collapse after rows", "For grid-cards"),
				strField("video-url-template", "Video URL template"),
				boolField("include-shorts", "Include shorts"),
			),
		},
		{
			Type: "hacker-news", Label: "Hacker News", Category: "Feeds",
			Description: "Posts from news.ycombinator.com",
			Defaults:    map[string]any{"type": "hacker-news"},
			Fields: withShared(
				limitField(""),
				collapseAfterField(),
				selectField("sort-by", "Sort by", "top", "Top", "new", "New", "best", "Best"),
				selectField("extra-sort-by", "Extra sort", "", "None", "engagement", "Engagement"),
				strField("comments-url-template", "Comments URL template"),
			),
		},
		{
			Type: "lobsters", Label: "Lobsters", Category: "Feeds",
			Description: "Posts from Lobsters or another instance",
			Defaults:    map[string]any{"type": "lobsters"},
			Fields: withShared(
				strField("instance-url", "Instance URL"),
				strField("custom-url", "Custom URL"),
				limitField(""),
				collapseAfterField(),
				selectField("sort-by", "Sort by", "hot", "Hot", "new", "New"),
				stringListField("tags", "Tags", ""),
			),
		},
		{
			Type: "reddit", Label: "Reddit", Category: "Feeds",
			Description: "Posts from a subreddit",
			Defaults:    map[string]any{"type": "reddit", "subreddit": "selfhosted", "show-thumbnails": true},
			Fields: withShared(
				strField("subreddit", "Subreddit"),
				selectField("style", "Style",
					"vertical-list", "Vertical list",
					"horizontal-cards", "Horizontal cards",
					"vertical-cards", "Vertical cards",
				),
				boolField("show-thumbnails", "Show thumbnails"),
				boolField("show-flairs", "Show flairs"),
				selectField("sort-by", "Sort by", "hot", "Hot", "new", "New", "top", "Top", "rising", "Rising"),
				selectField("top-period", "Top period", "hour", "Hour", "day", "Day", "week", "Week", "month", "Month", "year", "Year", "all", "All"),
				strField("search", "Search"),
				selectField("extra-sort-by", "Extra sort", "", "None", "engagement", "Engagement"),
				strField("comments-url-template", "Comments URL template"),
				limitField(""),
				collapseAfterField(),
				strField("request-url-template", "Request URL template"),
				proxy,
				objectField("app-auth", "App auth",
					strField("name", "Name"),
					strField("id", "ID"),
					secretField("secret", "Secret", ""),
				),
			),
		},
		{
			Type: "twitch-channels", Label: "Twitch channels", Category: "Feeds",
			Description: "Live status for Twitch channels",
			Defaults:    map[string]any{"type": "twitch-channels", "channels": []any{}},
			Fields: withShared(
				stringListField("channels", "Channels", "Channel logins"),
				collapseAfterField(),
				selectField("sort-by", "Sort by", "viewers", "Viewers", "live", "Live first"),
			),
		},
		{
			Type: "twitch-top-games", Label: "Twitch top games", Category: "Feeds",
			Description: "Currently popular Twitch categories",
			Defaults:    map[string]any{"type": "twitch-top-games"},
			Fields: withShared(
				stringListField("exclude", "Exclude", "Category names to hide"),
				limitField(""),
				collapseAfterField(),
			),
		},
		{
			Type: "releases", Label: "Releases", Category: "Feeds",
			Description: "GitHub, GitLab and Codeberg releases",
			Defaults:    map[string]any{"type": "releases", "repositories": []any{"glanceapp/glance"}},
			Fields: withShared(
				stringListField("repositories", "Repositories", "owner/name, or gitlab:group/project, codeberg:owner/name"),
				secretField("token", "GitHub token", ""),
				secretField("gitlab-token", "GitLab token", ""),
				limitField(""),
				collapseAfterField(),
				boolField("show-source-icon", "Show source icon"),
			),
		},
		{
			Type: "change-detection", Label: "ChangeDetection.io", Category: "Feeds",
			Description: "Website change notifications",
			Defaults:    map[string]any{"type": "change-detection"},
			Fields: withShared(
				stringListField("watches", "Watch UUIDs", ""),
				strField("instance-url", "Instance URL"),
				secretField("token", "Token", ""),
				limitField(""),
				collapseAfterField(),
			),
		},
		{
			Type: "repository", Label: "Repository", Category: "Feeds",
			Description: "GitHub repository overview",
			Defaults:    map[string]any{"type": "repository", "repository": "glanceapp/glance"},
			Fields: withShared(
				strField("repository", "Repository"),
				secretField("token", "Token", ""),
				numField("pull-requests-limit", "Pull requests limit", ""),
				numField("issues-limit", "Issues limit", ""),
				numField("commits-limit", "Commits limit", ""),
			),
		},
		{
			Type: "weather", Label: "Weather", Category: "Home",
			Description: "Forecast for a location",
			Defaults:    map[string]any{"type": "weather", "location": "London, United Kingdom", "units": "metric"},
			Fields: withShared(
				strField("location", "Location"),
				selectField("units", "Units", "metric", "Metric", "imperial", "Imperial"),
				hourFormatField(),
				boolField("hide-location", "Hide location"),
				boolField("show-area-name", "Show area name"),
			),
		},
		{
			Type: "calendar", Label: "Calendar", Category: "Home",
			Description: "Month calendar",
			Defaults:    map[string]any{"type": "calendar", "first-day-of-week": "monday"},
			Fields: withShared(
				selectField("first-day-of-week", "First day of week",
					"monday", "Monday", "tuesday", "Tuesday", "wednesday", "Wednesday",
					"thursday", "Thursday", "friday", "Friday", "saturday", "Saturday", "sunday", "Sunday",
				),
			),
		},
		{
			Type: "calendar-legacy", Label: "Calendar (legacy)", Category: "Home",
			Description: "Older calendar widget",
			Defaults:    map[string]any{"type": "calendar-legacy"},
			Fields: withShared(
				boolField("start-sunday", "Start on Sunday"),
			),
		},
		{
			Type: "clock", Label: "Clock", Category: "Home",
			Description: "Local time and extra timezones",
			Defaults:    map[string]any{"type": "clock", "hour-format": "24h"},
			Fields: withShared(
				hourFormatField(),
				objectListField("timezones", "Timezones",
					strField("timezone", "Timezone"),
					strField("label", "Label"),
				),
			),
		},
		{
			Type: "bookmarks", Label: "Bookmarks", Category: "Home",
			Description: "Grouped links",
			Defaults: map[string]any{"type": "bookmarks", "groups": []any{
				map[string]any{"title": "General", "links": []any{map[string]any{"title": "Glance", "url": "https://github.com/glanceapp/glance"}}},
			}},
			Fields: withShared(
				objectListField("groups", "Groups",
					strField("title", "Title"),
					hslField("color", "Color"),
					boolField("same-tab", "Same tab"),
					boolField("hide-arrow", "Hide arrow"),
					targetField(),
					objectListField("links", "Links",
						strField("title", "Title"),
						fieldSchema{Key: "url", Label: "URL", Type: "string", Placeholder: "https://"},
						strField("description", "Description"),
						strField("icon", "Icon"),
						boolField("same-tab", "Same tab"),
						boolField("hide-arrow", "Hide arrow"),
						targetField(),
					),
				),
			),
		},
		{
			Type: "search", Label: "Search", Category: "Home",
			Description: "Quick search with bangs",
			Defaults:    map[string]any{"type": "search", "search-engine": "duckduckgo"},
			Fields: withShared(
				strField("search-engine", "Search engine"),
				boolField("new-tab", "Open in new tab"),
				targetField(),
				boolField("autofocus", "Autofocus"),
				strField("placeholder", "Placeholder"),
				objectListField("bangs", "Bangs",
					strField("title", "Title"),
					strField("shortcut", "Shortcut"),
					strField("url", "URL"),
				),
			),
		},
		{
			Type: "to-do", Label: "To-do", Category: "Home",
			Description: "Local checklist stored in the browser",
			Defaults:    map[string]any{"type": "to-do"},
			Fields: withShared(
				strField("id", "List ID"),
			),
		},
		{
			Type: "markets", Label: "Markets", Category: "Home",
			Description: "Stock and crypto prices",
			Defaults: map[string]any{"type": "markets", "markets": []any{
				map[string]any{"symbol": "SPY", "name": "S&P 500"},
			}},
			Fields: withShared(
				objectListField("markets", "Markets",
					strField("symbol", "Symbol"),
					strField("name", "Name"),
					strField("chart-link", "Chart link"),
					strField("symbol-link", "Symbol link"),
				),
				objectListField("stocks", "Stocks (legacy)",
					strField("symbol", "Symbol"),
					strField("name", "Name"),
					strField("chart-link", "Chart link"),
					strField("symbol-link", "Symbol link"),
				),
				selectField("sort-by", "Sort by", "", "Configured order", "change", "Change", "absolute-change", "Absolute change"),
				strField("chart-link-template", "Chart link template"),
				strField("symbol-link-template", "Symbol link template"),
			),
		},
		{
			Type: "monitor", Label: "Monitor", Category: "Homelab",
			Description: "HTTP status of sites and services",
			Defaults: map[string]any{"type": "monitor", "sites": []any{
				map[string]any{"title": "Example", "url": "https://example.com"},
			}},
			Fields: withShared(
				selectField("style", "Style", "", "Default", "compact", "Compact"),
				boolField("show-failing-only", "Show failing only"),
				objectListField("sites", "Sites",
					strField("title", "Title"),
					fieldSchema{Key: "url", Label: "URL", Type: "string", Placeholder: "https://"},
					strField("check-url", "Check URL"),
					strField("error-url", "Error URL"),
					strField("icon", "Icon"),
					boolField("same-tab", "Same tab"),
					boolField("allow-insecure", "Allow insecure"),
					durationFieldSchema("timeout", "Timeout"),
					stringListField("alt-status-codes", "Alt status codes", "Treated as healthy"),
					basicAuth,
				),
			),
		},
		{
			Type: "docker-containers", Label: "Docker containers", Category: "Homelab",
			Description: "Status of local Docker containers",
			Defaults:    map[string]any{"type": "docker-containers"},
			Fields: withShared(
				boolField("hide-by-default", "Hide by default"),
				boolField("running-only", "Running only"),
				strField("category", "Category"),
				strField("sock-path", "Socket path"),
				boolField("format-container-names", "Format container names"),
				namedMapField("containers", "Container overrides", "Container name",
					strField("name", "Name"),
					strField("description", "Description"),
					strField("url", "URL"),
					strField("icon", "Icon"),
					strField("id", "ID"),
					strField("parent", "Parent"),
					boolField("hide", "Hide"),
				),
			),
		},
		{
			Type: "server-stats", Label: "Server stats", Category: "Homelab",
			Description: "CPU, memory and disk for local or remote hosts",
			Defaults: map[string]any{"type": "server-stats", "servers": []any{
				map[string]any{"type": "local"},
			}},
			Fields: withShared(
				objectListField("servers", "Servers",
					strField("name", "Name"),
					selectField("type", "Type", "local", "Local", "remote", "Remote"),
					strField("url", "URL"),
					secretField("token", "Token", ""),
					durationFieldSchema("timeout", "Timeout"),
					boolField("hide-swap", "Hide swap"),
					strField("cpu-temp-sensor", "CPU temp sensor"),
					boolField("hide-mountpoints-by-default", "Hide mountpoints by default"),
					namedMapField("mountpoints", "Mountpoints", "Path",
						strField("name", "Name"),
						boolField("hide", "Hide"),
					),
				),
			),
		},
		{
			Type: "dns-stats", Label: "DNS stats", Category: "Homelab",
			Description: "AdGuard, Pi-hole or Technitium stats",
			Defaults:    map[string]any{"type": "dns-stats", "service": "pihole"},
			Fields: withShared(
				selectField("service", "Service", "pihole", "Pi-hole v5", "pihole-v6", "Pi-hole v6", "adguard", "AdGuard Home", "technitium", "Technitium"),
				strField("url", "URL"),
				secretField("token", "Token", "Pi-hole v5 / Technitium"),
				strField("username", "Username"),
				secretField("password", "Password", "AdGuard or Pi-hole v6"),
				hourFormatField(),
				boolField("hide-graph", "Hide graph"),
				boolField("hide-top-domains", "Hide top domains"),
				boolField("allow-insecure", "Allow insecure"),
			),
		},
		{
			Type: "iframe", Label: "iframe", Category: "Custom",
			Description: "Embed another page",
			Defaults:    map[string]any{"type": "iframe", "source": "https://example.com", "height": 300},
			Fields: withShared(
				fieldSchema{Key: "source", Label: "Source URL", Type: "string", Placeholder: "https://"},
				numField("height", "Height", "Pixels"),
			),
		},
		{
			Type: "html", Label: "HTML", Category: "Custom",
			Description: "Raw HTML block",
			Defaults:    map[string]any{"type": "html", "source": "<p>Hello</p>"},
			Fields: withShared(
				textField("source", "HTML", ""),
			),
		},
		{
			Type: "custom-api", Label: "Custom API", Category: "Custom",
			Description: "Fetch JSON and render it with a template",
			Defaults: map[string]any{
				"type":     "custom-api",
				"url":      "https://",
				"template": "{{ .JSON }}",
			},
			Fields: withShared(
				strField("url", "URL"),
				boolField("allow-insecure", "Allow insecure"),
				selectField("method", "Method", "GET", "GET", "POST", "POST", "PUT", "PUT", "PATCH", "PATCH", "DELETE", "DELETE"),
				selectField("body-type", "Body type", "", "None", "json", "JSON", "string", "String"),
				textField("body", "Body", "JSON or string, depending on body type"),
				boolField("skip-json-validation", "Skip JSON validation"),
				headers,
				kvField("parameters", "Query parameters", ""),
				basicAuth,
				textField("template", "Template", "Go template"),
				boolField("frameless", "Frameless"),
				namedMapField("subrequests", "Subrequests", "Name",
					strField("url", "URL"),
					boolField("allow-insecure", "Allow insecure"),
					selectField("method", "Method", "GET", "GET", "POST", "POST", "PUT", "PUT", "PATCH", "PATCH", "DELETE", "DELETE"),
					selectField("body-type", "Body type", "", "None", "json", "JSON", "string", "String"),
					textField("body", "Body", ""),
					boolField("skip-json-validation", "Skip JSON validation"),
					headers,
					kvField("parameters", "Query parameters", ""),
					basicAuth,
				),
			),
		},
		{
			Type: "extension", Label: "Extension", Category: "Custom",
			Description: "Remote HTML extension widget",
			Defaults:    map[string]any{"type": "extension", "url": "https://"},
			Fields: withShared(
				strField("url", "URL"),
				strField("fallback-content-type", "Fallback content type"),
				kvField("parameters", "Parameters", ""),
				headers,
				boolField("allow-potentially-dangerous-html", "Allow HTML"),
			),
		},
		{
			Type: "group", Label: "Group", Category: "Layout",
			Description: "Tabbed group of widgets",
			Defaults: map[string]any{"type": "group", "widgets": []any{
				map[string]any{"type": "hacker-news"},
				map[string]any{"type": "lobsters"},
			}},
			Fields: withShared(
				widgetsField("widgets", "Widgets"),
			),
		},
		{
			Type: "split-column", Label: "Split column", Category: "Layout",
			Description: "Place widgets side by side inside a column",
			Defaults: map[string]any{"type": "split-column", "max-columns": 2, "widgets": []any{
				map[string]any{"type": "rss", "feeds": []any{map[string]any{"url": "https://"}}},
			}},
			Fields: withShared(
				numField("max-columns", "Max columns", "Minimum 2"),
				widgetsField("widgets", "Widgets"),
			),
		},
	}
}

func widgetSchemaByType() map[string]widgetTypeSchema {
	catalog := widgetTypeCatalog()
	out := make(map[string]widgetTypeSchema, len(catalog))
	for _, item := range catalog {
		out[item.Type] = item
	}
	return out
}

func defaultWidgetConfig(widgetType string) map[string]any {
	schema, ok := widgetSchemaByType()[widgetType]
	if !ok {
		return map[string]any{"type": widgetType}
	}

	cloned, err := cloneRawMap(schema.Defaults)
	if err != nil || cloned == nil {
		return map[string]any{"type": widgetType}
	}
	cloned["type"] = widgetType
	return cloned
}

func defaultPageConfig(name string) map[string]any {
	return map[string]any{
		"name": name,
		"columns": []any{
			map[string]any{"size": "small", "widgets": []any{}},
			map[string]any{"size": "full", "widgets": []any{}},
			map[string]any{"size": "small", "widgets": []any{}},
		},
	}
}

func pageFieldSchema() []fieldSchema {
	return []fieldSchema{
		strField("name", "Name"),
		strField("slug", "Slug"),
		selectField("width", "Width", "", "Default", "default", "Default", "slim", "Slim", "wide", "Wide"),
		selectField("desktop-navigation-width", "Desktop navigation width", "", "Same as page", "default", "Default", "slim", "Slim", "wide", "Wide"),
		boolField("center-vertically", "Center vertically"),
		boolField("hide-desktop-navigation", "Hide desktop navigation"),
		boolField("show-mobile-header", "Show mobile header"),
	}
}

func themeFieldSchema() []fieldSchema {
	return []fieldSchema{
		boolField("light", "Light scheme"),
		hslField("background-color", "Background"),
		hslField("primary-color", "Primary"),
		hslField("positive-color", "Positive"),
		hslField("negative-color", "Negative"),
		numField("contrast-multiplier", "Contrast multiplier", "1 is default, 1.3 is higher contrast"),
		numField("text-saturation-multiplier", "Text saturation multiplier", ""),
	}
}
