import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import {
  ROUTES,
  buildJsonLdForRoute,
  getCanonicalUrl,
  getRouteMeta,
} from "@/seo/jsonld.mjs";

describe("SEO route registry", () => {
  it("covers every concrete route registered in App.tsx", () => {
    const appSource = readFileSync(
      path.resolve(process.cwd(), "src/App.tsx"),
      "utf8"
    );
    const matches = [...appSource.matchAll(/<Route\s+path="([^"]+)"/g)].map(
      (m) => m[1]
    );
    const concretePaths = matches
      .filter((p) => p !== "*")
      .map((p) => (p.endsWith("/*") ? p.slice(0, -2) : p));

    for (const routePath of concretePaths) {
      expect(
        getRouteMeta(routePath),
        `Missing SEO route metadata for ${routePath}`
      ).not.toBeNull();
    }
  });

  it("ensures every route has a unique title, description, and canonical URL", () => {
    const titles = new Set<string>();
    const descriptions = new Set<string>();
    const canonicals = new Set<string>();

    for (const route of ROUTES) {
      expect(route.title.length).toBeGreaterThan(10);
      expect(route.description.length).toBeGreaterThan(30);
      expect(titles.has(route.title), `Duplicate title: ${route.title}`).toBe(false);
      expect(
        descriptions.has(route.description),
        `Duplicate description: ${route.description}`
      ).toBe(false);

      const canonical = getCanonicalUrl(route.path);
      expect(canonical.startsWith("https://ocultar.dev/")).toBe(true);
      expect(canonicals.has(canonical), `Duplicate canonical: ${canonical}`).toBe(false);

      titles.add(route.title);
      descriptions.add(route.description);
      canonicals.add(canonical);
    }
  });

  it("emits SoftwareApplication JSON-LD on every route plus page-specific schemas", () => {
    for (const route of ROUTES) {
      const schemas = buildJsonLdForRoute(route);
      const appSchema = schemas.find((s) => s["@type"] === "SoftwareApplication");
      expect(appSchema).toBeDefined();
      expect(appSchema?.codeRepository).toBe("https://github.com/ocultar-dev/ocultar");
    }

    const presidioSchemas = buildJsonLdForRoute(getRouteMeta("/vs-microsoft-presidio")!);
    expect(presidioSchemas.some((s) => s["@type"] === "TechArticle")).toBe(true);
    expect(presidioSchemas.some((s) => s["@type"] === "FAQPage")).toBe(true);

    const mcpSchemas = buildJsonLdForRoute(getRouteMeta("/guides/redact-pii-claude-mcp")!);
    expect(mcpSchemas.some((s) => s["@type"] === "TechArticle")).toBe(true);
    expect(mcpSchemas.some((s) => s["@type"] === "HowTo")).toBe(true);

    const gdprSchemas = buildJsonLdForRoute(
      getRouteMeta("/guides/local-pii-redaction-gdpr-eu-ai-act")!
    );
    expect(gdprSchemas.some((s) => s["@type"] === "TechArticle")).toBe(true);
  });
});
