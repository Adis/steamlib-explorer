import { CheckIcon } from "./icons.tsx";
import McpAccess from "./McpAccess.tsx";
import { SIDEBAR_FILTERS, useSidebarFilters } from "./sidebarFilters.ts";
import { TABLE_COLUMNS, useTableColumns } from "./tableColumns.ts";
import {
  COLOR_VISIONS,
  isAccessible,
  TEXT_SIZES,
  THEMES,
  useColorVision,
  useReduceMotion,
  useTextSize,
  useTheme,
  type Theme,
  type ThemeId,
} from "./theme.ts";

/** Settings page (#settings). Settings are saved in this browser. */
export default function Settings({ signedIn }: { signedIn: boolean }) {
  // One theme choice, offered in two sections: Theme and Accessibility.
  const [theme, setTheme] = useTheme();
  return (
    <article className="mx-auto max-w-3xl space-y-8">
      <header>
        <a href="#" className="text-sm text-link hover:text-link-hover">
          {signedIn ? "← Back to your library" : "← Back to sign in"}
        </a>
        <h2 className="page-title mt-3 text-2xl">Settings</h2>
        <p className="mt-1 text-sm text-muted">Saved in this browser, except the AI assistant token.</p>
      </header>

      <section className="panel rounded-xl border border-line bg-surface p-5">
        <fieldset>
          <legend className="heading">Theme</legend>
          <p className="mt-0.5 mb-4 text-sm text-muted">Colours used across the app.</p>
          <ThemeOptions themes={THEMES.filter((t) => !isAccessible(t))} theme={theme} setTheme={setTheme} />
        </fieldset>
      </section>

      <section className="panel rounded-xl border border-line bg-surface p-5">
        <h3 className="heading">Accessibility</h3>
        <fieldset className="mt-4">
          <legend className="text-sm font-medium">High contrast</legend>
          <p className="mt-0.5 mb-3 text-sm text-muted">
            Very strong contrast for all text, bold edges and a thick focus ring. Picking one
            replaces the theme above.
          </p>
          <ThemeOptions themes={THEMES.filter(isAccessible)} theme={theme} setTheme={setTheme} />
        </fieldset>
        <TextSizePicker />
        <ColorVisionPicker />
        <ReduceMotionSwitch />
      </section>

      <section className="panel rounded-xl border border-line bg-surface p-5">
        <SidebarFilterToggles />
      </section>

      <section className="panel rounded-xl border border-line bg-surface p-5">
        <TableColumnToggles />
      </section>

      {signedIn && (
        <section className="panel rounded-xl border border-line bg-surface p-5">
          <McpAccess />
        </section>
      )}
    </article>
  );
}

/** A row of theme cards; the chosen one is marked. */
function ThemeOptions({
  themes,
  theme,
  setTheme,
}: {
  themes: readonly Theme[];
  theme: ThemeId;
  setTheme: (id: ThemeId) => void;
}) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
      {themes.map((t) => {
        const selected = theme === t.id;
        return (
          <button
            key={t.id}
            onClick={() => setTheme(t.id)}
            aria-pressed={selected}
            className={`theme-option flex flex-col rounded-lg border p-2 text-left text-sm transition-colors ${selected ? "border-accent ring-1 ring-accent" : "border-line-strong hover:border-control"}`}
          >
            <ThemePreview colors={t.swatch} />
            <span className="mt-1.5 flex items-center justify-between gap-1 px-0.5">
              <span className={selected ? "font-medium text-fg" : "text-fg"}>{t.name}</span>
              {selected && <CheckIcon className="h-3.5 w-3.5 text-accent" />}
            </span>
          </button>
        );
      })}
    </div>
  );
}

