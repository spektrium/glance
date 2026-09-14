import { elem, find, findAll } from "./templating.js";

const baseURL = pageData.baseURL || "";

async function api(path, options = {}) {
    const response = await fetch(`${baseURL}${path}`, {
        headers: { "Content-Type": "application/json", ...(options.headers || {}) },
        ...options,
        body: options.body ? JSON.stringify(options.body) : undefined,
    });

    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
        throw new Error(data.error || response.statusText);
    }
    return data;
}

function field(label, control, help = "") {
    return elem("div").classes("studio-field").append(
        elem("label").classes("studio-label").text(label),
        control,
        help ? elem("div").classes("studio-help").text(help) : "",
    );
}

function input(value = "", type = "text") {
    return elem("input").classes("studio-input").attrs({ type, value: value ?? "" });
}

function textarea(value = "") {
    const node = elem("textarea").classes("studio-textarea");
    node.value = value ?? "";
    return node;
}

function checkbox(checked) {
    const node = elem("input").attrs({ type: "checkbox" });
    node.checked = Boolean(checked);
    return node;
}

function hslInput(value = "") {
    const parts = String(value || "").trim().split(/\s+/);
    const wrap = elem("div").classes("studio-hsl");
    const swatch = elem("div").classes("studio-swatch");
    const h = input(parts[0] || "240");
    const s = input(parts[1] || "8");
    const l = input(parts[2] || "9");
    const paint = () => {
        wrap.dataset.value = `${h.value} ${s.value} ${l.value}`.trim();
        swatch.style.background = `hsl(${h.value} ${s.value}% ${l.value}%)`;
    };
    [h, s, l].forEach((node) => node.on("input", paint));
    paint();
    return wrap.append(swatch, h, s, l);
}

function button(label, extra = "studio-btn") {
    return elem("button").attrs({ type: "button" }).classes(...extra.split(" ")).text(label);
}

const state = { section: "users" };

function renderUsers(data) {
    const list = elem("div");
    const table = elem("table").classes("admin-table").append(
        elem("thead").append(elem("tr").append(
            elem("th").text("User"),
            elem("th").text("Role"),
            elem("th").text(""),
        )),
    );
    const body = elem("tbody");
    (data.users || []).forEach((user) => {
        const adminCheck = checkbox(user.admin);
        const password = input("", "password");
        password.placeholder = "New password";
        body.append(elem("tr").append(
            elem("td").text(user.username),
            elem("td").append(elem("label").classes("studio-check").append(adminCheck, elem("span").text("Admin"))),
            elem("td").append(
                elem("div").classes("studio-row").append(
                    password,
                    button("Save").on("click", async () => {
                        const payload = { admin: adminCheck.checked };
                        if (password.value) payload.password = password.value;
                        await api(`/api/admin/users/${encodeURIComponent(user.username)}`, { method: "PATCH", body: payload });
                        load();
                    }),
                    button("Delete", "studio-btn studio-btn-danger").on("click", async () => {
                        if (!confirm(`Delete ${user.username}?`)) return;
                        await api(`/api/admin/users/${encodeURIComponent(user.username)}`, { method: "DELETE" });
                        load();
                    }),
                ),
            ),
        ));
    });
    table.append(body);

    const username = input();
    const password = input("", "password");
    const admin = checkbox(true);

    list.append(
        elem("div").classes("admin-card").append(
            elem("div").classes("admin-title").text("Users"),
            elem("p").classes("admin-sub").text(data.requiresAuth
                ? "Passwords are stored as hashes. The first admin can manage the rest."
                : "Auth is off until you add a user. Glance will generate a secret key automatically."),
            table,
        ),
        elem("div").classes("admin-card").append(
            elem("div").classes("size-h3").text("Add user"),
            field("Username", username),
            field("Password", password, "At least 6 characters"),
            elem("label").classes("studio-check", "studio-field").append(admin, elem("span").text("Administrator")),
            button("Create user", "studio-btn studio-btn-primary").on("click", async () => {
                await api("/api/admin/users", {
                    method: "POST",
                    body: { username: username.value, password: password.value, admin: admin.checked },
                });
                username.value = "";
                password.value = "";
                load();
            }),
        ),
    );
    return list;
}

