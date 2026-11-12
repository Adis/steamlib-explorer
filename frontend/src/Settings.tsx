import { Check } from "lucide-react";
import { THEMES, useTheme } from "./theme.ts";

/** Settings page (#settings). Settings are saved in this browser. */
export default function Settings({ signedIn }: { signedIn: boolean }) {
  return (
    <article className="mx-auto max-w-3xl space-y-8">
      <header>
        <a href="#" className="text-sm text-link hover:text-link-hover">
          {signedIn ? "← Back to your library" : "← Back to sign in"}
        </a>
        <h2 className="mt-3 text-2xl font-semibold tracking-tight">Settings</h2>
        <p className="mt-1 text-sm text-muted">Saved in this browser.</p>
      </header>

      <section className="rounded-xl border border-line bg-surface p-5">
        <ThemePicker />
      </section>
    </article>
  );
}

function ThemePicker() {
  const [theme, setTheme] = useTheme();
  return (
    <fieldset>
      <legend className="font-semibold tracking-tight">Theme</legend>
      <p className="mt-0.5 mb-4 text-sm text-muted">Colours used across the app.</p>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {THEMES.map((t) => {
          const selected = theme === t.id;
          return (
            <button
              key={t.id}
              onClick={() => setTheme(t.id)}
              aria-pressed={selected}
              className={`rounded-lg border p-2 text-left text-sm transition-colors ${selected ? "border-accent ring-1 ring-accent" : "border-line-strong hover:border-faint"}`}
            >
              <ThemePreview colors={t.swatch} />
              <span className="mt-1.5 flex items-center justify-between gap-1 px-0.5">
                <span className={selected ? "font-medium text-fg" : "text-muted"}>{t.name}</span>
                {selected && <Check aria-hidden className="h-3.5 w-3.5 shrink-0 text-accent" />}
              </span>
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}

/** A tiny mock of the page in the theme's colours: canvas, a card and an accent button. */
function ThemePreview({ colors }: { colors: readonly [string, string, string] }) {
  const [canvas, surface, accent] = colors;
  return (
    <span
      aria-hidden
      className="flex h-20 flex-col justify-between rounded-md border border-black/10 p-1.5"
      style={{ background: canvas }}
    >
      <span className="h-2 w-14 rounded-full" style={{ background: surface }} />
      <span className="flex items-end justify-between">
        <span className="h-7 w-20 rounded" style={{ background: surface }} />
        <span className="h-3.5 w-9 rounded-sm" style={{ background: accent }} />
      </span>
    </span>
  );
}
