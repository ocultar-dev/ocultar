import { SiteNav } from "@/components/site/SiteNav";
import { SiteFooter } from "@/components/site/SiteFooter";
import { useEffect } from "react";

const PARAGRAPHS = [
  "I grew up in Monteverde, Costa Rica — a cloud forest town built around the idea that some things are worth keeping intact, undisturbed, exactly as they are. I left for UT Austin, where I studied Economics and Latin American Studies, and the path since has taken me a long way from that forest. But the instinct it left me with never really left: some things aren't meant to be exposed, extracted, or shipped off somewhere else just because it's convenient.",
  "Since 2010, I've been developing business for startups — the kind of work where you learn fast what founders actually need versus what they say they need. OCULTAR grew out of that same instinct pointed at a problem I kept running into: teams shipping raw customer data straight into third-party AI models because building it safely felt like too much friction.",
  "That's the whole premise of OCULTAR — Spanish for to hide, to conceal. It's a zero-egress PII redaction proxy: it sits between you and whatever AI model you're calling, and it makes sure nothing raw — no name, no SSN, no medical record — ever leaves your machine. Deterministic tokenization, an encrypted local vault, fail-closed by design. Fully open source, AGPLv3, because sovereignty over your own data shouldn't come with a subscription.",
];

const LINKS = [
  { label: "LinkedIn", href: "https://www.linkedin.com/in/eduardo-trejos/" },
  { label: "GitHub", href: "https://github.com/ocultar-dev" },
  { label: "Email", href: "mailto:edu@ocultar.dev" },
];

export default function About() {
  useEffect(() => {
    document.title = "About — OCULTAR";
  }, []);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteNav />

      <main className="container-page max-w-3xl py-24 md:py-32">
        {/* Header */}
        <div className="mb-16 border-b border-border pb-10">
          <p className="font-mono text-[11px] uppercase tracking-[0.2em] text-primary mb-4">
            About
          </p>
          <h1 className="text-4xl font-semibold tracking-tight text-foreground mb-4">
            Eduardo Trejos
          </h1>
          <p className="text-sm text-muted-foreground leading-relaxed">
            Founder, OCULTAR &nbsp;·&nbsp; Economics &amp; Latin American Studies, UT Austin
          </p>
        </div>

        {/* Bio */}
        <div className="flex flex-col gap-6">
          {PARAGRAPHS.map((p, i) => (
            <p key={i} className="text-[15px] leading-relaxed text-muted-foreground">
              {p}
            </p>
          ))}
        </div>

        {/* Links */}
        <div className="mt-14 pt-10 border-t border-border">
          <p className="font-mono text-[11px] uppercase tracking-[0.2em] text-primary mb-4">
            Get in touch
          </p>
          <div className="flex flex-wrap gap-x-8 gap-y-2">
            {LINKS.map((l) => (
              <a
                key={l.label}
                href={l.href}
                target={l.href.startsWith("http") ? "_blank" : undefined}
                rel={l.href.startsWith("http") ? "noreferrer" : undefined}
                className="text-[14px] text-primary hover:underline underline-offset-4"
              >
                {l.label}
              </a>
            ))}
          </div>
        </div>
      </main>

      <SiteFooter />
    </div>
  );
}
