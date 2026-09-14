import { elem, find, findAll } from "./templating.js";

const EDITING_KEY = "glance-editing";

function api(path, options = {}) {
    return fetch(`${pageData.baseURL}${path}`, {
        headers: { "Content-Type": "application/json", ...(options.headers || {}) },
        ...options,
        body: options.body ? JSON.stringify(options.body) : undefined,
    }).then(async (response) => {
        const data = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(data.error || response.statusText);
        return data;
    });
}

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function getAt(root, path) {
    if (!path) return root;
    return path.split(".").reduce((acc, key) => (acc == null ? acc : acc[key]), root);
}

function parentPath(path) {
    const parts = path.split(".");
    parts.pop();
    return parts.join(".");
}

function lastKey(path) {
    return path.split(".").pop();
}

function widgetPathById(page, id, prefix = "") {
    const visitList = (list, listPath) => {
        if (!Array.isArray(list)) return null;
        for (let i = 0; i < list.length; i++) {
            const item = list[i];
            const itemPath = `${listPath}.${i}`;
            if (item && String(item._id) === String(id)) return itemPath;
            const nested = widgetPathById(item, id, itemPath);
            if (nested) return nested;
        }
        return null;
    };

    if (prefix) {
        return visitList(page?.widgets, `${prefix}.widgets`);
    }

    const head = visitList(page["head-widgets"], "head-widgets");
    if (head) return head;

    const columns = page.columns || [];
    for (let i = 0; i < columns.length; i++) {
        const found = visitList(columns[i].widgets, `columns.${i}.widgets`);
        if (found) return found;
    }
    return null;
}

function ensureList(page, path) {
    const parent = getAt(page, parentPath(path));
    const key = lastKey(path);
    if (!parent[key]) parent[key] = [];
    return parent[key];
}

function moveItem(list, from, to) {
    if (from === to || from < 0 || to < 0 || from >= list.length) return;
    const [item] = list.splice(from, 1);
    list.splice(Math.min(to, list.length), 0, item);
}

function iconButton(title, path) {
    return elem("button").attrs({ type: "button", title }).classes("editor-icon-btn").html(`
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="${path}" />
        </svg>
    `);
}

function fieldLabel(label) {
    return elem("label").classes("studio-label").text(label);
}

function inputControl(value, type = "text") {
    const node = elem("input").classes("studio-input").attrs({ type });
    node.value = value == null ? "" : value;
    return node;
}

function selectControl(value, options) {
    const node = elem("select").classes("studio-select");
    (options || []).forEach((option) => {
        const item = elem("option").attrs({ value: option.value }).text(option.label);
        if (String(option.value) === String(value ?? "")) item.selected = true;
        node.append(item);
    });
    return node;
}

function hslControl(value) {
    const parts = String(value || "").trim().split(/\s+/);
    const wrap = elem("div").classes("studio-hsl");
    const swatch = elem("div").classes("studio-swatch");
    const controls = [inputControl(parts[0] || ""), inputControl(parts[1] || ""), inputControl(parts[2] || "")];
    const sync = () => {
        wrap.dataset.value = controls.map((item) => item.value).join(" ").trim();
        if (controls[0].value) {
            swatch.style.background = `hsl(${controls[0].value} ${controls[1].value || 0}% ${controls[2].value || 0}%)`;
        }
    };
    controls.forEach((item) => item.on("input", sync));
    sync();
    wrap.getValue = () => wrap.dataset.value;
    return wrap.append(swatch, ...controls);
}

function kvControl(value) {
    const entries = Object.entries(value || {}).map(([key, item]) => ({ key, value: Array.isArray(item) ? item.join(", ") : item }));
    const wrap = elem("div").classes("studio-repeat");
    const draw = () => {
        wrap.innerHTML = "";
        entries.forEach((entry, index) => {
            wrap.append(elem("div").classes("studio-row", "studio-field").append(
                inputControl(entry.key).on("input", (event) => { entries[index].key = event.target.value; }),
                inputControl(entry.value).on("input", (event) => { entries[index].value = event.target.value; }),
                elem("button").classes("studio-btn", "studio-btn-ghost").text("×").on("click", () => {
                    entries.splice(index, 1);
                    draw();
                }),
            ));
        });
        wrap.append(elem("button").classes("studio-btn").text("Add pair").on("click", () => {
            entries.push({ key: "", value: "" });
            draw();
        }));
    };
    wrap.getValue = () => {
        const out = {};
        entries.forEach((entry) => {
            if (entry.key) out[entry.key] = entry.value;
        });
        return out;
    };
    draw();
    return wrap;
}

