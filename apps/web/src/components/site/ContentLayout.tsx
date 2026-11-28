import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { SiteNav } from "@/components/site/SiteNav";
import { SiteFooter } from "@/components/site/SiteFooter";

export interface ContentBreadcrumb {
  label: string;
  to?: string;
}

interface ContentLayoutProps {
  eyebrow: string;
  title: string;
  lead: string;
  meta?: string;
  breadcrumbs: ContentBreadcrumb[];
  children: ReactNode;
}

export function ContentLayout({
  eyebrow,
  title,
  lead,
  meta,
  breadcrumbs,
  children,
}: ContentLayoutProps) {
  return (
    <div id="top" className="min-h-screen bg-background text-foreground flex flex-col">
      <SiteNav />

      <main className="flex-1 pt-24 pb-20 md:pt-32 md:pb-28">
        <div className="container-page max-w-4xl">
          <nav
            aria-label="Breadcrumb"
            className="mb-8 flex flex-wrap items-center gap-1.5 font-mono text-[11px] text-muted-foreground"
          >
            {breadcrumbs.map((crumb, idx) => (
              <span key={crumb.label} className="inline-flex items-center gap-1.5">
                {idx > 0 && <ChevronRight className="h-3 w-3 text-border-strong" />}
                {crumb.to ? (
                  <Link
                    to={crumb.to}
                    className="transition-colors hover:text-foreground"
                  >
                    {crumb.label}
                  </Link>
                ) : (
                  <span className="text-foreground">{crumb.label}</span>
                )}
              </span>
            ))}
          </nav>

          <header className="mb-12 border-b border-border pb-10">
            <p className="font-mono text-[11px] uppercase tracking-[0.2em] text-primary mb-4">
              {eyebrow}
            </p>
            <h1 className="text-3xl sm:text-4xl md:text-[42px] font-semibold leading-[1.1] tracking-tight text-foreground text-balance">
              {title}
            </h1>
            <p className="mt-5 text-[16px] sm:text-[17px] leading-relaxed text-muted-foreground max-w-3xl">
              {lead}
            </p>
            {meta && (
              <p className="mt-5 font-mono text-[11px] text-muted-foreground">
                {meta}
              </p>
            )}
          </header>

          <article className="space-y-12 text-[15px] leading-relaxed text-muted-foreground">
            {children}
          </article>
        </div>
      </main>

      <SiteFooter />
    </div>
  );
}
