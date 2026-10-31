import { ReactNode, useId, useLayoutEffect, useRef, useState } from "react";
import {
  Area,
  Bar,
  BarChart,
  CartesianGrid,
  ComposedChart,
  Line,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { autoColor } from "./PmChip";
import { useThemeColors } from "./appearance/AppearanceContext";

// 차트 공통 규격(DESIGN_SYSTEM §11). 색은 useThemeColors() 또는 테마 CSS 변수(--c-*)에서만 얻는다.

// 테마 CSS 변수를 SVG/인라인 스타일에서 쓰는 색 문자열로. 예: cv("emerald-400"), cv("mint-400", 0.3)
export const cv = (name: string, alpha?: number) =>
  alpha === undefined ? `rgb(var(--c-${name}))` : `rgb(var(--c-${name}) / ${alpha})`;

// 이 앱의 의미색: 수입 = emerald, 지출 = rose, 경고 = amber
const INCOME = cv("emerald-400");
const EXPENSE = cv("rose-400");

// 축 숫자(§11.5). 값 표시는 풀 포맷(won), 축은 compact.
// 음수(잔액 부족 등)도 같은 단위로 줄인다 — 부호만 앞에 붙인다
export const compact = (n: number): string => {
  if (n < 0) return `-${compact(-n)}`;
  return n >= 100000000
    ? `${(n / 100000000).toFixed(1)}억`
    : n >= 10000
    ? `${Math.round(n / 10000).toLocaleString()}만`
    : Math.round(n).toLocaleString("ko-KR");
};

// 축 눈금 글자 규격: 11px mono
export function useAxisTick() {
  const tc = useThemeColors();
  return { fontSize: 11, fill: tc.axis, fontFamily: "var(--font-mono)" };
}

// useChartWidth 는 차트를 그릴 실제 폭(px)을 잰다. (직접 그리는 SVG 용)
export function useChartWidth(fallback = 660) {
  const ref = useRef<HTMLDivElement>(null);
  const [w, setW] = useState(fallback);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => setW(Math.max(240, Math.round(el.clientWidth)));
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, w] as const;
}

// 빈 데이터 문구
export function ChartEmpty({ children = "데이터 없음" }: { children?: ReactNode }) {
  return <p className="label-mono text-zinc-500 text-center py-8 normal-case">{children}</p>;
}

// 범례 점
export function LegendDot({ color, dashed, faint }: { color: string; dashed?: boolean; faint?: boolean }) {
  return dashed ? (
    <i className="inline-block w-3.5 border-t-2 border-dashed" style={{ borderColor: color }} />
  ) : (
    <i
      className="inline-block w-2 h-2 rounded-full shrink-0"
      style={{ background: color, opacity: faint ? 0.5 : 1 }}
    />
  );
}

export type TipRow = { color?: string; value: string; name?: string; dashed?: boolean; faint?: boolean; sub?: boolean };

// 테마 툴팁(§11.1): 라벨 + 계열별 줄
export function ChartTip({ head, rows }: { head: ReactNode; rows: TipRow[] }) {
  const tc = useThemeColors();
  return (
    <div style={{ ...tc.tooltip.contentStyle, padding: "10px 12px", pointerEvents: "none" }}>
      <div style={{ ...tc.tooltip.labelStyle, marginBottom: 4 }}>{head}</div>
      {rows.map((r, i) => (
        <div key={i} className="flex items-center gap-2 num" style={{ color: r.color ?? tc.tooltip.itemStyle.color, opacity: r.sub ? 0.7 : 1 }}>
          {r.color ? <LegendDot color={r.color} dashed={r.dashed} faint={r.faint} /> : <i className="w-2" />}
          <strong className="font-semibold">{r.value}</strong>
          {r.name && <span style={tc.tooltip.labelStyle}>{r.name}</span>}
        </div>
      ))}
    </div>
  );
}

const safeId = (s: string) => s.replace(/[^a-zA-Z0-9_-]/g, "");

