import { useId, useLayoutEffect, useRef, useState } from "react";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { autoColor } from "./PmChip";

const C_INCOME = "#22c55e";
const C_EXPENSE = "#f43f5e";
const C_TRANSFER = "#a855f7";
// 격자/축선은 라이트·다크 양쪽에서 보이도록 중립 회색. 텍스트 색은 CSS(.chart text)가 테마에 맞게 덮어쓴다.
const C_GRID = "rgba(140,145,170,0.22)";
const C_TEXT = "rgba(140,145,170,0.7)";
const C_TARGET = "#d08a2c";

export const PALETTE = [
  "#5b6cff",
  "#f43f5e",
  "#22c55e",
  "#eab308",
  "#a855f7",
  "#06b6d4",
  "#f97316",
  "#64748b",
];

export const compact = (n: number) =>
  n >= 100000000
    ? `${(n / 100000000).toFixed(1)}억`
    : n >= 10000
    ? `${Math.round(n / 10000)}만`
    : String(n);

// useChartWidth 는 차트를 그릴 실제 폭(px)을 잰다.
// 고정 viewBox 를 컨테이너에 맞춰 늘리면 넓은 카드에선 축 글자가 커지고 반쪽 카드에선
// 깨알처럼 작아진다. 폭을 재서 1단위 = 1px 로 그리면 어느 카드에서든 글자 크기가 같다.
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

