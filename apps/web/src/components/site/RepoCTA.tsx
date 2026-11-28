import { useState } from "react";
import { ArrowUpRight, Check, Copy, Github, Star, Terminal } from "lucide-react";
import { useGithubStars } from "@/hooks/useGithubStars";

interface RepoCTAProps {
  heading?: string;
  description?: string;
  command?: string;
  secondaryCommand?: {
    label: string;
    command: string;
  };
}

const DEFAULT_COMMAND = `docker run --rm -p 4141:4141 \\
  -e OCU_MASTER_KEY=$(openssl rand -hex 32) \\
  -e OCU_SALT=$(openssl rand -hex 16) \\
  -e OCU_AUDITOR_TOKEN=$(openssl rand -hex 24) \\
  ghcr.io/ocultar-dev/ocultar:latest -serve 4141`;

export function RepoCTA({
  heading = "Run OCULTAR locally",
  description = "AGPLv3 open source. Single Go binary or Docker container, zero telemetry, no account required.",
  command = DEFAULT_COMMAND,
  secondaryCommand,
}: RepoCTAProps) {
  const stars = useGithubStars();
  const [copiedPrimary, setCopiedPrimary] = useState(false);
  const [copiedSecondary, setCopiedSecondary] = useState(false);

  const copyText = async (text: string, setFn: (v: boolean) => void) => {
    try {
      await navigator.clipboard.writeText(text);
      setFn(true);
      setTimeout(() => setFn(false), 1800);
    } catch {
      // Clipboard API unavailable
    }
  };

  return (
    <aside className="mt-16 rounded-xl border border-border bg-surface/80 p-6 sm:p-8">
      <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
        <div className="max-w-xl">
          <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-primary mb-2">
            Open Source · Self-Hosted
          </p>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">
            {heading}
          </h2>
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
            {description}
          </p>
        </div>

        <a
          href="https://github.com/ocultar-dev/ocultar"
          target="_blank"
          rel="noreferrer"
          className="inline-flex h-10 shrink-0 items-center gap-2 rounded-md border border-border-strong bg-surface-elevated px-4 text-sm font-medium text-foreground transition-colors hover:border-primary/50 hover:text-primary"
        >
          <Github className="h-4 w-4" />
          <span>ocultar-dev/ocultar</span>
          {stars !== null && (
            <span className="inline-flex items-center gap-1 rounded-full bg-surface px-2 py-0.5 font-mono text-[11px] text-muted-foreground">
              <Star className="h-3 w-3 fill-current" />
              {stars}
            </span>
          )}
          <ArrowUpRight className="h-3.5 w-3.5 text-muted-foreground" />
        </a>
      </div>

      <div className="mt-6 space-y-3">
        <div className="rounded-lg border border-border bg-background/80">
          <div className="flex items-center justify-between border-b border-border px-3.5 py-2">
            <span className="inline-flex items-center gap-1.5 font-mono text-[11px] text-muted-foreground">
              <Terminal className="h-3.5 w-3.5 text-primary" />
              Quickstart (Docker)
            </span>
            <button
              type="button"
              onClick={() => copyText(command, setCopiedPrimary)}
              className="inline-flex items-center gap-1 font-mono text-[11px] text-muted-foreground transition-colors hover:text-foreground"
            >
              {copiedPrimary ? (
                <>
                  <Check className="h-3.5 w-3.5 text-success" />
                  <span className="text-success">Copied</span>
                </>
              ) : (
                <>
                  <Copy className="h-3.5 w-3.5" />
                  <span>Copy</span>
                </>
              )}
            </button>
          </div>
          <pre className="overflow-x-auto p-3.5 font-mono text-[12.5px] leading-relaxed text-foreground">
            <code>{command}</code>
          </pre>
        </div>

        {secondaryCommand && (
          <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-background/80 px-3.5 py-2.5">
            <div className="min-w-0">
              <span className="mr-2 font-mono text-[11px] text-muted-foreground">
                {secondaryCommand.label}:
              </span>
              <code className="font-mono text-[12.5px] text-primary">
                {secondaryCommand.command}
              </code>
            </div>
            <button
              type="button"
              onClick={() => copyText(secondaryCommand.command, setCopiedSecondary)}
              className="inline-flex shrink-0 items-center gap-1 font-mono text-[11px] text-muted-foreground transition-colors hover:text-foreground"
            >
              {copiedSecondary ? (
                <>
                  <Check className="h-3.5 w-3.5 text-success" />
                  <span className="text-success">Copied</span>
                </>
              ) : (
                <>
                  <Copy className="h-3.5 w-3.5" />
                  <span>Copy</span>
                </>
              )}
            </button>
          </div>
        )}
      </div>
    </aside>
  );
}
