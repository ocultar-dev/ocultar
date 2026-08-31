import { SiteNav } from "@/components/site/SiteNav";
import { SiteFooter } from "@/components/site/SiteFooter";
import { useEffect } from "react";

const LINK_CLASS = "text-primary hover:underline underline-offset-4";

const PARAGRAPHS = [
  <>
    I grew up in Monteverde, Costa Rica, a cloud forest community built around the idea that
    some things are worth protecting. I left for UT Austin to study Economics and Latin
    American Studies, then International Business at Grenoble École de Management, at the foot
    of the Alps.
  </>,
  <>
    I've spent fifteen years selling enterprise software, building Latin America into
    Bonitasoft's strongest region, and I'm now at{" "}
    <a href="https://vates.tech" target="_blank" rel="noreferrer" className={LINK_CLASS}>
      Vates
    </a>
    , the French open-source company behind XCP-ng, where we take on VMware. Nights and
    weekends, I build.
  </>,
  <>
    Ocultar started with a problem I had. I wanted the latest AI model to read my medical
    records, and I wasn't comfortable handing them over to do it. As I talked the idea through
    with friends, I kept seeing the same pattern: teams shipping raw customer data into
    third-party models, not because they wanted to, but because doing it safely was more
    friction than anyone had time for.
  </>,
  <>
    Ocultar is Spanish for to hide, to conceal. It's a zero-egress PII redaction proxy written
    in Go that sits between you and whatever model you're calling. Nothing raw ever leaves your
    machine: no name, no ID number, no medical record. Deterministic tokenization, an encrypted
    local vault, fail-closed by design. Fully open source under AGPLv3, because data
    sovereignty shouldn't come with a subscription.
  </>,
  <>
    <a href="https://getki.ai" target="_blank" rel="noreferrer" className={LINK_CLASS}>
      Ki!
    </a>{" "}
    is the same engine as a desktop app, for people who shouldn't need a proxy config to keep
    their own files private.
  </>,
  <>
    I work in English, Spanish and French. If you're building at the intersection of AI and
    privacy, or want to break something I've built, get in touch.
  </>,
];

const LINKS = [
  { label: "Email", href: "mailto:edu@ocultar.dev" },
  { label: "LinkedIn", href: "https://www.linkedin.com/in/eduardo-trejos/" },
  { label: "GitHub", href: "https://github.com/ocultar-dev" },
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
            Edu Trejos
          </h1>
          <p className="text-sm text-muted-foreground leading-relaxed mb-6">OCULTAR</p>
          <p className="text-[15px] font-medium text-foreground">Hola Mundo!</p>
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