// 최근 n개월 수입/지출 막대 추이
export function TrendChart({ points }: { points: store.MonthPoint[] }) {
  const [ref, W] = useChartWidth();
  const H = 240;
  const padX = 44;
  const padY = 26;
  const max = Math.max(1, ...points.map((p) => Math.max(p.income, p.expense)));
  const slot = (W - padX * 2) / Math.max(1, points.length);
  const bw = Math.min(26, slot / 3.2);
  const scaleY = (v: number) => ((H - padY * 2) * v) / max;

  return (
    <div ref={ref}>
      <div className="legend">
        <span><i style={{ background: C_INCOME }} /> 수입</span>
        <span><i style={{ background: C_EXPENSE }} /> 지출</span>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} className="chart">
        {[0.25, 0.5, 0.75, 1].map((f) => (
          <g key={f}>
            <line
              x1={padX} x2={W - padX}
              y1={H - padY - (H - padY * 2) * f}
              y2={H - padY - (H - padY * 2) * f}
              stroke={C_GRID}
            />
            <text x={4} y={H - padY - (H - padY * 2) * f + 4} fontSize={11} fill={C_TEXT}>
              {compact(max * f)}
            </text>
          </g>
        ))}
        <line x1={padX} x2={W - padX} y1={H - padY} y2={H - padY} stroke={C_GRID} />
        {points.map((p, i) => {
          const cx = padX + i * slot + slot / 2;
          const hi = scaleY(p.income);
          const he = scaleY(p.expense);
          return (
            <g key={p.month}>
              <rect x={cx - bw - 2} y={H - padY - hi} width={bw} height={hi} rx={Math.min(6, bw / 3)} fill={C_INCOME} opacity={0.9} />
              <rect x={cx + 2} y={H - padY - he} width={bw} height={he} rx={Math.min(6, bw / 3)} fill={C_EXPENSE} opacity={0.9} />
              <text x={cx} y={H - 7} fontSize={12} fill={C_TEXT} textAnchor="middle">
                {Number(p.month.slice(5))}월
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}

// 이번 달 일별 지출 영역 그래프 + 누적선
export function DailyChart({ points }: { points: store.DayPoint[] }) {
  const [ref, W] = useChartWidth();
  const H = 200;
  const padX = 36;
  const padY = 22;
  const n = points.length || 1;
  const max = Math.max(1, ...points.map((p) => p.amount));
  let acc = 0;
  const cum = points.map((p) => (acc += p.amount));
  const cumMax = Math.max(1, acc);

  const x = (i: number) => padX + ((W - padX * 2) * i) / Math.max(1, n - 1);
  const y = (v: number) => H - padY - ((H - padY * 2) * v) / max;
  const yCum = (v: number) => H - padY - ((H - padY * 2) * v) / cumMax;

  const area =
    `M ${x(0)} ${H - padY} ` +
    points.map((p, i) => `L ${x(i)} ${y(p.amount)}`).join(" ") +
    ` L ${x(n - 1)} ${H - padY} Z`;
  const cumLine = cum.map((v, i) => `${i === 0 ? "M" : "L"} ${x(i)} ${yCum(v)}`).join(" ");

  return (
    <div ref={ref}>
      <div className="legend">
        <span><i style={{ background: C_EXPENSE }} /> 일별 지출</span>
        <span><i style={{ background: C_TRANSFER }} /> 누적 (이번 달 총 {won(acc)})</span>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} className="chart">
        <defs>
          <linearGradient id="dailyFill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={C_EXPENSE} stopOpacity={0.55} />
            <stop offset="100%" stopColor={C_EXPENSE} stopOpacity={0.05} />
          </linearGradient>
        </defs>
        {[0.5, 1].map((f) => (
          <line
            key={f}
            x1={padX} x2={W - padX}
            y1={H - padY - (H - padY * 2) * f}
            y2={H - padY - (H - padY * 2) * f}
            stroke={C_GRID}
          />
        ))}
        <line x1={padX} x2={W - padX} y1={H - padY} y2={H - padY} stroke={C_GRID} />
        <path d={area} fill="url(#dailyFill)" stroke={C_EXPENSE} strokeWidth={1.5} />
        <path d={cumLine} fill="none" stroke={C_TRANSFER} strokeWidth={2} strokeDasharray="5 4" />
        {points.map((p, i) =>
          i % 5 === 0 || i === n - 1 ? (
            <text key={i} x={x(i)} y={H - 6} fontSize={11} fill={C_TEXT} textAnchor="middle">
              {p.day}
            </text>
          ) : null
        )}
        <text x={4} y={padY + 4} fontSize={11} fill={C_TEXT}>{compact(max)}</text>
      </svg>
    </div>
  );
}

// 도넛 차트: 상위 7개 + 기타
export function Donut({ rows, centerLabel }: { rows: store.NamedAmount[]; centerLabel: string }) {
  const total = rows.reduce((s, r) => s + r.amount, 0);
  if (total === 0) return <p className="muted">데이터 없음</p>;
  const top = rows.slice(0, 7);
  const rest = rows.slice(7).reduce((s, r) => s + r.amount, 0);
  const segs = rest > 0 ? [...top, { name: "기타", kind: "expense", amount: rest }] : top;

  const R = 62;
  const CIRC = 2 * Math.PI * R;
  let offset = 0;

  return (
    <div className="donut-wrap">
      <svg viewBox="0 0 170 170" className="donut">
        {segs.map((s, i) => {
          const frac = s.amount / total;
          const el = (
            <circle
              key={i}
              cx={85} cy={85} r={R}
              fill="none"
              stroke={PALETTE[i % PALETTE.length]}
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
        <text x={85} y={80} textAnchor="middle" fontSize={11} fill={C_TEXT}>{centerLabel}</text>
        <text x={85} y={98} textAnchor="middle" fontSize={13} fontWeight={700} className="donut-total">
          {compact(total)}원
        </text>
      </svg>
      <ul className="donut-legend">
        {segs.map((s, i) => (
          <li key={i}>
            <i style={{ background: PALETTE[i % PALETTE.length] }} />
            <span className="dl-name">{s.name}</span>
            <span className="dl-pct">{Math.round((s.amount / total) * 100)}%</span>
            <span className="dl-amt">{won(s.amount)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export type LineSeries = { name: string; color?: string; values: number[] };

// monotonePath 는 점들을 지나는 부드러운 곡선(단조 3차 보간, Fritsch–Carlson).
// 일반 곡선 보간(Catmull-Rom 등)은 계단처럼 꺾이는 곳에서 위아래로 출렁이는데,
// 누적 지출이 잠깐 줄어드는 것처럼 보이면 거짓 정보다. 단조 보간은 두 점 사이에서
// 값을 넘어서지 않아 꺾은선과 같은 사실을 유지하면서 모양만 부드러워진다.
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

// niceMax 는 축 최댓값을 보기 좋은 수(1·2·2.5·5 ×10ⁿ)로 올린다. 눈금이 17만·34만 같은
// 어정쩡한 수로 찍히면 읽기 어렵다.
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
  const [ref, W] = useChartWidth();
  const [hi, setHi] = useState<number | null>(null);
  const gid = useId();
  const H = height;
  const padL = 46;
  const padR = 16;
  const padT = 14;
  const padB = 26;
  const n = xLabels.length;

  const plotted = series.map((s) => {
    if (!cumulative) return s.values;
    let acc = 0;
    return s.values.map((v) => (acc += v));
  });
  const max = niceMax(Math.max(1, ...plotted.flatMap((vals) => vals)));
  const totals = series.map((s) => s.values.reduce((a, b) => a + b, 0));

  const step = (W - padL - padR) / Math.max(1, n - 1);
  const x = (i: number) => padL + step * i;
  const y = (v: number) => H - padB - ((H - padT - padB) * v) / max;
  const color = (i: number) => series[i].color || PALETTE[i % PALETTE.length];

  const hasData = totals.some((t) => t > 0);
  const labelEvery = n <= 12 ? 1 : n <= 20 ? 2 : 5;
  // 시리즈가 하나면 아래에 옅은 면을 깔아 모양을 살린다(여럿이면 겹쳐서 지저분해진다)
  const single = series.length === 1;

  const onMove = (e: React.PointerEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    const px = ((e.clientX - r.left) / r.width) * W;
    setHi(Math.max(0, Math.min(n - 1, Math.round((px - padL) / step))));
  };

  // 툴팁: 값이 큰 순으로, 값이 앞에 굵게 오고 이름은 뒤에 흐리게
  const tipRows =
    hi === null
      ? []
      : series
          .map((s, si) => ({ name: s.name, color: color(si), v: plotted[si][hi] ?? 0 }))
          .sort((a, b) => b.v - a.v)
          .slice(0, 8);
  const tipLeft = hi === null ? 0 : x(hi);
  const tipFlip = tipLeft > W * 0.62;

  return (
    <div ref={ref} className="lc-wrap">
      {showLegend && (
        <div className="legend wrap">
          {series.map((s, i) => (
            <span key={i}>
              <i style={{ background: color(i) }} /> {s.name}
              <em className="lc-total">{valueFmt(totals[i])}</em>
            </span>
          ))}
        </div>
      )}
      {!hasData ? (
        <p className="muted">데이터 없음</p>
      ) : (
        <div className="lc-plot">
          <svg
            viewBox={`0 0 ${W} ${H}`}
            className="chart"
            onPointerMove={onMove}
            onPointerLeave={() => setHi(null)}
          >
            {single && (
              <defs>
                <linearGradient id={`lc-${gid}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={color(0)} stopOpacity={0.28} />
                  <stop offset="100%" stopColor={color(0)} stopOpacity={0} />
                </linearGradient>
              </defs>
            )}
            {[0, 0.25, 0.5, 0.75, 1].map((f) => (
              <g key={f}>
                <line
                  x1={padL} x2={W - padR}
                  y1={y(max * f)} y2={y(max * f)}
                  stroke={C_GRID}
                  strokeDasharray={f === 0 ? undefined : "2 4"}
                />
                <text x={padL - 8} y={y(max * f) + 4} fontSize={11} fill={C_TEXT} textAnchor="end">
                  {compact(max * f)}
                </text>
              </g>
            ))}
            {marker && marker.x >= 1 && marker.x <= n && (
              <g>
                <line
                  x1={x(marker.x - 1)} x2={x(marker.x - 1)}
                  y1={padT} y2={H - padB}
                  stroke={C_TEXT} strokeWidth={1} strokeDasharray="3 3"
                />
                {marker.label && (
                  <text x={x(marker.x - 1)} y={padT - 3} fontSize={11} fill={C_TEXT} textAnchor="middle">
                    {marker.label}
                  </text>
                )}
              </g>
            )}
            {plotted.map((vals, si) => {
              const pts = vals.map((v, i) => [x(i), y(v)] as [number, number]);
              const d = monotonePath(pts);
              const last = pts[pts.length - 1];
              return (
                <g key={si}>
                  {single && last && (
                    <path d={`${d} L ${last[0]} ${H - padB} L ${pts[0][0]} ${H - padB} Z`} fill={`url(#lc-${gid})`} />
                  )}
                  <path
                    d={d}
                    fill="none"
                    stroke={color(si)}
                    strokeWidth={2}
                    strokeLinejoin="round"
                    strokeLinecap="round"
                    opacity={hi === null ? 1 : 0.9}
                  />
                  {/* 끝점만 찍는다 — 모든 점에 동그라미를 달면 선이 구슬 목걸이가 된다 */}
                  {last && <circle cx={last[0]} cy={last[1]} r={3} fill={color(si)} />}
                </g>
              );
            })}
            {xLabels.map((lab, i) =>
              i % labelEvery === 0 || i === n - 1 ? (
                <text key={i} x={x(i)} y={H - 7} fontSize={11} fill={C_TEXT} textAnchor="middle">
                  {lab}
                </text>
              ) : null
            )}
            {hi !== null && (
              <g pointerEvents="none">
                <line x1={x(hi)} x2={x(hi)} y1={padT} y2={H - padB} className="lc-cross" />
                {plotted.map((vals, si) => (
                  <circle
                    key={si}
                    cx={x(hi)} cy={y(vals[hi] ?? 0)} r={4.5}
                    fill={color(si)}
                    className="lc-hot"
                  />
                ))}
              </g>
            )}
          </svg>
          {hi !== null && (
            <div
              className={`chart-tip${tipFlip ? " flip" : ""}`}
              style={{ left: tipLeft }}
            >
              <div className="ct-head">{xLabels[hi]}</div>
              {tipRows.map((r) => (
                <div className="ct-row" key={r.name}>
                  <i style={{ background: r.color }} />
                  <strong>{valueFmt(r.v)}</strong>
                  <span>{r.name}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// inkOn 은 색 위에 올릴 글자색(밝은 바탕이면 진한 남색, 어두운 바탕이면 흰색).
// 노랑 위의 흰 글자처럼 대비가 무너지지 않게 바탕 밝기로 고른다.
function inkOn(hex: string): string {
  const h = hex.replace("#", "");
  if (h.length !== 6) return "#fff";
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255);
  const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b;
  return lum > 0.6 ? "#1c2140" : "#fff";
}

const OTHER = "기타";
const OTHER_COLOR = "#94a3b8";

// 그룹별 구성 적층 막대(카테고리별 귀속자, 귀속자별 카테고리, 카드별 카테고리).
//   - 막대 길이 = 그룹 총액(최댓값 대비), 조각 = 구성 항목.
//   - 항목 색은 전 그룹 공통이고 합계 큰 순으로 정한다. 팔레트(8색)를 넘으면 색을 돌려 쓰지
//     않고 상위 7개 + "기타"(회색)로 묶는다 — 같은 색이 두 항목을 뜻하면 읽을 수 없다.
//   - 조각 사이 2px 틈으로 경계를 또렷하게, 폭이 넉넉한 조각에는 이름을 직접 쓴다.
//   - 조각(또는 범례)에 마우스를 올리면 같은 항목이 모든 줄에서 함께 밝아지고 툴팁이 뜬다.
export function StackedBars({ data }: { data: store.CardCategory[] }) {
  const [focus, setFocus] = useState<string | null>(null);
  const [tip, setTip] = useState<{ x: number; y: number; row: string; seg: string; amt: number; share: number } | null>(
    null
  );
  const boxRef = useRef<HTMLDivElement>(null);
  if (data.length === 0) return <p className="muted">데이터 없음</p>;

  const totals: Record<string, number> = {};
  for (const d of data) for (const c of d.categories) totals[c.name] = (totals[c.name] || 0) + c.amount;
  const sorted = Object.keys(totals).sort((a, b) => totals[b] - totals[a]);
  const fold = sorted.length > PALETTE.length;
  const keep = fold ? sorted.slice(0, PALETTE.length - 1) : sorted;
  const keepSet = new Set(keep);
  const color = (name: string) => (keepSet.has(name) ? PALETTE[keep.indexOf(name)] : OTHER_COLOR);
  const series = fold
    ? [...keep, OTHER].map((n) => ({
        name: n,
        total: n === OTHER ? sorted.slice(keep.length).reduce((a, k) => a + totals[k], 0) : totals[n],
      }))
    : keep.map((n) => ({ name: n, total: totals[n] }));

  // 줄마다 "기타"로 묶은 조각 목록
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

  const move = (e: React.MouseEvent, row: string, seg: string, amt: number, total: number) => {
    const r = boxRef.current?.getBoundingClientRect();
    if (!r) return;
    setTip({ x: e.clientX - r.left, y: e.clientY - r.top, row, seg, amt, share: total ? amt / total : 0 });
  };

  return (
    <div className="sb" ref={boxRef} onMouseLeave={() => { setFocus(null); setTip(null); }}>
      <div className="sb-legend">
        {series.map((s) => (
          <span
            key={s.name}
            className={`sb-key${focus === s.name ? " on" : focus ? " dim" : ""}`}
            onMouseEnter={() => setFocus(s.name)}
            onMouseLeave={() => setFocus(null)}
          >
            <i style={{ background: s.name === OTHER ? OTHER_COLOR : color(s.name) }} />
            {s.name}
            <em>{compact(s.total)}</em>
          </span>
        ))}
      </div>
      <div className="sb-rows">
        {rows.map((d) => (
          <div className="sb-row" key={d.card}>
            <span className="sb-label" title={d.card}>
              {d.color && <i style={{ background: d.color }} />}
              {d.card}
            </span>
            <div className="sb-cell">
              <div className="sb-track" style={{ width: `${(d.total / max) * 100}%` }}>
                {d.segs.map((c) => {
                  const bg = c.name === OTHER ? OTHER_COLOR : color(c.name);
                  const wide = c.amount / max >= 0.09;
                  return (
                    <div
                      key={c.name}
                      className={`sb-seg${focus && focus !== c.name ? " dim" : ""}${focus === c.name ? " on" : ""}`}
                      style={{ flexGrow: c.amount, background: bg, color: inkOn(bg) }}
                      onMouseEnter={(e) => { setFocus(c.name); move(e, d.card, c.name, c.amount, d.total); }}
                      onMouseMove={(e) => move(e, d.card, c.name, c.amount, d.total)}
                    >
                      {wide && <span>{c.name}</span>}
                    </div>
                  );
                })}
              </div>
            </div>
            <span className="sb-amt">{won(d.total)}</span>
          </div>
        ))}
      </div>
      {tip && (
        <div
          className={`chart-tip sb-tip${tip.x > (boxRef.current?.clientWidth ?? 0) * 0.62 ? " flip" : ""}`}
          style={{ left: tip.x, top: tip.y + 16 }}
        >
          <div className="ct-head">{tip.row}</div>
          <div className="ct-row">
            <i style={{ background: tip.seg === OTHER ? OTHER_COLOR : color(tip.seg) }} />
            <strong>{won(tip.amt)}</strong>
            <span>{tip.seg} · {Math.round(tip.share * 100)}%</span>
          </div>
        </div>
      )}
    </div>
  );
}

// 카드 실적기간 누적 진행 스파크라인: 누적선(실선) + 목표선(가로) + 이상 페이스(대각선).
// 누적선이 대각선 위에 있으면 목표 달성 페이스.
export function PaceSparkline({ pace }: { pace: store.CardPace }) {
  const W = 300;
  const H = 96;
  const padX = 6;
  const padY = 12;
  const days = Math.max(1, pace.days);
  const max = Math.max(1, pace.target, pace.projected, ...pace.cumulative);
  const x = (i: number) => padX + ((W - padX * 2) * i) / Math.max(1, days - 1);
  const y = (v: number) => H - padY - ((H - padY * 2) * v) / max;
  const c = pace.card.color || autoColor(pace.card.name);

  const cumPath = pace.cumulative
    .map((v, i) => `${i === 0 ? "M" : "L"} ${x(i)} ${y(v)}`)
    .join(" ");
  const last = pace.cumulative.length - 1;

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="chart pace">
      {/* 목표선 */}
      <line x1={padX} x2={W - padX} y1={y(pace.target)} y2={y(pace.target)} stroke={C_TARGET} strokeWidth={1} strokeDasharray="4 3" />
      <text x={W - padX} y={y(pace.target) - 3} fontSize={8} fill={C_TARGET} textAnchor="end">목표</text>
      {/* 이상 페이스 대각선 (기간말에 목표 도달) */}
      <line x1={x(0)} x2={x(days - 1)} y1={y(0)} y2={y(pace.target)} stroke={C_TEXT} strokeWidth={1} strokeDasharray="2 3" opacity={0.6} />
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

// 전월 대비 증감 막대. 가운데 0선을 두고 늘어난 쪽은 오른쪽(빨강), 줄어든 쪽은 왼쪽(초록)으로
// 뻗는다. 금액순이 아니라 증감 큰 순으로 와야 "무엇이 총액을 끌어올렸나"가 위에서부터 읽힌다.
export function DeltaBars({
  rows,
  onPick,
}: {
  rows: store.CategoryDelta[];
  onPick?: (name: string) => void;
}) {
  if (rows.length === 0) return <p className="muted">비교할 지출이 없습니다.</p>;
  const max = Math.max(1, ...rows.map((r) => Math.abs(r.delta)));

  return (
    <ul className="delta-bars">
      {rows.map((r) => {
        const up = r.delta > 0;
        const w = (Math.abs(r.delta) / max) * 50; // 한쪽 최대 50%
        return (
          <li
            key={r.name}
            className={onPick ? "pickable" : undefined}
            onClick={onPick ? () => onPick(r.name) : undefined}
          >
            <span className="db-name" title={r.name}>{r.name}</span>
            <span className="db-track">
              <i className="db-zero" />
              <i
                className={`db-fill ${up ? "up" : "down"}`}
                style={up ? { left: "50%", width: `${w}%` } : { right: "50%", width: `${w}%` }}
              />
            </span>
            <span className={`db-delta ${up ? "bad" : r.delta < 0 ? "good" : "flat"}`}>
              {r.delta > 0 ? "+" : ""}
              {won(r.delta)}
            </span>
            <span className="db-pct muted">
              {r.isNew ? "신규" : r.gone ? "없어짐" : r.prev === 0 ? "—" : `${r.pct > 0 ? "+" : ""}${r.pct}%`}
            </span>
            <span className="db-amt">
              <span className="muted">{compact(r.prev)}</span>
              {" → "}
              <strong>{compact(r.current)}</strong>
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
  const gid = useId(); // 그라디언트 id 충돌 방지
  const W = 1000;
  const H = 170;
  const padL = 10;
  const padR = 10;
  const padT = 30; // 강조 지점 위 금액 라벨 자리
  const padB = 24; // 월 라벨 자리
  const n = values.length;
  if (n === 0) return null;

  const max = Math.max(1, ...values);
  const avg = values.reduce((s, v) => s + v, 0) / n;
  const x = (i: number) => (n === 1 ? W / 2 : padL + ((W - padL - padR) * i) / (n - 1));
  const y = (v: number) => H - padB - ((H - padT - padB) * v) / max;

  const line = values.map((v, i) => `${i === 0 ? "M" : "L"} ${x(i)} ${y(v)}`).join(" ");
  const area = `${line} L ${x(n - 1)} ${H - padB} L ${x(0)} ${H - padB} Z`;
  const ay = y(avg);

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="chart mini-trend">
      <defs>
        <linearGradient id={`mt-${gid}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#5b6cff" stopOpacity={0.36} />
          <stop offset="100%" stopColor="#5b6cff" stopOpacity={0.02} />
        </linearGradient>
      </defs>

      {/* 바닥선과 평균선 — 평균 위아래로 어느 달이 튀는지 한눈에 */}
      <line x1={padL} x2={W - padR} y1={H - padB} y2={H - padB} stroke={C_GRID} strokeWidth={1} />
      {avg > 0 && (
        <>
          <line x1={padL} x2={W - padR} y1={ay} y2={ay} stroke={C_TEXT} strokeWidth={1} strokeDasharray="3 4" />
          <text x={W - padR} y={ay - 4} fontSize={9} fill={C_TEXT} textAnchor="end">
            평균 {compact(Math.round(avg))}
          </text>
        </>
      )}

      <path d={area} fill={`url(#mt-${gid})`} />
      <path d={line} fill="none" stroke="#5b6cff" strokeWidth={2.5} strokeLinejoin="round" strokeLinecap="round" />

      {values.map((v, i) => {
        const on = i === active;
        const pickable = !!onPick && v > 0 && !on;
        // 양 끝 라벨은 가운데 정렬하면 그림 밖으로 밀려 잘린다
        const anchor = i === 0 ? "start" : i === n - 1 ? "end" : "middle";
        return (
          <g
            key={i}
            className={pickable ? "mt-pick" : undefined}
            onClick={pickable ? () => onPick!(i) : undefined}
          >
            {/* 넓은 투명 히트 영역 — 점이 작아도 누르기 쉽게 */}
            <rect
              x={x(i) - (W - padL - padR) / (2 * Math.max(1, n - 1))}
              y={0}
              width={(W - padL - padR) / Math.max(1, n - 1)}
              height={H}
              fill="transparent"
            >
              <title>{`${Number(months[i].slice(5))}월 · ${won(v)}`}</title>
            </rect>
            {on && <line x1={x(i)} x2={x(i)} y1={y(v)} y2={H - padB} stroke="#5b6cff" strokeWidth={1} opacity={0.45} />}
            <circle
              cx={x(i)}
              cy={y(v)}
              r={on ? 5 : 3}
              fill={on ? "#5b6cff" : "var(--glass-strong)"}
              stroke="#5b6cff"
              strokeWidth={on ? 2.5 : 1.5}
            />
            {on && (
              <text className="mt-on" x={x(i)} y={y(v) - 12} fontSize={12} fontWeight={700} textAnchor={anchor}>
                {won(v)}
              </text>
            )}
            <text
              x={x(i)}
              y={H - 7}
              fontSize={10}
              className={on ? "mt-on" : undefined}
              fontWeight={on ? 700 : 400}
              textAnchor={anchor}
            >
              {Number(months[i].slice(5))}월
            </text>
          </g>
        );
      })}
    </svg>
  );
}

// 항목 × 월 히트맵. 행 = 항목(총액 큰 순), 열 = 월, 칸 = 그 달 금액.
//   - 칸은 간격을 띄운 둥근 유리 타일. 진하기는 제곱근 척도라 큰 칸 하나가 나머지를 모두
//     흐리게 만들지 않는다. 진한 타일은 글자를 흰색으로 바꿔 대비를 지킨다.
//   - 빈 칸도 옅은 타일로 남겨 격자가 읽히게 한다.
//   - 합계 열에는 비중 막대를 붙여 어느 항목이 큰지 숫자를 읽지 않아도 보이게 한다.
//   - 이름 열은 가로 스크롤 때 붙박이로 남되, 불투명 판 대신 서리 유리라 밑으로 지나가는
//     칸이 흐리게 비친다.
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
  if (rows.length === 0) return <p className="muted">데이터 없음</p>;
  const monthLabel = (ym: string) => `${Number(ym.slice(5))}월`;
  // 전체 최댓값 기준으로 진하기를 정한다(항목 간 크기 비교가 되도록)
  const max = Math.max(1, ...rows.flatMap((r) => r.values));
  const maxTotal = Math.max(1, ...rows.map((r) => r.total));
  const last = months.length - 1;

  return (
    <div className="heatmap-wrap">
      <table className="heatmap">
        <thead>
          <tr>
            <th className="hm-corner">항목</th>
            {months.map((m, i) => (
              <th key={m} className={i === last ? "hm-now" : undefined}>
                <span>{monthLabel(m)}</span>
              </th>
            ))}
            <th className="hm-sum-h">합계</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, ri) => (
            <tr key={r.name}>
              <th className="hm-name" title={r.name}>
                <span className="hm-rank">{ri + 1}</span>
                <span className="hm-label">{r.name}</span>
              </th>
              {r.values.map((v, i) => {
                const lv = v > 0 ? Math.sqrt(v / max) : 0;
                const clickable = !!onCell && v > 0;
                const cls = [
                  "hm-cell",
                  v > 0 ? "" : "empty",
                  lv > 0.55 ? "strong" : "",
                  v === max && v > 0 ? "peak" : "",
                  clickable ? "click" : "",
                ]
                  .filter(Boolean)
                  .join(" ");
                return (
                  <td key={i}>
                    <span
                      className={cls}
                      style={v > 0 ? { background: `rgba(91,108,255,${0.1 + 0.8 * lv})` } : undefined}
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
              <td className="hm-sum">
                <span className="hm-sum-bar">
                  <i style={{ width: `${(r.total / maxTotal) * 100}%` }} />
                </span>
                <strong>{compact(r.total)}</strong>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
