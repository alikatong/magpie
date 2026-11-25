const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const partner = { id: "acme", name: "Acme AI", kind: "partner", added: true, chat: "https://api.acme.example/v1", regions: [{ id: "global", name: "Global", chat: "https://api.acme.example/v1" }, { id: "asia", name: "Asia", chat: "https://asia.acme.example/v1" }] };
const vendor = { id: "openai", name: "OpenAI", icon: "openai", kind: "vendor", chat: "https://api.openai.example/v1" };
const installed = { id: "acme-existing", name: "Acme Existing", preset: "acme", chat: partner.chat, models: [], agents: [], key: { set: true } };

function serve(lang) {
  return async (route) => {
    const { pathname } = new URL(route.request().url());
    if (pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = ${JSON.stringify({ lang, theme: "light", web: false })};` });
    if (pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (pathname === "/api/state") return route.fulfill({ json: { agents: [], profiles: [], settings: { lang, theme: "light" } } });
    if (pathname === "/api/providers") return route.fulfill({ json: { providers: [installed], presets: [partner, vendor], pluginPartners: [{ id: "notes", name: "Sponsor Only", plugin: "notes-sponsored" }], excluded: [], gateway: { running: true, window: true } } });
    if (pathname === "/api/plugins/listings") return route.fulfill({ json: { listings: [{ name: "Regular Plugin", package: "regular-plugin", kind: "agent", summary: { en: "Normal discovery result" }, npm: { version: "1.0.0" } }] } });
    if (pathname.startsWith("/api/")) return route.fulfill({ json: {} });
    const file = path.join(assets, pathname === "/" ? "index.html" : pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const headings = { en: "Partners", zh: "合作伙伴", "zh-TW": "合作夥伴", ja: "パートナー", de: "Partner" };

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  test(`${engine}: provider and plugin discovery skip promotions`, async (context) => {
    const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
    context.after(() => browser.close());
    for (const [lang, heading] of Object.entries(headings)) {
      await context.test(lang, async () => {
        const page = await browser.newPage({ viewport: { width: 440, height: 800 } });
        page.setDefaultTimeout(5000);
        const errors = [];
        const promotionRequests = [];
        page.on("pageerror", (error) => errors.push(error.message));
        page.on("request", (request) => {
          const pathname = new URL(request.url()).pathname;
          if (["/api/partner", "/api/page", "/pluginpartners.js"].includes(pathname)) promotionRequests.push(pathname);
        });
        try {
          await page.route("**/*", serve(lang));
          await page.goto("http://magpie.test/?view=providers");
          const existing = page.locator(`#providers [data-id="${installed.id}"] .name`);
          await existing.waitFor();
          assert.match(await existing.textContent(), /Acme Existing/);
          await existing.click();
          const savedEditor = page.locator(".editor");
          const regionButtons = savedEditor.locator(".regions > button");
          await regionButtons.last().waitFor();
          assert.equal(await regionButtons.count(), 2);
          await page.keyboard.press("Escape");
          await savedEditor.waitFor({ state: "detached" });
          const add = page.locator("#addProvider");
          assert.equal(await add.evaluate((button) => button.classList.contains("has-new")), false);
          await add.click();
          const sheet = page.locator("#addSheet");
          const openai = sheet.getByRole("button", { name: vendor.name, exact: true });
          await openai.waitFor();
          assert.equal(await sheet.getByText(heading, { exact: true }).count(), 0);
          assert.equal(await sheet.locator(".sponsor-me").count(), 0);
          assert.equal(await sheet.getByText(partner.name, { exact: true }).count(), 0);
          const search = sheet.locator(".find");
          await search.fill(partner.name);
          await sheet.locator(".none").waitFor();
          await search.fill(vendor.name);
          await openai.click();
          await page.locator("#modal .editor").waitFor();
          await page.goto("http://magpie.test/?view=plugins");
          const plugins = page.locator("#view-plugins");
          await plugins.locator('[data-pkg="regular-plugin"]').first().waitFor();
          assert.equal(await plugins.locator(".pm-partners, .sponsor-me").count(), 0);
          assert.equal(await plugins.getByText("Sponsor Only", { exact: true }).count(), 0);
          assert.deepEqual(promotionRequests, []);
          assert.deepEqual(errors, []);
        } finally {
          await page.close();
        }
      });
    }
  });
}
