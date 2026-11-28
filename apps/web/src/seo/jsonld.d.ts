export interface BreadcrumbItem {
  name: string;
  path: string;
}

export interface FaqItem {
  question: string;
  answer: string;
}

export interface HowToStep {
  name: string;
  text: string;
}

export interface RouteSeoMeta {
  path: string;
  title: string;
  description: string;
  ogType: "website" | "article";
  priority?: string;
  changefreq?: string;
  noindex?: boolean;
  article?: {
    headline: string;
    datePublished: string;
    dateModified: string;
    section: string;
  };
  breadcrumbs?: BreadcrumbItem[];
  faq?: FaqItem[];
  howto?: {
    name: string;
    description: string;
    steps: HowToStep[];
  };
}

export declare const SITE_URL: string;
export declare const SITE_NAME: string;
export declare const DEFAULT_OG_IMAGE: string;
export declare const ROUTES: RouteSeoMeta[];

export declare function normalizePath(pathname: string): string;
export declare function getCanonicalUrl(pathname: string): string;
export declare function getRouteMeta(pathname: string): RouteSeoMeta | null;
export declare function buildSoftwareApplicationJsonLd(): Record<string, unknown>;
export declare function buildJsonLdForRoute(route: RouteSeoMeta): Record<string, unknown>[];