// 최근 n개월 수입/지출 막대 추이
export function TrendChart({ points }: { points: store.MonthPoint[] }) {
  const tc = useThemeColors();
  const tick = useAxisTick();
  const data = points.map((p) => ({ label: `${Number(p.month.slice(5))}월`, income: p.income, expense: p.expense }));
  return (
    <div>
      <div className="flex gap-4 mb-2 text-[11px]" style={{ color: tc.muted }}>
        <span className="inline-flex items-center gap-1.5"><LegendDot color={INCOME} /> 수입</span>
        <span className="inline-flex items-center gap-1.5"><LegendDot color={EXPENSE} /> 지출</span>
      </div>
      <div className="h-60">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 5, right: 10, left: 0, bottom: 0 }} barGap={4}>
            <CartesianGrid stroke={tc.grid} strokeOpacity={0.4} vertical={false} />
            <XAxis dataKey="label" tick={tick} axisLine={{ stroke: tc.grid }} tickLine={false} />
            <YAxis tick={tick} domain={[0, "auto"]} width={55} axisLine={false} tickLine={false} tickFormatter={compact} />
            <Tooltip
              cursor={{ fill: tc.text, fillOpacity: 0.05 }}
              content={({ active, payload, label }) =>
                active && payload?.length ? (
                  <ChartTip
                    head={label}
                    rows={[
                      { color: INCOME, value: won(Number(payload.find((p) => p.dataKey === "income")?.value ?? 0)), name: "수입" },
                      { color: EXPENSE, value: won(Number(payload.find((p) => p.dataKey === "expense")?.value ?? 0)), name: "지출" },
                    ]}
                  />
                ) : null
              }
            />
            <Bar dataKey="income" fill={INCOME} radius={[4, 4, 0, 0]} maxBarSize={26} isAnimationActive={false} />
            <Bar dataKey="expense" fill={EXPENSE} radius={[4, 4, 0, 0]} maxBarSize={26} isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