function stringListControl(value) {
    const items = Array.isArray(value) ? value.map(String) : [];
    const wrap = elem("div").classes("studio-repeat");
    const draw = () => {
        wrap.innerHTML = "";
        items.forEach((item, index) => {
            wrap.append(elem("div").classes("studio-row", "studio-field").append(
                inputControl(item).on("input", (event) => { items[index] = event.target.value; }),
                elem("button").classes("studio-btn", "studio-btn-ghost").text("×").on("click", () => {
                    items.splice(index, 1);
                    draw();
                }),
            ));
        });
        wrap.append(elem("button").classes("studio-btn").text("Add").on("click", () => {
            items.push("");
            draw();
        }));
    };
    wrap.getValue = () => items.filter((item) => item !== "");
    draw();
    return wrap;
}

function objectListControl(value, fields, schemaMap) {
    const items = Array.isArray(value) ? clone(value) : [];
    const wrap = elem("div").classes("studio-repeat");
    const getters = [];
    const draw = () => {
        wrap.innerHTML = "";
        getters.length = 0;
        items.forEach((item, index) => {
            const body = elem("div");
            const controls = renderFields(item || {}, fields, schemaMap);
            getters[index] = controls.getValue;
            body.append(controls.node);
            wrap.append(elem("div").classes("studio-repeat-item").append(
                elem("div").classes("studio-repeat-item-bar").append(
                    elem("span").text(`Item ${index + 1}`),
                    elem("button").classes("studio-btn", "studio-btn-ghost", "studio-btn-danger").text("Remove").on("click", () => {
                        items.splice(index, 1);
                        draw();
                    }),
                ),
                body,
            ));
        });
        wrap.append(elem("button").classes("studio-btn").text("Add item").on("click", () => {
            items.push({});
            draw();
        }));
    };
    wrap.getValue = () => getters.map((getter, index) => getter ? getter() : items[index]);
    draw();
    return wrap;
}

function namedMapControl(value, fields, schemaMap) {
    const entries = Object.entries(value || {}).map(([key, item]) => ({ key, value: item || {} }));
    const wrap = elem("div").classes("studio-repeat");
    const draw = () => {
        wrap.innerHTML = "";
        entries.forEach((entry, index) => {
            const controls = renderFields(entry.value, fields, schemaMap);
            wrap.append(elem("div").classes("studio-repeat-item").append(
                elem("div").classes("studio-repeat-item-bar").append(
                    inputControl(entry.key).on("input", (event) => { entries[index].key = event.target.value; }),
                    elem("button").classes("studio-btn", "studio-btn-ghost", "studio-btn-danger").text("Remove").on("click", () => {
                        entries.splice(index, 1);
                        draw();
                    }),
                ),
                controls.node,
            ));
            entries[index].getValue = controls.getValue;
        });
        wrap.append(elem("button").classes("studio-btn").text("Add").on("click", () => {
            entries.push({ key: "", value: {} });
            draw();
        }));
    };
    wrap.getValue = () => {
        const out = {};
        entries.forEach((entry) => {
            if (entry.key) out[entry.key] = entry.getValue ? entry.getValue() : entry.value;
        });
        return out;
    };
    draw();
    return wrap;
}

function widgetsControl(value, schemaMap) {
    const items = Array.isArray(value) ? clone(value) : [];
    const wrap = elem("div").classes("studio-repeat");
    const draw = () => {
        wrap.innerHTML = "";
        items.forEach((item, index) => {
            wrap.append(elem("div").classes("studio-repeat-item").append(
                elem("div").classes("studio-repeat-item-bar").append(
                    elem("span").text(item.title || item.type || "Widget"),
                    elem("button").classes("studio-btn", "studio-btn-ghost", "studio-btn-danger").text("Remove").on("click", () => {
                        items.splice(index, 1);
                        draw();
                    }),
                ),
                elem("p").classes("studio-help").text("Use the dashboard overlay to edit nested widget settings."),
            ));
        });
    };
    wrap.getValue = () => items;
    draw();
    return wrap;
}

