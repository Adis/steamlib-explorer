import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchFacets, type Facet } from "./api.ts";
import { activeFilterCount, type SetParam, type UrlParams } from "./filters.ts";

type Props = {
  params: UrlParams;
  setParam: SetParam;
  clearFilters: () => void;
};

export default function FilterPanel({ params, setParam, clearFilters }: Props) {
  const facets = useQuery({ queryKey: ["facets"], queryFn: fetchFacets });
  const active = activeFilterCount(params);

  return (
    <aside className="space-y-6 text-sm">
      <div className="flex items-center justify-between">
        <h2 className="font-semibold tracking-tight">Filters</h2>
        {active > 0 && (
          <button
            onClick={clearFilters}
            className="text-link hover:text-link-hover"
          >
            Clear all ({active})
          </button>
        )}
      </div>

      <SearchBox value={params.get("q") ?? ""} onChange={(v) => setParam("q", v)} />

      <Group title="Played">
        <Segmented
          value={params.get("played") ?? ""}
          options={[
            ["", "All"],
            ["true", "Played"],
            ["false", "Unplayed"],
          ]}
          onChange={(v) => setParam("played", v || null)}
        />
      </Group>

      <Group title="Playtime (hours)">
        <Range
          params={params}
          setParam={setParam}
          minKey="min_hours"
          maxKey="max_hours"
          step="any"
          placeholder={["0", "any"]}
        />
      </Group>

      <Group title="Steam reviews">
        <Select
          value={params.get("min_review_score") ?? ""}
          onChange={(v) => setParam("min_review_score", v || null)}
          options={REVIEW_RATINGS}
        />
      </Group>

      <Group title="Metacritic">
        <Select
          value={params.get("min_metacritic") ?? ""}
          onChange={(v) => setParam("min_metacritic", v || null)}
          options={[["", "Any"], ...[90, 80, 70, 60, 50].map((n) => [String(n), `${n} or more`] as const)]}
        />
      </Group>

      <Group title="Release year">
        <Range
          params={params}
          setParam={setParam}
          minKey="from_year"
          maxKey="to_year"
          step="1"
          placeholder={["from", "to"]}
        />
      </Group>

      <Group title="Platforms">
        <div className="flex flex-wrap gap-x-4 gap-y-1">
          {[
            ["windows", "Windows"],
            ["mac", "macOS"],
            ["linux", "Linux / SteamOS"],
          ].map(([value, label]) => (
            <Check
              key={value}
              label={label}
              checked={params.getAll("platform").includes(value)}
              onChange={() => setParam("platform", toggle(params.getAll("platform"), value))}
            />
          ))}
        </div>
      </Group>

      {facets.isError && (
        <p className="text-amber-300">Could not load genres: {facets.error.message}</p>
      )}
      <FacetList
        title="Genres"
        facets={facets.data?.genres}
        selected={params.getAll("genre")}
        onChange={(v) => setParam("genre", v)}
      />
      <FacetList
        title="Features"
        facets={facets.data?.categories}
        selected={params.getAll("category")}
        onChange={(v) => setParam("category", v)}
      />
    </aside>
  );
}

// Steam's own review ratings (review_score 9..5), each meaning "this or better".
const REVIEW_RATINGS: (readonly [string, string])[] = [
  ["", "Any"],
  ["9", "Overwhelmingly Positive"],
  ["8", "Very Positive or better"],
  ["7", "Positive or better"],
  ["6", "Mostly Positive or better"],
  ["5", "Mixed or better"],
];

