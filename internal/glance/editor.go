package glance

import (
	"fmt"
	"net/http"
)

func (a *application) registerEditorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/editor/schema", a.handleEditorSchema)
	mux.HandleFunc("GET /api/editor/page/{page}", a.handleEditorGetPage)
	mux.HandleFunc("PUT /api/editor/page/{page}", a.handleEditorSavePage)
}

func (a *application) handleEditorSchema(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"widgets": widgetTypeCatalog(),
		"page":    pageFieldSchema(),
		"theme":   themeFieldSchema(),
	})
}

func (a *application) handleEditorGetPage(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	slug := r.PathValue("page")
	page, exists := a.slugToPage[slug]
	if !exists {
		writeJSONError(w, http.StatusNotFound, "Page not found")
		return
	}

	rawPage, err := a.rawPageCopy(slug)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	annotatePageWidgetIDs(page, rawPage)

	writeJSON(w, http.StatusOK, map[string]any{
		"slug": slug,
		"page": rawPage,
	})
}

func (a *application) handleEditorSavePage(w http.ResponseWriter, r *http.Request) {
	if a.handleForbiddenResponse(w, r, showUnauthorizedJSON) {
		return
	}

	slug := r.PathValue("page")
	payload, err := readJSONRaw(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	pagePayload := mapStringAny(payload["page"])
	if pagePayload == nil {
		pagePayload = payload
	}
	pagePayload = stripInternalKeys(pagePayload).(map[string]any)

	if asString(pagePayload["name"]) == "" {
		writeJSONError(w, http.StatusBadRequest, "Page name is required")
		return
	}

	if err := a.holder.saveRaw(func(raw map[string]any) error {
		pages := rawPages(raw)
		index := findRawPageIndex(raw, slug)
		if index < 0 {
			return fmt.Errorf("page not found")
		}

		nextSlug := rawPageSlug(pagePayload)
		if nextSlug != slug {
			if slicesContainsString(reservedPageSlugs, nextSlug) {
				return fmt.Errorf("page slug %q is reserved", nextSlug)
			}
			if findRawPageIndex(raw, nextSlug) >= 0 {
				return fmt.Errorf("page slug %q already exists", nextSlug)
			}
		}

		pages[index] = pagePayload
		raw["pages"] = pages
		return nil
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	app := a.holder.current()
	nextSlug := rawPageSlug(pagePayload)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"slug": nextSlug,
		"page": func() map[string]any {
			copied, err := app.rawPageCopy(nextSlug)
			if err != nil {
				return pagePayload
			}
			if page, exists := app.slugToPage[nextSlug]; exists {
				annotatePageWidgetIDs(page, copied)
			}
			return copied
		}(),
	})
}

func (a *application) rawPageCopy(slug string) (map[string]any, error) {
	index := findRawPageIndex(a.rawConfig, slug)
	if index < 0 {
		return nil, fmt.Errorf("page not found")
	}

	cloned, err := cloneRawMap(mapStringAny(rawPages(a.rawConfig)[index]))
	if err != nil {
		return nil, err
	}
	return cloned, nil
}

func annotatePageWidgetIDs(page *page, raw map[string]any) {
	if page == nil || raw == nil {
		return
	}

	annotateWidgets(page.HeadWidgets, sliceAny(raw["head-widgets"]))
	rawColumns := sliceAny(raw["columns"])
	for i := range page.Columns {
		if i >= len(rawColumns) {
			break
		}
		column := mapStringAny(rawColumns[i])
		if column == nil {
			continue
		}
		annotateWidgets(page.Columns[i].Widgets, sliceAny(column["widgets"]))
	}
}

func annotateWidgets(runtime widgets, raw []any) {
	limit := min(len(runtime), len(raw))
	for i := 0; i < limit; i++ {
		item := mapStringAny(raw[i])
		if item == nil {
			continue
		}

		item["_id"] = runtime[i].GetID()
		item["type"] = runtime[i].GetType()

		children := widgetChildren(runtime[i])
		if len(children) == 0 {
			continue
		}

		nested := sliceAny(item["widgets"])
		if nested != nil {
			annotateWidgets(children, nested)
		}
	}
}

func widgetChildren(w widget) widgets {
	switch typed := w.(type) {
	case *groupWidget:
		return typed.Widgets
	case *splitColumnWidget:
		return typed.Widgets
	default:
		return nil
	}
}

func slicesContainsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