function renderSettings(data) {
    const server = data.server || {};
    const branding = data.branding || {};
    const documentHead = (data.document || {}).head || "";
    const host = input(server.host || "");
    const port = input(server.port || 8080, "number");
    const proxied = checkbox(server.proxied);
    const base = input(server["base-url"] || "");
    const assets = input(server["assets-path"] || "");
    const appName = input(branding["app-name"] || "");
    const logoText = input(branding["logo-text"] || "");
    const logoURL = input(branding["logo-url"] || "");
    const favicon = input(branding["favicon-url"] || "");
    const appIcon = input(branding["app-icon-url"] || "");
    const appBg = input(branding["app-background-color"] || "");
    const hideFooter = checkbox(branding["hide-footer"]);
    const footer = textarea(branding["custom-footer"] || "");
    const head = textarea(documentHead);

    return elem("div").append(
        elem("div").classes("admin-card").append(
            elem("div").classes("admin-title").text("Settings"),
            elem("p").classes("admin-sub").text("Host and port apply after a process restart. Everything else reloads immediately."),
            field("Host", host),
            field("Port", port),
            elem("label").classes("studio-check", "studio-field").append(proxied, elem("span").text("Behind a reverse proxy")),
            field("Base URL", base, "Only if Glance is served under a subdirectory"),
            field("Assets path", assets),
        ),
        elem("div").classes("admin-card").append(
            elem("div").classes("size-h3").text("Branding"),
            field("App name", appName),
            field("Logo text", logoText),
            field("Logo URL", logoURL),
            field("Favicon URL", favicon),
            field("App icon URL", appIcon),
            field("App background color", appBg),
            elem("label").classes("studio-check", "studio-field").append(hideFooter, elem("span").text("Hide footer")),
            field("Custom footer HTML", footer),
        ),
        elem("div").classes("admin-card").append(
            elem("div").classes("size-h3").text("Document"),
            field("Custom HTML in <head>", head),
            button("Save settings", "studio-btn studio-btn-primary").on("click", async () => {
                await api("/api/admin/settings", {
                    method: "PUT",
                    body: {
                        server: {
                            host: host.value,
                            port: Number(port.value) || 8080,
                            proxied: proxied.checked,
                            "base-url": base.value,
                            "assets-path": assets.value,
                        },
                        branding: {
                            "app-name": appName.value,
                            "logo-text": logoText.value,
                            "logo-url": logoURL.value,
                            "favicon-url": favicon.value,
                            "app-icon-url": appIcon.value,
                            "app-background-color": appBg.value,
                            "hide-footer": hideFooter.checked,
                            "custom-footer": footer.value,
                        },
                        document: { head: head.value },
                    },
                });
                load();
            }),
        ),
    );
}

function themeFromForm(nodes) {
    const theme = {
        light: nodes.light.checked,
        "background-color": nodes.background.dataset.value,
        "primary-color": nodes.primary.dataset.value,
        "positive-color": nodes.positive.dataset.value,
        "negative-color": nodes.negative.dataset.value,
        "contrast-multiplier": Number(nodes.contrast.value) || undefined,
        "text-saturation-multiplier": Number(nodes.saturation.value) || undefined,
        "custom-css-file": nodes.css.value,
        "disable-picker": nodes.disable.checked,
        presets: nodes.presets,
    };
    return theme;
}

function renderTheme(data) {
    const theme = data.theme || {};
    const presets = theme.presets || {};
    const nodes = {
        light: checkbox(theme.light),
        background: hslInput(theme["background-color"]),
        primary: hslInput(theme["primary-color"]),
        positive: hslInput(theme["positive-color"]),
        negative: hslInput(theme["negative-color"]),
        contrast: input(theme["contrast-multiplier"] ?? 1, "number"),
        saturation: input(theme["text-saturation-multiplier"] ?? 1, "number"),
        css: input(theme["custom-css-file"] || ""),
        disable: checkbox(theme["disable-picker"]),
        presets,
    };

    const presetWrap = elem("div").classes("studio-repeat");
    const renderPresets = () => {
        presetWrap.innerHTML = "";
        Object.entries(nodes.presets).forEach(([key, value]) => {
            const name = input(key);
            const item = elem("div").classes("studio-repeat-item").append(
                elem("div").classes("studio-repeat-item-bar").append(
                    elem("span").text(key),
                    button("Remove", "studio-btn studio-btn-ghost studio-btn-danger").on("click", () => {
                        delete nodes.presets[key];
                        renderPresets();
                    }),
                ),
                field("Key", name.on("change", () => {
                    const next = name.value.trim();
                    if (!next || next === key) return;
                    nodes.presets[next] = nodes.presets[key];
                    delete nodes.presets[key];
                    renderPresets();
                })),
            );
            presetWrap.append(item);
        });
    };
    renderPresets();

    const gallery = elem("div").classes("admin-theme-grid");
    (data.communityPresets || []).forEach((preset) => {
        const preview = elem("div").classes("admin-theme-preview");
        const [h, s, l] = String(preset["background-color"] || "240 8 9").split(" ");
        preview.style.background = `linear-gradient(90deg, hsl(${h} ${s}% ${l}%), hsl(${String(preset["primary-color"] || "43 50 70").replaceAll(" ", " ")}))`;
        gallery.append(elem("button").classes("studio-btn", "admin-theme-card").append(
            preview,
            elem("div").text(preset.label),
        ).on("click", () => {
            nodes.light.checked = Boolean(preset.light);
            ["background-color", "primary-color", "positive-color", "negative-color"].forEach((key) => {
                if (!preset[key]) return;
            });
            nodes.background.replaceWith(nodes.background = hslInput(preset["background-color"]));
            nodes.primary.replaceWith(nodes.primary = hslInput(preset["primary-color"]));
            nodes.positive.replaceWith(nodes.positive = hslInput(preset["positive-color"] || preset["primary-color"]));
            nodes.negative.replaceWith(nodes.negative = hslInput(preset["negative-color"]));
            nodes.contrast.value = preset["contrast-multiplier"] ?? 1;
            nodes.saturation.value = preset["text-saturation-multiplier"] ?? 1;
        }));
    });

    const save = button("Save theme", "studio-btn studio-btn-primary").on("click", async () => {
        await api("/api/admin/theme", { method: "PUT", body: { theme: themeFromForm(nodes) } });
        load();
    });

    return elem("div").append(
        elem("div").classes("admin-card").append(
            elem("div").classes("admin-title").text("Theme"),
            elem("p").classes("admin-sub").text("Default theme plus optional presets for the picker."),
            elem("label").classes("studio-check", "studio-field").append(nodes.light, elem("span").text("Light scheme")),
            field("Background", nodes.background),
            field("Primary", nodes.primary),
            field("Positive", nodes.positive),
            field("Negative", nodes.negative),
            field("Contrast multiplier", nodes.contrast),
            field("Text saturation multiplier", nodes.saturation),
            field("Custom CSS file", nodes.css),
            elem("label").classes("studio-check", "studio-field").append(nodes.disable, elem("span").text("Disable theme picker")),
            save,
        ),
        elem("div").classes("admin-card").append(
            elem("div").classes("size-h3").text("Community themes"),
            elem("p").classes("admin-sub").text("Click to copy values into the default theme, then save."),
            gallery,
        ),
        elem("div").classes("admin-card").append(
            elem("div").classes("size-h3").text("Presets"),
            presetWrap,
            button("Add preset").on("click", () => {
                const key = `preset-${Object.keys(nodes.presets).length + 1}`;
                nodes.presets[key] = {
                    "background-color": nodes.background.dataset.value,
                    "primary-color": nodes.primary.dataset.value,
                };
                renderPresets();
            }),
        ),
    );
}

