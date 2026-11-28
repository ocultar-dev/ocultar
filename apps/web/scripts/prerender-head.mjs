import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  DEFAULT_OG_IMAGE,
  ROUTES,
  SITE_NAME,
  buildJsonLdForRoute,
  getCanonicalUrl,
} from "../src/seo/jsonld.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const DIST_DIR = path.resolve(__dirname, "../dist");
const TEMPLATE_PATH = path.join(DIST_DIR, "index.html");

const START_MARKER = "<!-- SEO_HEAD_START -->";
const END_MARKER = "<!-- SEO_HEAD_END -->";

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function renderHeadTags(route) {
  const canonical = getCanonicalUrl(route.path);
  const title = escapeHtml(route.title);
  const description = escapeHtml(route.description);
  const ogType = escapeHtml(route.ogType || "website");
  const ogImage = escapeHtml(DEFAULT_OG_IMAGE);
  const siteName = escapeHtml(SITE_NAME);
  const schemas = buildJsonLdForRoute(route);

  const lines = [
    `<title>${title}</title>`,
    `<meta name="description" content="${description}" />`,
    `<meta name="author" content="${siteName}" />`,
    `<link rel="canonical" href="${canonical}" />`,
    `<meta property="og:site_name" content="${siteName}" />`,
    `<meta property="og:type" content="${ogType}" />`,
    `<meta property="og:url" content="${canonical}" />`,
    `<meta property="og:title" content="${title}" />`,
    `<meta property="og:description" content="${description}" />`,
    `<meta property="og:image" content="${ogImage}" />`,
    `<meta property="og:image:width" content="1200" />`,
    `<meta property="og:image:height" content="630" />`,
    `<meta name="twitter:card" content="summary_large_image" />`,
    `<meta name="twitter:title" content="${title}" />`,
    `<meta name="twitter:description" content="${description}" />`,
    `<meta name="twitter:image" content="${ogImage}" />`,
  ];

  if (route.noindex) {
    lines.push(`<meta name="robots" content="noindex, nofollow" />`);
  }

  for (const schema of schemas) {
    const json = JSON.stringify(schema).replace(/</g, "\\u003c");
    lines.push(`<script type="application/ld+json">${json}</script>`);
  }

  return lines.map((line) => `    ${line}`).join("\n");
}

function injectHead(template, headHtml) {
  const startIdx = template.indexOf(START_MARKER);
  const endIdx = template.indexOf(END_MARKER);
  if (startIdx === -1 || endIdx === -1 || endIdx < startIdx) {
    throw new Error("SEO_HEAD_START / SEO_HEAD_END markers missing in dist/index.html");
  }
  return (
    template.slice(0, startIdx + START_MARKER.length) +
    "\n" +
    headHtml +
    "\n    " +
    template.slice(endIdx)
  );
}

function buildSitemapXml(routes) {
  const today = new Date().toISOString().slice(0, 10);
  const entries = routes
    .filter((r) => !r.noindex)
    .map((r) => {
      const loc = getCanonicalUrl(r.path);
      const lastmod = r.article?.dateModified || today;
      const priority = r.priority || "0.5";
      const changefreq = r.changefreq || "monthly";
      return [
        "  <url>",
        `    <loc>${escapeHtml(loc)}</loc>`,
        `    <lastmod>${lastmod}</lastmod>`,
        `    <changefreq>${changefreq}</changefreq>`,
        `    <priority>${priority}</priority>`,
        "  </url>",
      ].join("\n");
    })
    .join("\n");

  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    entries,
    "</urlset>",
    "",
  ].join("\n");
}

function main() {
  const template = readFileSync(TEMPLATE_PATH, "utf8");

  for (const route of ROUTES) {
    const html = injectHead(template, renderHeadTags(route));
    if (route.path === "/") {
      writeFileSync(TEMPLATE_PATH, html, "utf8");
      continue;
    }

    const relDir = route.path.replace(/^\/+/, "");
    const outDir = path.join(DIST_DIR, relDir);
    mkdirSync(outDir, { recursive: true });
    writeFileSync(path.join(outDir, "index.html"), html, "utf8");
  }

  const sitemapXml = buildSitemapXml(ROUTES);
  writeFileSync(path.join(DIST_DIR, "sitemap.xml"), sitemapXml, "utf8");

  console.log(`[prerender-head] wrote ${ROUTES.length} route heads + dist/sitemap.xml`);
}

main();
