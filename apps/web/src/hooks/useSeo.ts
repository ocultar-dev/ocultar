import { useEffect } from "react";
import {
  DEFAULT_OG_IMAGE,
  SITE_NAME,
  buildJsonLdForRoute,
  getCanonicalUrl,
  getRouteMeta,
  type RouteSeoMeta,
} from "@/seo/jsonld.mjs";

interface UseSeoOptions {
  path: string;
  title?: string;
  description?: string;
  noindex?: boolean;
}

function upsertMeta(attr: "name" | "property", key: string, content: string) {
  let el = document.head.querySelector<HTMLMetaElement>(`meta[${attr}="${key}"]`);
  if (!el) {
    el = document.createElement("meta");
    el.setAttribute(attr, key);
    document.head.appendChild(el);
  }
  el.setAttribute("content", content);
}

function upsertCanonical(href: string) {
  let el = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!el) {
    el = document.createElement("link");
    el.setAttribute("rel", "canonical");
    document.head.appendChild(el);
  }
  el.setAttribute("href", href);
}

function replaceJsonLd(schemas: Record<string, unknown>[]) {
  document.head
    .querySelectorAll('script[type="application/ld+json"]')
    .forEach((node) => node.remove());

  for (const schema of schemas) {
    const script = document.createElement("script");
    script.type = "application/ld+json";
    script.textContent = JSON.stringify(schema);
    document.head.appendChild(script);
  }
}

export function useSeo({ path, title, description, noindex }: UseSeoOptions) {
  useEffect(() => {
    const registered = getRouteMeta(path);
    const meta: RouteSeoMeta = {
      path,
      title: title ?? registered?.title ?? "OCULTAR — Zero-Egress Local PII Redaction Firewall",
      description:
        description ??
        registered?.description ??
        "Open-source, local-first PII detection and redaction firewall written in Go.",
      ogType: registered?.ogType ?? "website",
      noindex: noindex ?? registered?.noindex,
      article: registered?.article,
      breadcrumbs: registered?.breadcrumbs,
      faq: registered?.faq,
      howto: registered?.howto,
    };

    const canonical = getCanonicalUrl(meta.path);

    document.title = meta.title;
    upsertMeta("name", "description", meta.description);
    upsertCanonical(canonical);

    upsertMeta("property", "og:site_name", SITE_NAME);
    upsertMeta("property", "og:type", meta.ogType);
    upsertMeta("property", "og:url", canonical);
    upsertMeta("property", "og:title", meta.title);
    upsertMeta("property", "og:description", meta.description);
    upsertMeta("property", "og:image", DEFAULT_OG_IMAGE);
    upsertMeta("property", "og:image:width", "1200");
    upsertMeta("property", "og:image:height", "630");

    upsertMeta("name", "twitter:card", "summary_large_image");
    upsertMeta("name", "twitter:title", meta.title);
    upsertMeta("name", "twitter:description", meta.description);
    upsertMeta("name", "twitter:image", DEFAULT_OG_IMAGE);

    const existingRobots = document.head.querySelector<HTMLMetaElement>('meta[name="robots"]');
    if (meta.noindex) {
      upsertMeta("name", "robots", "noindex, nofollow");
    } else if (existingRobots) {
      existingRobots.remove();
    }

    replaceJsonLd(buildJsonLdForRoute(meta));
  }, [path, title, description, noindex]);
}
