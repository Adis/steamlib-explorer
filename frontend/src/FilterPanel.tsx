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
        <h2 className="font-semibold">Filters</h2>
        {active > 0 && (
          <button
            onClick={clearFilters}
            className="text-sky-400 hover:text-sky-300"
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

      <Group title="Positive reviews">
        <Select
          value={params.get("min_review") ?? ""}
          onChange={(v) => setParam("min_review", v || null)}
          options={[["", "Any"], ...[95, 90, 80, 70, 50].map((n) => [String(n), `${n}% or more`] as const)]}
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

function toggle(list: string[], value: string) {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

function Group({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <fieldset>
      <legend className="mb-2 text-xs font-medium uppercase tracking-wide text-slate-400">
        {title}
      </legend>
      {children}
    </fieldset>
  );
}

const inputClass =
  "w-full rounded border border-slate-700 bg-slate-900 px-2.5 py-1.5 text-slate-100 placeholder:text-slate-500 focus:border-sky-500 focus:outline-none";

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
      <span className="text-slate-500">to</span>
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
    <div className="flex overflow-hidden rounded border border-slate-700">
      {options.map(([v, label]) => (
        <button
          key={v}
          onClick={() => onChange(v)}
          aria-pressed={value === v}
          className={`flex-1 px-2 py-1.5 ${value === v ? "bg-sky-600 text-white" : "hover:bg-slate-800"}`}
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
    <label className="flex cursor-pointer items-center gap-2 py-0.5">
      <input type="checkbox" checked={checked} onChange={onChange} className="accent-sky-500" />
      <span className="flex-1">{label}</span>
      {count !== undefined && <span className="tabular-nums text-slate-500">{count}</span>}
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
        <p className="mb-1 text-xs text-slate-500">Showing games with all of them</p>
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
          className="mt-1 text-sky-400 hover:text-sky-300"
        >
          {expanded ? "Show fewer" : `Show all ${facets.length}`}
        </button>
      )}
    </Group>
  );
}
