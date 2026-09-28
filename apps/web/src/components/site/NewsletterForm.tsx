const BUTTONDOWN_USERNAME = "he-690021";

export const NewsletterForm = () => (
  <form
    action={`https://buttondown.com/api/emails/embed-subscribe/${BUTTONDOWN_USERNAME}`}
    method="post"
    target="popupwindow"
    onSubmit={() => {
      window.open(`https://buttondown.com/${BUTTONDOWN_USERNAME}`, "popupwindow");
    }}
    className="flex flex-col gap-3"
  >
    <h4 className="mono-label">Stay in the loop</h4>
    <p className="max-w-xs text-[13px] leading-relaxed text-muted-foreground">
      New releases and compliance guides. No spam.
    </p>
    <div className="flex gap-2">
      <input
        type="email"
        name="email"
        required
        placeholder="you@example.com"
        className="h-9 w-full max-w-[200px] rounded-md border border-border bg-surface px-3 text-[13px] text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
      />
      <input type="hidden" value="1" name="embed" />
      <button
        type="submit"
        className="h-9 shrink-0 rounded-md bg-foreground px-3 text-[13px] font-semibold text-background hover:bg-foreground/90 transition-colors"
      >
        Subscribe
      </button>
    </div>
  </form>
);