// 이번 달 일별 지출 영역 그래프 + 누적선
export function DailyChart({ points }: { points: store.DayPoint[] }) {
  const tc = useThemeColors();
  const tick = useAxisTick();
  const gid = safeId(useId());
  let acc = 0;
  const data = points.map((p) => ({ day: p.day, amount: p.amount, cum: (acc += p.amount) }));
  const cumColor = tc.palette[1] ?? tc.axis;
  return (
    <div>
      <div className="flex flex-wrap gap-4 mb-2 text-[11px]" style={{ color: tc.muted }}>
        <span className="inline-flex items-center gap-1.5"><LegendDot color={EXPENSE} /> 일별 지출</span>
        <span className="inline-flex items-center gap-1.5">
          <LegendDot color={cumColor} dashed /> 누적 (이번 달 총 <span className="num">{won(acc)}</span>)
        </span>
      </div>
      <div className="h-52">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data} margin={{ top: 5, right: 10, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id={`${gid}-d`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={EXPENSE} stopOpacity={0.32} />
                <stop offset="100%" stopColor={EXPENSE} stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid stroke={tc.grid} strokeOpacity={0.4} vertical={false} />
            <XAxis dataKey="day" tick={tick} minTickGap={30} axisLine={{ stroke: tc.grid }} tickLine={false} />
            <YAxis yAxisId="a" tick={tick} domain={[0, "auto"]} width={55} axisLine={false} tickLine={false} tickFormatter={compact} />
            <YAxis yAxisId="c" hide domain={[0, "auto"]} />
            <Tooltip
              cursor={{ stroke: tc.axis, strokeDasharray: "3 3" }}
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                const row = payload[0].payload as { day: number; amount: number; cum: number };
                return (
                  <ChartTip
                    head={`${row.day}일`}
                    rows={[
                      { color: EXPENSE, value: won(row.amount), name: "지출" },
                      { color: cumColor, value: won(row.cum), name: "누적", dashed: true },
                    ]}
                  />
                );
              }}
            />
            <Area yAxisId="a" type="monotone" dataKey="amount" stroke={EXPENSE} strokeWidth={1.75} fill={`url(#${gid}-d)`} dot={false} isAnimationActive={false} />
            <Line yAxisId="c" type="monotone" dataKey="cum" stroke={cumColor} strokeWidth={1.5} strokeDasharray="5 4" dot={false} isAnimationActive={false} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

// 도넛 차트: 상위 7개 + 기타 (팔레트를 돌려 쓰지 않는다)
export function Donut({ rows, centerLabel }: { rows: store.NamedAmount[]; centerLabel: string }) {
  const tc = useThemeColors();
  const total = rows.reduce((s, r) => s + r.amount, 0);
  if (total === 0) return <ChartEmpty />;
  const topN = Math.min(7, tc.palette.length);
  const top = rows.slice(0, topN);
  const rest = rows.slice(topN).reduce((s, r) => s + r.amount, 0);
  const segs = rest > 0 ? [...top, { name: "기타", kind: "expense", amount: rest }] : top;
  const colorOf = (i: number, name: string) => (name === "기타" && rest > 0 && i === segs.length - 1 ? tc.axis : tc.palette[i] ?? tc.axis);

  const R = 62;
  const CIRC = 2 * Math.PI * R;
  let offset = 0;

  return (
    <div className="flex flex-wrap items-center gap-5">
      <svg viewBox="0 0 170 170" className="w-40 h-40 shrink-0">
        {segs.map((s, i) => {
          const frac = s.amount / total;
          const el = (
            <circle
              key={i}
              cx={85} cy={85} r={R}
              fill="none"
              stroke={colorOf(i, s.name)}
              strokeWidth={24}
              strokeDasharray={`${frac * CIRC} ${CIRC}`}
              strokeDashoffset={-offset * CIRC}
              transform="rotate(-90 85 85)"
              strokeLinecap="butt"
            />
          );
          offset += frac;
          return el;
        })}
        <text x={85} y={80} textAnchor="middle" fontSize={11} fill={tc.axis} fontFamily="var(--font-mono)">{centerLabel}</text>
        <text x={85} y={98} textAnchor="middle" fontSize={13} fontWeight={700} fill={tc.text} fontFamily="var(--font-mono)">
          {compact(total)}원
        </text>
      </svg>
      <ul className="flex-1 min-w-[180px] space-y-1.5 text-xs">
        {segs.map((s, i) => (
          <li key={i} className="flex items-center gap-2">
            <LegendDot color={colorOf(i, s.name)} />
            <span className="flex-1 truncate text-zinc-300">{s.name}</span>
            <span className="num text-zinc-500">{Math.round((s.amount / total) * 100)}%</span>
            <span className="num text-white w-24 text-right">{won(s.amount)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export type LineSeries = { name: string; color?: string; values: number[] };

// monotonePath 는 점들을 지나는 부드러운 곡선(단조 3차 보간, Fritsch–Carlson).
export function monotonePath(pts: [number, number][]): string {
  const n = pts.length;
  if (n === 0) return "";
  if (n < 3) return pts.map((p, i) => `${i ? "L" : "M"} ${p[0]} ${p[1]}`).join(" ");
  const dx: number[] = [];
  const m: number[] = [];
  for (let i = 0; i < n - 1; i++) {
    dx[i] = pts[i + 1][0] - pts[i][0];
    m[i] = dx[i] === 0 ? 0 : (pts[i + 1][1] - pts[i][1]) / dx[i];
  }
  const t: number[] = new Array(n);
  t[0] = m[0];
  t[n - 1] = m[n - 2];
  for (let i = 1; i < n - 1; i++) t[i] = m[i - 1] * m[i] <= 0 ? 0 : (m[i - 1] + m[i]) / 2;
  for (let i = 0; i < n - 1; i++) {
    if (m[i] === 0) {
      t[i] = 0;
      t[i + 1] = 0;
      continue;
    }
    const a = t[i] / m[i];
    const b = t[i + 1] / m[i];
    const s = a * a + b * b;
    if (s > 9) {
      const k = 3 / Math.sqrt(s);
      t[i] = k * a * m[i];
      t[i + 1] = k * b * m[i];
    }
  }
  let d = `M ${pts[0][0]} ${pts[0][1]}`;
  for (let i = 0; i < n - 1; i++) {
    const h = dx[i] / 3;
    d +=
      ` C ${pts[i][0] + h} ${pts[i][1] + t[i] * h}` +
      ` ${pts[i + 1][0] - h} ${pts[i + 1][1] - t[i + 1] * h}` +
      ` ${pts[i + 1][0]} ${pts[i + 1][1]}`;
  }
  return d;
}

// niceMax 는 축 최댓값을 보기 좋은 수(1·2·2.5·5 ×10ⁿ)로 올린다.
export function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v)));
  for (const k of [1, 1.2, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10]) {
    if (k * p >= v) return k * p;
  }
  return 10 * p;
}

// 멀티라인 그래프. cumulative=true 면 각 시리즈를 누적합으로 그린다(범례 합계는 원본 합).
// marker 를 주면 해당 x 위치(1-based)에 점선 세로선을 그린다.
// 마우스를 올리면 십자선이 가장 가까운 x 에 붙고, 그 x 의 모든 시리즈 값을 툴팁에 보여 준다.
export function LineChart({
  series,
  xLabels,
  cumulative = false,
  height = 220,
  marker,
  showLegend = true,
  valueFmt = won,
}: {
  series: LineSeries[];
  xLabels: string[];
  cumulative?: boolean;
  height?: number;
  marker?: { x: number; label?: string };
  showLegend?: boolean;
  valueFmt?: (n: number) => string;
}) {
  const tc = useThemeColors();
  const tick = useAxisTick();
  const gid = safeId(useId());
  const n = xLabels.length;
  const color = (i: number) => series[i].color || tc.palette[i] || tc.axis;

  const plotted = series.map((s) => {
    if (!cumulative) return s.values;
    let acc = 0;
    return s.values.map((v) => (acc += v));
  });
  const totals = series.map((s) => s.values.reduce((a, b) => a + b, 0));
  const hasData = totals.some((t) => t > 0);
  const single = series.length === 1;
  const interval = n <= 12 ? 0 : n <= 20 ? 1 : 4;

  const data = xLabels.map((_, i) => {
    const row: Record<string, number> = { i };
    plotted.forEach((vals, si) => (row[`s${si}`] = vals[i] ?? 0));
    return row;
  });

  return (
    <div>
      {showLegend && (
        <div className="flex flex-wrap gap-x-4 gap-y-1 mb-2 text-[11px]" style={{ color: tc.muted }}>
          {series.map((s, i) => (
            <span key={i} className="inline-flex items-center gap-1.5">
              <LegendDot color={color(i)} /> {s.name}
              <em className="not-italic num text-zinc-500">{valueFmt(totals[i])}</em>
            </span>
          ))}
        </div>
      )}
      {!hasData ? (
        <ChartEmpty />
      ) : (
        <div style={{ height }}>
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={data} margin={{ top: 14, right: 16, left: 0, bottom: 0 }}>
              <defs>
                {single && (
                  <linearGradient id={`${gid}-a`} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={color(0)} stopOpacity={0.32} />
                    <stop offset="100%" stopColor={color(0)} stopOpacity={0} />
                  </linearGradient>
                )}
              </defs>
              <CartesianGrid stroke={tc.grid} strokeOpacity={0.4} vertical={false} />
              <XAxis
                dataKey="i"
                tick={tick}
                interval={interval}
                tickFormatter={(i: number) => xLabels[i] ?? ""}
                axisLine={{ stroke: tc.grid }}
                tickLine={false}
              />
              <YAxis
                tick={tick}
                width={55}
                axisLine={false}
                tickLine={false}
                tickFormatter={compact}
                tickCount={5}
                domain={[0, (max: number) => niceMax(Math.max(1, max))]}
              />
              {marker && marker.x >= 1 && marker.x <= n && (
                <ReferenceLine
                  x={marker.x - 1}
                  stroke={tc.axis}
                  strokeDasharray="3 3"
                  label={marker.label ? { value: marker.label, position: "top", fill: tc.axis, fontSize: 11 } : undefined}
                />
              )}
              <Tooltip
                cursor={{ stroke: tc.axis, strokeDasharray: "3 3" }}
                content={({ active, payload }) => {
                  if (!active || !payload?.length) return null;
                  const i = (payload[0].payload as { i: number }).i;
                  const rows = series
                    .map((s, si) => ({ name: s.name, color: color(si), v: plotted[si][i] ?? 0 }))
                    .sort((a, b) => b.v - a.v)
                    .slice(0, 8);
                  return <ChartTip head={xLabels[i]} rows={rows.map((r) => ({ color: r.color, value: valueFmt(r.v), name: r.name }))} />;
                }}
              />
              {series.map((_, si) =>
                single ? (
                  <Area
                    key={si}
                    type="monotone"
                    dataKey={`s${si}`}
                    stroke={color(si)}
                    strokeWidth={1.75}
                    fill={`url(#${gid}-a)`}
                    isAnimationActive={false}
                    dot={(p: { cx?: number; cy?: number; index?: number }) =>
                      p.index === n - 1 && p.cx !== undefined ? (
                        <circle key="end" cx={p.cx} cy={p.cy} r={3} fill={color(si)} />
                      ) : (
                        <g key={`d${p.index}`} />
                      )
                    }
                  />
                ) : (
                  <Line
                    key={si}
                    type="monotone"
                    dataKey={`s${si}`}
                    stroke={color(si)}
                    strokeWidth={1.75}
                    isAnimationActive={false}
                    dot={(p: { cx?: number; cy?: number; index?: number }) =>
                      p.index === n - 1 && p.cx !== undefined ? (
                        <circle key="end" cx={p.cx} cy={p.cy} r={3} fill={color(si)} />
                      ) : (
                        <g key={`d${p.index}`} />
                      )
                    }
                  />
                )
              )}
            </ComposedChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

const OTHER = "기타";

// 그룹별 구성 적층 막대(카테고리별 귀속자, 귀속자별 카테고리, 카드별 카테고리).
//   - 막대 길이 = 그룹 총액(최댓값 대비), 조각 = 구성 항목. 조각마다 호버/강조가 있어
//     Recharts 보다 직접 그린 HTML 이 정확하다(커스텀 구현 유지).
//   - 항목 색은 전 그룹 공통이고 합계 큰 순. 팔레트를 넘으면 돌려 쓰지 않고 상위 N-1 + "기타".
export function StackedBars({ data }: { data: store.CardCategory[] }) {
  const tc = useThemeColors();
  const [focus, setFocus] = useState<string | null>(null);
  const [tip, setTip] = useState<{ x: number; y: number; row: string; seg: string; amt: number; share: number } | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);
  if (data.length === 0) return <ChartEmpty />;

  const palette = tc.palette;
  const totals: Record<string, number> = {};
  for (const d of data) for (const c of d.categories) totals[c.name] = (totals[c.name] || 0) + c.amount;
  const sorted = Object.keys(totals).sort((a, b) => totals[b] - totals[a]);
  const fold = sorted.length > palette.length;
  const keep = fold ? sorted.slice(0, palette.length - 1) : sorted;
  const keepSet = new Set(keep);
  const color = (name: string) => (keepSet.has(name) ? palette[keep.indexOf(name)] : tc.axis);
  const series = fold
    ? [...keep, OTHER].map((nm) => ({
        name: nm,
        total: nm === OTHER ? sorted.slice(keep.length).reduce((a, k) => a + totals[k], 0) : totals[nm],
      }))
    : keep.map((nm) => ({ name: nm, total: totals[nm] }));

  const rows = data.map((d) => {
    const segs: { name: string; amount: number }[] = [];
    let other = 0;
    for (const c of d.categories) {
      if (keepSet.has(c.name)) segs.push({ name: c.name, amount: c.amount });
      else other += c.amount;
    }
    if (other > 0) segs.push({ name: OTHER, amount: other });
    return { ...d, segs };
  });
  const max = Math.max(1, ...data.map((d) => d.total));

  // 색 위에 올릴 글자색: 바탕 밝기로 고른다 (라이트 테마에서 white 토큰은 어두운 색)
  const inkOn = (hex: string) => {
    const h = hex.replace("#", "");
    if (h.length !== 6) return cv("paper");
    const [r, g, b] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255);
    const light = 0.2126 * r + 0.7152 * g + 0.0722 * b > 0.6;
    if (!light) return cv("paper");
    return tc.mode === "dark" ? cv("ink-950") : cv("white");
  };

  const move = (e: React.MouseEvent, row: string, seg: string, amt: number, total: number) => {
    const r = boxRef.current?.getBoundingClientRect();
    if (!r) return;
    setTip({ x: e.clientX - r.left, y: e.clientY - r.top, row, seg, amt, share: total ? amt / total : 0 });
  };
  const boxW = boxRef.current?.clientWidth ?? 0;

  return (
    <div ref={boxRef} className="relative" onMouseLeave={() => { setFocus(null); setTip(null); }}>
      <div className="flex flex-wrap gap-x-3 gap-y-1.5 mb-3">
        {series.map((s) => (
          <span
            key={s.name}
            className={`inline-flex items-center gap-1.5 text-[11px] text-zinc-300 transition-opacity cursor-default ${
              focus && focus !== s.name ? "opacity-40" : ""
            }`}
            onMouseEnter={() => setFocus(s.name)}
            onMouseLeave={() => setFocus(null)}
          >
            <LegendDot color={color(s.name)} />
            {s.name}
            <em className="not-italic num text-zinc-500">{compact(s.total)}</em>
          </span>
        ))}
      </div>
      <div className="space-y-2">
        {rows.map((d) => (
          <div key={d.card} className="grid grid-cols-[minmax(80px,120px)_1fr_auto] items-center gap-3">
            <span className="flex items-center gap-1.5 text-xs text-zinc-300 truncate" title={d.card}>
              {d.color && <i className="w-2 h-2 rounded-full shrink-0" style={{ background: d.color }} />}
              <span className="truncate">{d.card}</span>
            </span>
            <div className="min-w-0">
              <div className="flex gap-[2px] h-7" style={{ width: `${(d.total / max) * 100}%` }}>
                {d.segs.map((c) => {
                  const bg = color(c.name);
                  const wide = c.amount / max >= 0.09;
                  return (
                    <div
                      key={c.name}
                      className={`min-w-[2px] rounded-[4px] flex items-center justify-center text-[10px] font-medium overflow-hidden whitespace-nowrap transition-opacity ${
                        focus && focus !== c.name ? "opacity-30" : ""
                      }`}
                      style={{ flexGrow: c.amount, flexBasis: 0, background: bg, color: inkOn(bg) }}
                      onMouseEnter={(e) => { setFocus(c.name); move(e, d.card, c.name, c.amount, d.total); }}
                      onMouseMove={(e) => move(e, d.card, c.name, c.amount, d.total)}
                    >
                      {wide && <span>{c.name}</span>}
                    </div>
                  );
                })}
              </div>
            </div>
            <span className="num text-xs text-white text-right">{won(d.total)}</span>
          </div>
        ))}
      </div>
      {tip && (
        <div
          className="absolute z-10"
          style={{ left: tip.x, top: tip.y + 16, transform: tip.x > boxW * 0.62 ? "translateX(calc(-100% - 12px))" : "translateX(12px)" }}
        >
          <ChartTip
            head={tip.row}
            rows={[{ color: color(tip.seg), value: won(tip.amt), name: `${tip.seg} · ${Math.round(tip.share * 100)}%` }]}
          />
        </div>
      )}
    </div>
  );
}

// 카드 실적기간 누적 진행 스파크라인: 누적선(실선) + 목표선(가로) + 이상 페이스(대각선).
// 누적선이 대각선 위에 있으면 목표 달성 페이스. 작은 고정 SVG 라 직접 그린다.
export function PaceSparkline({ pace }: { pace: store.CardPace }) {
  const tc = useThemeColors();
  const W = 300;
  const H = 96;
  const padX = 6;
  const padY = 12;
  const days = Math.max(1, pace.days);
  const max = Math.max(1, pace.target, pace.projected, ...pace.cumulative);
  const x = (i: number) => padX + ((W - padX * 2) * i) / Math.max(1, days - 1);
  const y = (v: number) => H - padY - ((H - padY * 2) * v) / max;
  const c = pace.card.color || autoColor(pace.card.name);

  const cumPath = pace.cumulative.map((v, i) => `${i === 0 ? "M" : "L"} ${x(i)} ${y(v)}`).join(" ");
  const last = pace.cumulative.length - 1;

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-auto block">
      {/* 목표선 */}
      <line x1={padX} x2={W - padX} y1={y(pace.target)} y2={y(pace.target)} stroke={tc.restricted} strokeWidth={1} strokeDasharray="4 3" />
      <text x={W - padX} y={y(pace.target) - 3} fontSize={8} fill={tc.restricted} textAnchor="end" fontFamily="var(--font-mono)">목표</text>
      {/* 이상 페이스 대각선 (기간말에 목표 도달) */}
      <line x1={x(0)} x2={x(days - 1)} y1={y(0)} y2={y(pace.target)} stroke={tc.axis} strokeWidth={1} strokeDasharray="2 3" opacity={0.6} />
      {/* 실제 누적 */}
      {last >= 0 && (
        <>
          <path d={cumPath} fill="none" stroke={c} strokeWidth={2.5} strokeLinejoin="round" strokeLinecap="round" />
          <circle cx={x(last)} cy={y(pace.cumulative[last])} r={3} fill={c} />
        </>
      )}
    </svg>
  );
}

// 전월 대비 증감 막대. 가운데 0선을 두고 늘어난 쪽은 오른쪽(rose), 줄어든 쪽은 왼쪽(emerald)으로 뻗는다.
// 금액순이 아니라 증감 큰 순.
export function DeltaBars({
  rows,
  onPick,
}: {
  rows: store.CategoryDelta[];
  onPick?: (name: string) => void;
}) {
  if (rows.length === 0) return <ChartEmpty>비교할 지출이 없습니다.</ChartEmpty>;
  const max = Math.max(1, ...rows.map((r) => Math.abs(r.delta)));

  return (
    <ul className="space-y-1">
      {rows.map((r) => {
        const up = r.delta > 0;
        const w = (Math.abs(r.delta) / max) * 50; // 한쪽 최대 50%
        return (
          <li
            key={r.name}
            className={`grid grid-cols-[minmax(70px,110px)_1fr_auto] md:grid-cols-[minmax(70px,110px)_1fr_100px_52px_130px] items-center gap-3 px-2 py-1.5 rounded-lg text-xs ${
              onPick ? "cursor-pointer hover:bg-white/[0.05] transition-colors" : ""
            }`}
            onClick={onPick ? () => onPick(r.name) : undefined}
          >
            <span className="truncate text-zinc-300" title={r.name}>{r.name}</span>
            <span className="relative h-2.5 rounded-full bg-white/[0.04]">
              <i className="absolute left-1/2 top-[-3px] bottom-[-3px] w-px bg-white/20" />
              <i
                className={`absolute top-0 bottom-0 ${up ? "rounded-r-full bg-rose-400" : "rounded-l-full bg-emerald-400"}`}
                style={up ? { left: "50%", width: `${w}%` } : { right: "50%", width: `${w}%` }}
              />
            </span>
            <span className={`num text-right ${up ? "text-rose-400" : r.delta < 0 ? "text-emerald-400" : "text-zinc-500"}`}>
              {r.delta > 0 ? "+" : ""}
              {won(r.delta)}
            </span>
            <span className="num text-right text-zinc-500 hidden md:block">
              {r.isNew ? "신규" : r.gone ? "없어짐" : r.prev === 0 ? "—" : `${r.pct > 0 ? "+" : ""}${r.pct}%`}
            </span>
            <span className="num text-right text-zinc-500 hidden md:block">
              {compact(r.prev)}
              {" → "}
              <strong className="font-semibold text-white">{compact(r.current)}</strong>
            </span>
          </li>
        );
      })}
    </ul>
  );
}

// 항목 하나의 월별 금액 추이. 드릴다운 모달 머리에 놓아 "이 달이 유난한가"를 바로 보게 한다.
// 고른 달은 강조하고, 값이 있는 다른 달을 누르면 그 달로 옮겨 간다.
export function MiniTrend({
  values,
  months,
  active,
  onPick,
}: {
  values: number[];
  months: string[];
  active: number; // 강조할 달의 인덱스
  onPick?: (i: number) => void;
}) {
  const tc = useThemeColors();
  const tick = useAxisTick();
  const gid = safeId(useId());
  const n = values.length;
  if (n === 0) return null;

  const avg = values.reduce((s, v) => s + v, 0) / n;
  const data = values.map((v, i) => ({ i, v, label: `${Number(months[i].slice(5))}월` }));
  const pickable = (i: number) => !!onPick && (values[i] ?? 0) > 0 && i !== active;

  return (
    <div className="h-44">
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart
          data={data}
          margin={{ top: 28, right: 24, left: 24, bottom: 0 }}
          onClick={(s) => {
            const i = Number(s?.activeTooltipIndex);
            if (Number.isInteger(i) && pickable(i)) onPick!(i);
          }}
          style={{ cursor: onPick ? "pointer" : undefined }}
        >
          <defs>
            <linearGradient id={`${gid}-m`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={tc.accent} stopOpacity={0.32} />
              <stop offset="100%" stopColor={tc.accent} stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="label"
            tick={tick}
            interval={0}
            axisLine={{ stroke: tc.grid }}
            tickLine={false}
            padding={{ left: 8, right: 8 }}
          />
          <YAxis hide domain={[0, (max: number) => Math.max(1, max)]} />
          {avg > 0 && (
            <ReferenceLine
              y={avg}
              stroke={tc.axis}
              strokeDasharray="3 4"
              label={{ value: `평균 ${compact(Math.round(avg))}`, position: "insideTopRight", fill: tc.axis, fontSize: 10, fontFamily: "var(--font-mono)" }}
            />
          )}
          <Tooltip
            cursor={{ stroke: tc.axis, strokeDasharray: "3 3" }}
            content={({ active: a, payload }) => {
              if (!a || !payload?.length) return null;
              const row = payload[0].payload as { i: number; v: number; label: string };
              return <ChartTip head={row.label} rows={[{ color: tc.accent, value: won(row.v) }]} />;
            }}
          />
          <Area
            type="monotone"
            dataKey="v"
            stroke={tc.accent}
            strokeWidth={1.75}
            fill={`url(#${gid}-m)`}
            isAnimationActive={false}
            dot={(p: { cx?: number; cy?: number; index?: number }) => {
              const on = p.index === active;
              return p.cx === undefined ? (
                <g key={`d${p.index}`} />
              ) : (
                <circle
                  key={`d${p.index}`}
                  cx={p.cx}
                  cy={p.cy}
                  r={on ? 5 : 3}
                  fill={on ? tc.accent : tc.surface}
                  stroke={tc.accent}
                  strokeWidth={on ? 2.5 : 1.5}
                />
              );
            }}
            activeDot={false}
          />
          {active >= 0 && active < n && (
            <ReferenceDot
              x={data[active].label}
              y={values[active]}
              r={0}
              ifOverflow="visible"
              label={{ value: won(values[active]), position: "top", fill: tc.text, fontSize: 12, fontWeight: 700, fontFamily: "var(--font-mono)" }}
            />
          )}
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}

// 항목 × 월 히트맵. 행 = 항목(총액 큰 순), 열 = 월, 칸 = 그 달 금액.
//   - 칸은 2px 간격의 둥근(4px) 타일. 진하기는 강조색(mint) 알파 단계, 제곱근 척도라
//     큰 칸 하나가 나머지를 모두 흐리게 만들지 않는다. 빈 칸은 bg-white/[0.03].
//   - 합계 열에는 비중 막대. 이름 열은 가로 스크롤 때 붙박이.
export function Heatmap({
  rows,
  months,
  onCell,
}: {
  rows: store.Series[];
  months: string[];
  // 값이 있는 칸을 누르면 그 달·그 항목의 거래를 열어 보게 한다
  onCell?: (name: string, ym: string) => void;
}) {
  const tc = useThemeColors();
  if (rows.length === 0) return <ChartEmpty />;
  const monthLabel = (ym: string) => `${Number(ym.slice(5))}월`;
  // 전체 최댓값 기준으로 진하기를 정한다(항목 간 크기 비교가 되도록)
  const max = Math.max(1, ...rows.flatMap((r) => r.values));
  const maxTotal = Math.max(1, ...rows.map((r) => r.total));
  const last = months.length - 1;
  const strongText = tc.mode === "dark" ? "text-ink-950" : "text-white";

  return (
    <div className="overflow-x-auto scrollbar-hide">
      <table className="border-separate [border-spacing:2px] w-full text-[11px]">
        <thead>
          <tr>
            <th className="sticky left-0 z-[1] text-left font-normal label-mono px-2 py-1 bg-ink-900/70 backdrop-blur-md rounded-[4px]">항목</th>
            {months.map((m, i) => (
              <th key={m} className={`font-normal label-mono px-1 py-1 min-w-[44px] ${i === last ? "!text-mint-400" : ""}`}>
                {monthLabel(m)}
              </th>
            ))}
            <th className="font-normal label-mono px-2 py-1 min-w-[96px] text-left">합계</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, ri) => (
            <tr key={r.name}>
              <th
                className="sticky left-0 z-[1] text-left font-normal px-2 py-1 bg-ink-900/70 backdrop-blur-md rounded-[4px] max-w-[140px]"
                title={r.name}
              >
                <span className="flex items-center gap-1.5">
                  <span className="num text-[10px] text-zinc-500 w-4 shrink-0">{ri + 1}</span>
                  <span className="truncate text-zinc-200">{r.name}</span>
                </span>
              </th>
              {r.values.map((v, i) => {
                const lv = v > 0 ? Math.sqrt(v / max) : 0;
                const clickable = !!onCell && v > 0;
                const peak = v === max && v > 0;
                return (
                  <td key={i} className="p-0">
                    <span
                      className={`num flex items-center justify-center h-8 rounded-[4px] ${
                        v > 0 ? (lv > 0.55 ? `${strongText} font-semibold` : "text-white") : "bg-white/[0.03]"
                      } ${peak ? "ring-1 ring-mint-300" : ""} ${
                        clickable ? "cursor-pointer hover:ring-1 hover:ring-white/40 transition-shadow" : ""
                      }`}
                      style={v > 0 ? { background: cv("mint-400", Number((0.1 + 0.8 * lv).toFixed(2))) } : undefined}
                      title={
                        v > 0
                          ? `${r.name} · ${monthLabel(months[i])} · ${won(v)}${clickable ? " — 눌러서 내역 보기" : ""}`
                          : `${r.name} · ${monthLabel(months[i])} · 지출 없음`
                      }
                      onClick={clickable ? () => onCell!(r.name, months[i]) : undefined}
                    >
                      {v > 0 ? compact(v) : ""}
                    </span>
                  </td>
                );
              })}
              <td className="px-2">
                <span className="flex items-center gap-2">
                  <span className="flex-1 h-1.5 rounded-full bg-white/[0.04] overflow-hidden min-w-[30px]">
                    <i className="block h-full rounded-full bg-mint-400/70" style={{ width: `${(r.total / maxTotal) * 100}%` }} />
                  </span>
                  <strong className="num font-semibold text-white">{compact(r.total)}</strong>
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