function renderFields(values, fields, schemaMap) {
    const node = elem("div");
    const getters = [];

    (fields || []).forEach((field) => {
        const current = values ? values[field.key] : undefined;
        let control;
        if (field.type === "bool") {
            control = elem("label").classes("studio-check").append(
                elem("input").attrs({ type: "checkbox" }).tap((input) => { input.checked = Boolean(current); }),
                elem("span").text("Enabled"),
            );
            getters.push(() => ({ [field.key]: control.querySelector("input").checked }));
        } else if (field.type === "select") {
            control = selectControl(current, field.options);
            getters.push(() => ({ [field.key]: control.value }));
        } else if (field.type === "number") {
            control = inputControl(current ?? "", "number");
            getters.push(() => ({ [field.key]: control.value === "" ? undefined : Number(control.value) }));
        } else if (field.type === "text") {
            control = elem("textarea").classes("studio-textarea");
            control.value = current ?? "";
            getters.push(() => ({ [field.key]: control.value }));
        } else if (field.type === "duration") {
            control = inputControl(current ?? "");
            control.placeholder = "15m";
            getters.push(() => ({ [field.key]: control.value }));
        } else if (field.type === "hsl") {
            control = hslControl(current);
            getters.push(() => ({ [field.key]: control.getValue() }));
        } else if (field.type === "kv") {
            control = kvControl(current);
            getters.push(() => ({ [field.key]: control.getValue() }));
        } else if (field.type === "string-list") {
            control = stringListControl(current);
            getters.push(() => ({ [field.key]: control.getValue() }));
        } else if (field.type === "object-list") {
            control = objectListControl(current, field.fields, schemaMap);
            getters.push(() => ({ [field.key]: control.getValue() }));
        } else if (field.type === "object") {
            const nested = renderFields(current || {}, field.fields, schemaMap);
            control = nested.node;
            getters.push(() => ({ [field.key]: nested.getValue() }));
        } else if (field.type === "named-map") {
            control = namedMapControl(current, field.fields, schemaMap);
            getters.push(() => ({ [field.key]: control.getValue() }));
        } else if (field.type === "widgets") {
            control = widgetsControl(current, schemaMap);
            getters.push(() => ({ [field.key]: control.getValue() }));
        } else {
            control = inputControl(current ?? "", field.secret ? "password" : "text");
            if (field.placeholder) control.placeholder = field.placeholder;
            getters.push(() => ({ [field.key]: control.value }));
        }

        const block = elem("div").classes("studio-field").append(fieldLabel(field.label), control);
        if (field.help) block.append(elem("div").classes("studio-help").text(field.help));
        node.append(block);
    });

    return {
        node,
        getValue() {
            const out = { ...(values || {}) };
            getters.forEach((getter) => Object.assign(out, getter()));
            Object.keys(out).forEach((key) => {
                if (out[key] === "" || out[key] === undefined) delete out[key];
            });
            return out;
        },
    };
}

function schemaFor(type, schema) {
    return (schema.widgets || []).find((item) => item.type === type);
}

function cleanPage(page) {
    const walk = (value) => {
        if (Array.isArray(value)) return value.map(walk);
        if (value && typeof value === "object") {
            const out = {};
            Object.entries(value).forEach(([key, item]) => {
                if (key.startsWith("_")) return;
                out[key] = walk(item);
            });
            return out;
        }
        return value;
    };
    return walk(page);
}