function toggle(list: string[], value: string) {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

function Group({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <fieldset>
      <legend className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-faint">
        {title}
      </legend>
      {children}
    </fieldset>
  );
}

const inputClass =
  "w-full rounded-md border border-line-strong bg-surface px-2.5 py-1.5 text-fg placeholder:text-faint transition-colors hover:border-faint focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/25";

/** Name search; waits for a pause in typing before filtering. */
function SearchBox({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [text, setText] = useState(value);
  const pushed = useRef(value);

  // Follow outside changes such as "Clear all" or the back button.
  useEffect(() => {
    if (value !== pushed.current) {
      pushed.current = value;
      setText(value);
    }
  }, [value]);

  useEffect(() => {
    if (text === pushed.current) return;
    const t = setTimeout(() => {
      pushed.current = text;
      onChange(text);
    }, 300);
    return () => clearTimeout(t);
  }, [text, onChange]);

  return (
    <input
      type="search"
      value={text}
      onChange={(e) => setText(e.target.value)}
      placeholder="Search by name"
      aria-label="Search by name"
      className={inputClass}
    />
  );
}

function Range(props: {
  params: UrlParams;
  setParam: SetParam;
  minKey: string;
  maxKey: string;
  step: string;
  placeholder: [string, string];
}) {
  const { params, setParam, minKey, maxKey, step, placeholder } = props;
  return (
    <div className="flex items-center gap-2">
      <input
        type="number"
        min={0}
        step={step}
        value={params.get(minKey) ?? ""}
        onChange={(e) => setParam(minKey, e.target.value || null)}
        placeholder={placeholder[0]}
        aria-label={`${minKey.replace("_", " ")}`}
        className={inputClass}
      />
      <span className="text-faint">to</span>
      <input
        type="number"
        min={0}
        step={step}
        value={params.get(maxKey) ?? ""}
        onChange={(e) => setParam(maxKey, e.target.value || null)}
        placeholder={placeholder[1]}
        aria-label={`${maxKey.replace("_", " ")}`}
        className={inputClass}
      />
    </div>
  );
}

function Select({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: (readonly [string, string])[];
}) {
  return (
    <select value={value} onChange={(e) => onChange(e.target.value)} className={inputClass}>
      {options.map(([v, label]) => (
        <option key={v} value={v}>
          {label}
        </option>
      ))}
    </select>
  );
}

function Segmented({
  value,
  options,
  onChange,
}: {
  value: string;
  options: [string, string][];
  onChange: (v: string) => void;
}) {
  return (
    <div className="flex gap-0.5 rounded-md border border-line-strong bg-surface p-0.5">
      {options.map(([v, label]) => (
        <button
          key={v}
          onClick={() => onChange(v)}
          aria-pressed={value === v}
          className={`flex-1 rounded px-2 py-1 transition-colors ${value === v ? "bg-accent font-medium text-on-accent shadow-sm" : "text-muted hover:bg-raised hover:text-fg"}`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}

function Check({
  label,
  checked,
  onChange,
  count,
}: {
  label: string;
  checked: boolean;
  onChange: () => void;
  count?: number;
}) {
  return (
    <label className="-mx-1 flex cursor-pointer items-center gap-2 rounded px-1 py-0.5 hover:bg-surface">
      <input type="checkbox" checked={checked} onChange={onChange} className="h-3.5 w-3.5 accent-(--color-accent)" />
      <span className="flex-1">{label}</span>
      {count !== undefined && <span className="tabular-nums text-faint">{count}</span>}
    </label>
  );
}

const COLLAPSED = 8;

/** Genre or category checkboxes; a game must have every checked one. */
function FacetList({
  title,
  facets,
  selected,
  onChange,
}: {
  title: string;
  facets: Facet[] | undefined;
  selected: string[];
  onChange: (v: string[]) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  if (!facets || facets.length === 0) return null;

  // Checked values stay visible when the list is collapsed.
  const shown = expanded
    ? facets
    : facets.filter((f, i) => i < COLLAPSED || selected.includes(f.value));

  return (
    <Group title={title}>
      {selected.length > 1 && (
        <p className="mb-1 text-xs text-faint">Showing games with all of them</p>
      )}
      {shown.map((f) => (
        <Check
          key={f.value}
          label={f.value}
          count={f.games}
          checked={selected.includes(f.value)}
          onChange={() => onChange(toggle(selected, f.value))}
        />
      ))}
      {facets.length > COLLAPSED && (
        <button
          onClick={() => setExpanded(!expanded)}
          className="mt-1 text-link hover:text-link-hover"
        >
          {expanded ? "Show fewer" : `Show all ${facets.length}`}
        </button>
      )}
    </Group>
  );
}