/** Text size for the whole app, on top of any theme. */
function TextSizePicker() {
  const [size, setSize] = useTextSize();
  return (
    <fieldset className="mt-6">
      <legend className="text-sm font-medium">Text size</legend>
      <p className="mt-0.5 mb-3 text-sm text-muted">Makes text and the space around it larger. Works with any theme.</p>
      <div className="flex flex-wrap gap-3">
        {TEXT_SIZES.map((s) => {
          const selected = size === s;
          return (
            <button
              key={s}
              onClick={() => setSize(s)}
              aria-pressed={selected}
              className={`theme-option flex min-w-24 items-baseline justify-center gap-2 rounded-lg border px-4 py-2 transition-colors ${selected ? "border-accent ring-1 ring-accent" : "border-line-strong hover:border-control"}`}
            >
              <span aria-hidden className="leading-none text-fg" style={{ fontSize: `${(16 * s) / 100}px` }}>
                Aa
              </span>
              <span className="text-sm text-muted">{s}%</span>
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

function SidebarFilterToggles() {
  const { isShown, setShown } = useSidebarFilters();
  return (
    <ToggleGroup
      title="Sidebar filters"
      description="Choose which filters the library sidebar shows. A hidden filter that is already set still applies until you clear it."
      items={SIDEBAR_FILTERS}
      isShown={isShown}
      setShown={setShown}
    />
  );
}

function TableColumnToggles() {
  const { isShown, setShown } = useTableColumns();
  return (
    <ToggleGroup
      title="Table columns"
      description="Choose which columns the table view shows. The game name is always shown."
      items={TABLE_COLUMNS}
      isShown={isShown}
      setShown={setShown}
    />
  );
}

/** A titled grid of on/off switches, one per item. */
/** Sample rating colours for each colour vision option, best to worst (dark themes). */
const CV_SAMPLES: Record<string, string[]> = {
  default: ["#34d399", "#a3e635", "#fbbf24", "#fb7185"],
  rg: ["#3b9eff", "#93c5fd", "#a1a1aa", "#f59e0b"],
  by: ["#2dd4bf", "#a5f3fc", "#a1a1aa", "#f87171"],
};

/** Review rating colours that work with colour blindness, on top of any theme. */
function ColorVisionPicker() {
  const [cv, setCv] = useColorVision();
  return (
    <fieldset className="mt-6">
      <legend className="text-sm font-medium">Colour vision</legend>
      <p className="mt-0.5 mb-3 text-sm text-muted">
        Colours for Steam review ratings. Ratings also show a thumbs up, dash or thumbs down, so
        they never rely on colour alone. Works with any theme.
      </p>
      <div className="grid gap-3 sm:grid-cols-3">
        {COLOR_VISIONS.map((c) => {
          const selected = cv === c.id;
          return (
            <button
              key={c.id}
              onClick={() => setCv(c.id)}
              aria-pressed={selected}
              className={`theme-option flex flex-col gap-1.5 rounded-lg border p-3 text-left text-sm transition-colors ${selected ? "border-accent ring-1 ring-accent" : "border-line-strong hover:border-control"}`}
            >
              <span aria-hidden className="flex gap-1">
                {CV_SAMPLES[c.id].map((color) => (
                  <span key={color} className="h-3 w-6 rounded-sm" style={{ background: color }} />
                ))}
              </span>
              <span className="flex items-center justify-between gap-1">
                <span className={selected ? "font-medium text-fg" : "text-fg"}>{c.name}</span>
                {selected && <CheckIcon className="h-3.5 w-3.5 text-accent" />}
              </span>
              <span className="text-xs text-muted">{c.detail}</span>
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}

function ReduceMotionSwitch() {
  const [on, setOn] = useReduceMotion();
  return (
    <div className="mt-6">
      <SwitchRow
        label="Reduce motion"
        description="Turns off animations: the card lift and cover zoom on hover, and the spinning sync icon. Follows your system setting until you change it here."
        on={on}
        setOn={setOn}
      />
    </div>
  );
}

/** A label with an on/off switch on the right. */
function SwitchRow({
  label,
  description,
  on,
  setOn,
}: {
  label: string;
  description?: string;
  on: boolean;
  setOn: (on: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-3 border-b border-line py-2.5 text-sm">
      <span>
        <span className={description ? "font-medium text-fg" : on ? "text-fg" : "text-muted"}>{label}</span>
        {description && <span className="mt-0.5 block text-muted">{description}</span>}
      </span>
      <input
        type="checkbox"
        role="switch"
        checked={on}
        onChange={(e) => setOn(e.target.checked)}
        className="peer sr-only"
      />
      <span
        aria-hidden
        className="relative h-5 w-9 shrink-0 rounded-full bg-control transition-colors peer-checked:bg-accent peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-accent after:absolute after:left-0.5 after:top-0.5 after:h-4 after:w-4 after:rounded-full after:bg-white after:shadow after:transition-transform peer-checked:after:translate-x-4"
      />
    </label>
  );
}

function ToggleGroup<Id extends string>({
  title,
  description,
  items,
  isShown,
  setShown,
}: {
  title: string;
  description: string;
  items: readonly { id: Id; label: string }[];
  isShown: (id: Id) => boolean;
  setShown: (id: Id, shown: boolean) => void;
}) {
  return (
    <fieldset>
      <legend className="heading">{title}</legend>
      <p className="mt-0.5 mb-4 text-sm text-muted">{description}</p>
      <div className="grid gap-x-6 sm:grid-cols-2">
        {items.map((f) => (
          <SwitchRow key={f.id} label={f.label} on={isShown(f.id)} setOn={(on) => setShown(f.id, on)} />
        ))}
      </div>
    </fieldset>
  );
}
