import { store } from "../wailsjs/go/models";
import { won } from "./lib";

const C_INCOME = "#22c55e";
const C_EXPENSE = "#f43f5e";
const C_TRANSFER = "#a855f7";
const C_GRID = "rgba(42,47,69,0.10)";
const C_TEXT = "rgba(42,47,69,0.5)";

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
        <text x={85} y={98} textAnchor="middle" fontSize={13} fontWeight={700} fill="#2a2f45">
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