function renderPages(data) {
    const wrap = elem("div").classes("admin-card");
    wrap.append(
        elem("div").classes("admin-title").text("Pages"),
        elem("p").classes("admin-sub").text("Create, rename and reorder pages. Widget layout is edited on the dashboard."),
    );
    const list = elem("div").classes("studio-repeat");
    (data.pages || []).forEach((page, index) => {
        list.append(elem("div").classes("studio-repeat-item").append(
            elem("div").classes("studio-repeat-item-bar").append(
                elem("span").text(`${index + 1}. ${page.name}`),
                elem("div").classes("studio-row").append(
                    button("Up", "studio-btn studio-btn-ghost").on("click", async () => {
                        if (index === 0) return;
                        const slugs = data.pages.map((item) => item.slug);
                        [slugs[index - 1], slugs[index]] = [slugs[index], slugs[index - 1]];
                        await api("/api/admin/pages/order", { method: "PUT", body: { slugs } });
                        load();
                    }),
                    button("Down", "studio-btn studio-btn-ghost").on("click", async () => {
                        if (index >= data.pages.length - 1) return;
                        const slugs = data.pages.map((item) => item.slug);
                        [slugs[index + 1], slugs[index]] = [slugs[index], slugs[index + 1]];
                        await api("/api/admin/pages/order", { method: "PUT", body: { slugs } });
                        load();
                    }),
                    button("Delete", "studio-btn studio-btn-ghost studio-btn-danger").on("click", async () => {
                        if (!confirm(`Delete page ${page.name}?`)) return;
                        await api(`/api/admin/pages/${encodeURIComponent(page.slug)}`, { method: "DELETE" });
                        load();
                    }),
                ),
            ),
            elem("a").attrs({ href: `${baseURL}/${page.slug}` }).text("Open and edit layout"),
        ));
    });

    const name = input("New page");
    wrap.append(
        list,
        field("New page name", name),
        button("Add page", "studio-btn studio-btn-primary").on("click", async () => {
            await api("/api/admin/pages", { method: "POST", body: { name: name.value } });
            load();
        }),
    );
    return wrap;
}

async function load() {
    const main = find("#admin-main");
    main.innerHTML = "";
    try {
        if (state.section === "users") main.append(renderUsers(await api("/api/admin/users")));
        if (state.section === "settings") main.append(renderSettings(await api("/api/admin/settings")));
        if (state.section === "theme") main.append(renderTheme(await api("/api/admin/theme")));
        if (state.section === "pages") main.append(renderPages(await api("/api/admin/pages")));
    } catch (error) {
        main.append(elem("div").classes("admin-card").text(error.message));
    }
}

findAll("#admin-nav button").forEach((buttonNode) => {
    buttonNode.addEventListener("click", () => {
        findAll("#admin-nav button").forEach((item) => item.classList.remove("is-active"));
        buttonNode.classList.add("is-active");
        state.section = buttonNode.dataset.section;
        load();
    });
});

load();