export function setupEditor() {
    if (!pageData.canManage) return;

    const schemaPromise = api("/api/editor/schema");
    let schema = { widgets: [], page: [] };
    let page = null;
    let saving = false;

    const bar = elem("div").classes("studio-bar").hide();
    const status = elem("div").classes("editor-status");
    const drawer = elem("aside").classes("studio-drawer");
    const backdrop = elem("div").classes("studio-backdrop").hide();
    document.body.append(bar, backdrop, drawer);

    const closeDrawer = () => {
        drawer.classList.remove("is-open");
        backdrop.hide();
        drawer.innerHTML = "";
    };

    backdrop.on("click", closeDrawer);

    async function loadPage() {
        const payload = await api(`/api/editor/page/${pageData.slug || ""}`);
        page = payload.page;
        decorate();
    }

    async function save(nextPage = page) {
        saving = true;
        status.text("Saving");
        try {
            const payload = await api(`/api/editor/page/${pageData.slug || ""}`, {
                method: "PUT",
                body: { page: cleanPage(nextPage) },
            });
            page = payload.page;
            if (payload.slug !== undefined && payload.slug !== pageData.slug) {
                window.location.href = `${pageData.baseURL}/${payload.slug}`;
                return;
            }
            sessionStorage.setItem(EDITING_KEY, "1");
            window.location.reload();
        } catch (error) {
            status.text(error.message);
            alert(error.message);
        } finally {
            saving = false;
        }
    }

    function openDrawer(title, fieldsNode, onSave) {
        drawer.innerHTML = "";
        const header = elem("div").classes("studio-drawer-header").append(
            elem("div").classes("studio-bar-title").text(title),
            elem("button").classes("studio-btn", "studio-btn-ghost").text("Close").on("click", closeDrawer),
        );
        drawer.append(header, elem("div").classes("studio-drawer-body").append(fieldsNode));
        if (onSave) {
            drawer.append(elem("div").classes("studio-drawer-footer").append(
                elem("button").classes("studio-btn", "studio-btn-primary").text("Save").on("click", async () => {
                    await onSave();
                    closeDrawer();
                }),
            ));
        }
        backdrop.show();
        drawer.classList.add("is-open");
    }

    function openWidgetPicker(listPath) {
        const groups = {};
        schema.widgets.forEach((item) => {
            groups[item.category] ||= [];
            groups[item.category].push(item);
        });

        const body = elem("div");
        Object.entries(groups).forEach(([category, items]) => {
            body.append(elem("div").classes("studio-label").text(category));
            const grid = elem("div").classes("editor-picker");
            items.forEach((item) => {
                grid.append(elem("button").classes("studio-btn", "editor-picker-item").append(
                    elem("strong").text(item.label),
                    elem("small").text(item.description),
                ).on("click", async () => {
                    const list = ensureList(page, listPath);
                    list.push(clone(item.defaults || { type: item.type }));
                    closeDrawer();
                    await save(page);
                }));
            });
            body.append(grid);
        });
        openDrawer("Add widget", body);
    }

    function openWidgetEditor(path) {
        const widget = getAt(page, path);
        if (!widget) return;
        const typeSchema = schemaFor(widget.type, schema);
        const form = renderFields(widget, typeSchema ? typeSchema.fields : [], schema);
        openDrawer(typeSchema ? typeSchema.label : widget.type, form.node, async () => {
            const next = form.getValue();
            next.type = widget.type;
            if (widget.widgets) next.widgets = widget.widgets;
            const parent = getAt(page, parentPath(path));
            parent[lastKey(path)] = next;
            await save(page);
        });
    }

    function openPageSettings() {
        const form = renderFields(page, schema.page, schema);
        openDrawer("Page", form.node, async () => {
            Object.assign(page, form.getValue());
            await save(page);
        });
    }

    function decorate() {
        if (!document.body.classList.contains("editing") || !page) return;

        findAll(".editor-widget-toolbar, .editor-zone-toolbar").forEach((node) => node.remove());

        findAll(".widget[data-widget-id]").forEach((widgetEl) => {
            const path = widgetPathById(page, widgetEl.dataset.widgetId);
            if (!path) return;
            if (widgetEl.classList.contains("widget-type-group") || widgetEl.classList.contains("widget-type-split-column")) {
                const nestedZone = widgetEl.querySelector(".widget-group-contents, .masonry") || widgetEl;
                nestedZone.dataset.editorZone = `${path}.widgets`;
            }
        });

        findAll("[data-editor-zone]").forEach((zone) => {
            const path = zone.dataset.editorZone;
            const toolbar = elem("div").classes("editor-zone-toolbar");
            const isColumn = path.startsWith("columns.") && path.endsWith(".widgets");
            if (isColumn) {
                const columnIndex = path.split(".")[1];
                const size = selectControl(page.columns[columnIndex].size, [
                    { value: "small", label: "Small" },
                    { value: "full", label: "Full" },
                ]).on("change", async (event) => {
                    page.columns[columnIndex].size = event.target.value;
                    await save(page);
                });
                const tools = elem("div").classes("studio-row").append(elem("span").classes("studio-label").text("Column"), size);
                if ((page.columns || []).length > 1) {
                    tools.append(elem("button").classes("studio-btn", "studio-btn-ghost", "studio-btn-danger").text("Remove").on("click", async () => {
                        if (!confirm("Remove this column and its widgets?")) return;
                        page.columns.splice(Number(columnIndex), 1);
                        await save(page);
                    }));
                }
                toolbar.append(tools);
            } else {
                toolbar.append(elem("span").classes("studio-label").text(path === "head-widgets" ? "Head widgets" : "Widgets"));
            }
            toolbar.append(elem("button").classes("studio-btn").text("Add widget").on("click", () => openWidgetPicker(path)));
            zone.prepend(toolbar);
            if (zone.dataset.editorBound === "1") return;
            zone.dataset.editorBound = "1";
            zone.addEventListener("dragover", (event) => {
                event.preventDefault();
                zone.classList.add("editor-drop-target");
            });
            zone.addEventListener("dragleave", () => zone.classList.remove("editor-drop-target"));
            zone.addEventListener("drop", async (event) => {
                event.preventDefault();
                zone.classList.remove("editor-drop-target");
                const from = event.dataTransfer.getData("text/glance-widget") || event.dataTransfer.getData("text/plain");
                if (!from || saving) return;
                const widget = getAt(page, from);
                const fromList = getAt(page, parentPath(from));
                fromList.splice(Number(lastKey(from)), 1);
                const toList = ensureList(page, path);
                toList.push(widget);
                await save(page);
            });
        });

        findAll(".widget[data-widget-id]").forEach((widgetEl) => {
            const path = widgetPathById(page, widgetEl.dataset.widgetId);
            if (!path) return;
            widgetEl.dataset.editorPath = path;
            const toolbar = elem("div").classes("editor-widget-toolbar");
            const handle = iconButton("Drag", "M3.75 9h16.5m-16.5 6.75h16.5");
            handle.attr("draggable", "true");
            handle.on("dragstart", (event) => {
                event.dataTransfer.setData("text/plain", path);
                event.dataTransfer.setData("text/glance-widget", path);
                event.dataTransfer.effectAllowed = "move";
            });
            toolbar.append(
                handle,
                iconButton("Settings", "m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125").on("click", () => openWidgetEditor(path)),
                iconButton("Delete", "M6 18 18 6M6 6l12 12").on("click", async () => {
                    if (!confirm("Remove this widget?")) return;
                    const list = getAt(page, parentPath(path));
                    list.splice(Number(lastKey(path)), 1);
                    await save(page);
                }),
            );
            widgetEl.append(toolbar);
        });
    }

    function setEditing(enabled) {
        document.body.classList.toggle("editing", enabled);
        bar.showIf(enabled);
        findAll(".js-edit-dashboard").forEach((button) => button.classList.toggle("is-active", enabled));
        if (enabled) {
            sessionStorage.setItem(EDITING_KEY, "1");
            decorate();
        } else {
            sessionStorage.removeItem(EDITING_KEY);
            findAll(".editor-widget-toolbar, .editor-zone-toolbar").forEach((node) => node.remove());
            closeDrawer();
        }
    }

    bar.append(
        elem("div").classes("studio-bar-title").text("Editing"),
        status,
        elem("button").classes("studio-btn").text("Page settings").on("click", openPageSettings),
        elem("button").classes("studio-btn").text("Add column").on("click", async () => {
            if ((page.columns || []).length >= 3) {
                alert("A page can have at most 3 columns");
                return;
            }
            page.columns = page.columns || [];
            page.columns.push({ size: page.columns.some((column) => column.size === "full") ? "small" : "full", widgets: [] });
            await save(page);
        }),
        elem("button").classes("studio-btn").text("Head widgets").on("click", () => {
            if (!page["head-widgets"]) page["head-widgets"] = [];
            openWidgetPicker("head-widgets");
        }),
        elem("button").classes("studio-btn", "studio-btn-primary").text("Done").on("click", () => setEditing(false)),
    );

    findAll(".js-edit-dashboard").forEach((button) => {
        button.addEventListener("click", async () => {
            if (!schema.widgets.length) schema = await schemaPromise;
            if (!page) await loadPage();
            setEditing(!document.body.classList.contains("editing"));
        });
    });

    schemaPromise.then((value) => { schema = value; });

    if (sessionStorage.getItem(EDITING_KEY) === "1") {
        schemaPromise.then(async (value) => {
            schema = value;
            await loadPage();
            setEditing(true);
        });
    }
}
