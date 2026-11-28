import { ROUTES_CONFIG } from "./routes.mjs";

export const SITE_URL = ROUTES_CONFIG.siteUrl;
export const SITE_NAME = ROUTES_CONFIG.siteName;
export const DEFAULT_OG_IMAGE = ROUTES_CONFIG.defaultOgImage;
export const ROUTES = ROUTES_CONFIG.routes;

export function normalizePath(pathname) {
  if (!pathname || pathname === "/") return "/";
  return pathname.replace(/\/+$/, "");
}

export function getCanonicalUrl(pathname) {
  const normalized = normalizePath(pathname);
  return normalized === "/" ? `${SITE_URL}/` : `${SITE_URL}${normalized}`;
}

export function getRouteMeta(pathname) {
  const normalized = normalizePath(pathname);
  const direct = ROUTES.find((r) => r.path === normalized);
  if (direct) return direct;

  // Fallback for unknown /docs/* paths
  if (normalized.startsWith("/docs/")) {
    const slug = normalized.replace("/docs/", "").split("/").pop() || "Documentation";
    const label = slug.replace(/[_-]+/g, " ");
    return {
      path: normalized,
      title: `${label} — OCULTAR Docs`,
      description: `OCULTAR technical documentation for ${label}.`,
      ogType: "article",
    };
  }

  return null;
}

export function buildSoftwareApplicationJsonLd() {
  return {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "OCULTAR",
    alternateName: "OCULTAR PII Refinery",
    applicationCategory: "SecurityApplication",
    applicationSubCategory: "PII Detection and Redaction Firewall",
    operatingSystem: "Linux, macOS, Windows",
    softwareVersion: "1.16.0",
    url: `${SITE_URL}/`,
    downloadUrl: "https://github.com/ocultar-dev/ocultar/releases/latest",
    codeRepository: "https://github.com/ocultar-dev/ocultar",
    license: "https://www.gnu.org/licenses/agpl-3.0.en.html",
    programmingLanguage: ["Go", "Python"],
    description:
      "Zero-egress, local-first PII detection and redaction firewall written in Go with deterministic tokenization, AES-256-GCM local vault, and MCP connectors for Claude, Mistral, and Goose.",
    offers: {
      "@type": "Offer",
      price: "0",
      priceCurrency: "USD",
    },
    author: {
      "@type": "Person",
      name: "Edu Trejos",
      url: `${SITE_URL}/about`,
    },
  };
}

export function buildJsonLdForRoute(route) {
  const schemas = [buildSoftwareApplicationJsonLd()];
  const canonical = getCanonicalUrl(route.path);

  if (route.breadcrumbs && route.breadcrumbs.length > 0) {
    schemas.push({
      "@context": "https://schema.org",
      "@type": "BreadcrumbList",
      itemListElement: route.breadcrumbs.map((crumb, idx) => ({
        "@type": "ListItem",
        position: idx + 1,
        name: crumb.name,
        item: getCanonicalUrl(crumb.path),
      })),
    });
  }

  if (route.article) {
    schemas.push({
      "@context": "https://schema.org",
      "@type": "TechArticle",
      headline: route.article.headline,
      description: route.description,
      url: canonical,
      datePublished: route.article.datePublished,
      dateModified: route.article.dateModified,
      articleSection: route.article.section,
      image: DEFAULT_OG_IMAGE,
      author: {
        "@type": "Organization",
        name: SITE_NAME,
        url: `${SITE_URL}/`,
      },
      publisher: {
        "@type": "Organization",
        name: SITE_NAME,
        url: `${SITE_URL}/`,
      },
      mainEntityOfPage: {
        "@type": "WebPage",
        "@id": canonical,
      },
    });
  }

  if (route.faq && route.faq.length > 0) {
    schemas.push({
      "@context": "https://schema.org",
      "@type": "FAQPage",
      mainEntity: route.faq.map((item) => ({
        "@type": "Question",
        name: item.question,
        acceptedAnswer: {
          "@type": "Answer",
          text: item.answer,
        },
      })),
    });
  }

  if (route.howto) {
    schemas.push({
      "@context": "https://schema.org",
      "@type": "HowTo",
      name: route.howto.name,
      description: route.howto.description,
      step: route.howto.steps.map((step, idx) => ({
        "@type": "HowToStep",
        position: idx + 1,
        name: step.name,
        text: step.text,
      })),
    });
  }

  return schemas;
}
