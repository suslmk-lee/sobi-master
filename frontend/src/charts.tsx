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
  "#5b82f0",
  "#f43f5e",
  "#22c55e",
  "#eab308",
  "#a855f7",
  "#06b6d4",
  "#f97316",
  "#64748b",
];

const compact = (n: number) =>
  n >= 100000000
    ? `${(n / 100000000).toFixed(1)}억`
    : n >= 10000
    ? `${Math.round(n / 10000)}만`
    : String(n);

// 최근 n개월 수입/지출 막대 추이
export function TrendChart({ points }: { points: store.MonthPoint[] }) {
  const W = 660;
  const H = 220;
  const padX = 36;
  const padY = 26;
  const max = Math.max(1, ...points.map((p) => Math.max(p.income, p.expense)));
  const slot = (W - padX * 2) / Math.max(1, points.length);
  const bw = Math.min(16, slot / 3);
  const scaleY = (v: number) => ((H - padY * 2) * v) / max;

  return (
    <div>
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
            <text x={4} y={H - padY - (H - padY * 2) * f + 4} fontSize={9} fill={C_TEXT}>
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
              <rect x={cx - bw - 1} y={H - padY - hi} width={bw} height={hi} rx={3} fill={C_INCOME} opacity={0.9} />
              <rect x={cx + 1} y={H - padY - he} width={bw} height={he} rx={3} fill={C_EXPENSE} opacity={0.9} />
              <text x={cx} y={H - 8} fontSize={10} fill={C_TEXT} textAnchor="middle">
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
  const W = 660;
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
    <div>
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
            <text key={i} x={x(i)} y={H - 6} fontSize={9} fill={C_TEXT} textAnchor="middle">
              {p.day}
            </text>
          ) : null
        )}
        <text x={4} y={padY + 4} fontSize={9} fill={C_TEXT}>{compact(max)}</text>
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

// 멀티라인 꺾은선 그래프. cumulative=true 면 각 시리즈를 누적합으로 그린다(범례 합계는 원본 합).
// marker 를 주면 해당 x 위치(1-based)에 점선 세로선을 그린다(전월 비교의 "동일시점").
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
  const W = 660;
  const H = height;
  const padX = 42;
  const padY = 24;
  const n = xLabels.length;

  const plotted = series.map((s) => {
    if (!cumulative) return s.values;
    let acc = 0;
    return s.values.map((v) => (acc += v));
  });
  const max = Math.max(1, ...plotted.flatMap((vals) => vals));
  const totals = series.map((s) => s.values.reduce((a, b) => a + b, 0));

  const x = (i: number) => padX + ((W - padX * 2) * i) / Math.max(1, n - 1);
  const y = (v: number) => H - padY - ((H - padY * 2) * v) / max;
  const color = (i: number) => series[i].color || PALETTE[i % PALETTE.length];

  const hasData = totals.some((t) => t > 0);
  const showDots = n <= 12;
  const labelEvery = n <= 12 ? 1 : 5;

  return (
    <div>
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
        <svg viewBox={`0 0 ${W} ${H}`} className="chart">
          {[0, 0.25, 0.5, 0.75, 1].map((f) => (
            <g key={f}>
              <line
                x1={padX} x2={W - padX}
                y1={H - padY - (H - padY * 2) * f}
                y2={H - padY - (H - padY * 2) * f}
                stroke={C_GRID}
              />
              <text x={4} y={H - padY - (H - padY * 2) * f + 4} fontSize={9} fill={C_TEXT}>
                {compact(max * f)}
              </text>
            </g>
          ))}
          {marker && marker.x >= 1 && marker.x <= n && (
            <g>
              <line
                x1={x(marker.x - 1)} x2={x(marker.x - 1)}
                y1={padY - 6} y2={H - padY}
                stroke={C_TEXT} strokeWidth={1} strokeDasharray="3 3"
              />
              {marker.label && (
                <text x={x(marker.x - 1)} y={padY - 9} fontSize={9} fill={C_TEXT} textAnchor="middle">
                  {marker.label}
                </text>
              )}
            </g>
          )}
          {plotted.map((vals, si) => (
            <g key={si}>
              <path
                d={vals.map((v, i) => `${i === 0 ? "M" : "L"} ${x(i)} ${y(v)}`).join(" ")}
                fill="none"
                stroke={color(si)}
                strokeWidth={2}
                strokeLinejoin="round"
                strokeLinecap="round"
              />
              {showDots &&
                vals.map((v, i) => (
                  <circle key={i} cx={x(i)} cy={y(v)} r={2.5} fill={color(si)} />
                ))}
            </g>
          ))}
          {xLabels.map((lab, i) =>
            i % labelEvery === 0 || i === n - 1 ? (
              <text key={i} x={x(i)} y={H - 6} fontSize={9} fill={C_TEXT} textAnchor="middle">
                {lab}
              </text>
            ) : null
          )}
        </svg>
      )}
    </div>
  );
}

// 결제수단별 카테고리 적층 막대. 막대 길이=카드 총액(최대 대비), 세그먼트=카테고리 구성.
// 카테고리 색은 전 카드 공통(합계 큰 순으로 팔레트 배정).
export function StackedBars({ data }: { data: store.CardCategory[] }) {
  if (data.length === 0) return <p className="muted">데이터 없음</p>;

  const catTotals: Record<string, number> = {};
  for (const d of data)
    for (const c of d.categories) catTotals[c.name] = (catTotals[c.name] || 0) + c.amount;
  const cats = Object.keys(catTotals).sort((a, b) => catTotals[b] - catTotals[a]);
  const catColor: Record<string, string> = {};
  cats.forEach((c, i) => (catColor[c] = PALETTE[i % PALETTE.length]));

  const max = Math.max(1, ...data.map((d) => d.total));

  return (
    <div>
      <div className="legend wrap">
        {cats.map((c) => (
          <span key={c}>
            <i style={{ background: catColor[c] }} /> {c}
          </span>
        ))}
      </div>
      <div className="stacked-list">
        {data.map((d) => (
          <div className="stacked-row" key={d.card}>
            <span className="stacked-label">{d.card}</span>
            <div className="stacked-cell">
              <div className="stacked-track" style={{ width: `${(d.total / max) * 100}%` }}>
                {d.categories.map((c, i) => (
                  <div
                    key={i}
                    className="stacked-seg"
                    style={{ width: `${(c.amount / d.total) * 100}%`, background: catColor[c.name] }}
                    title={`${c.name} ${won(c.amount)} (${Math.round((c.amount / d.total) * 100)}%)`}
                  />
                ))}
              </div>
            </div>
            <span className="stacked-amount">{won(d.total)}</span>
          </div>
        ))}
      </div>
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
