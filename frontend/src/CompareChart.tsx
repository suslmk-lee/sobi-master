import { useId, useState } from "react";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { compact, monotonePath, niceMax, useChartWidth } from "./charts";

// 전월 동일시점 누적 비교 — 통계 탭의 대표 차트.
//
// "이번 달 지금까지 지난달 이맘때보다 더 썼나?"가 가계부에서 가장 자주 묻는 질문이라
// 맨 위에 크게 둔다. 숫자 네 개(이번 달·전월 같은 기간·차이·월말 예상)로 답을 먼저 주고,
// 차트는 그 답이 어떻게 만들어졌는지 보여 준다.
//   - 이번 달: 빛나는 선 + 면. 오늘까지만 그린다(앞날은 0원이라 평평한 선이 거짓말을 한다).
//   - 전월: 옅은 회색 기준선.
//   - 오늘 지점에서 두 선의 간격을 막대로 잇고 차액을 칩으로 붙인다.
//   - 진행 중인 달이면 지금 속도로 월말까지 간다고 보고 점선 예상선을 긋는다.
//   - 주말은 옅은 띠 — 주말에 몰아 쓰는지 한눈에 보인다.

const WD = ["일", "월", "화", "수", "목", "금", "토"];
const C_PREV = "#94a3b8";

function daysIn(ym: string) {
  const [y, m] = ym.split("-").map(Number);
  return new Date(y, m, 0).getDate();
}

export default function CompareChart({ data }: { data: store.CumCompare }) {
  const [ref, W] = useChartWidth(900);
  const [hi, setHi] = useState<number | null>(null);
  const gid = useId();

  const [cy, cm] = data.curMonth.split("-").map(Number);
  const pm = Number(data.prevMonth.slice(5));
  const curDays = daysIn(data.curMonth);
  const prevDays = daysIn(data.prevMonth);
  const cd = Math.max(1, Math.min(data.compareDay, curDays));
  const inProgress = cd < curDays;

  const curAt = data.current[cd - 1] ?? 0;
  const prevAt = data.previous[cd - 1] ?? 0;
  const prevTotal = data.previous[prevDays - 1] ?? 0;
  const diff = curAt - prevAt;
  const diffPct = prevAt > 0 ? Math.round((diff / prevAt) * 100) : null;
  const perDay = curAt / cd;
  const projected = inProgress ? Math.round(perDay * curDays) : curAt;
  const projVsPrev = prevTotal > 0 ? Math.round(((projected - prevTotal) / prevTotal) * 100) : null;

  // 좌표
  const H = 300;
  const padL = 54;
  const padR = 28;
  const padT = 26;
  const padB = 30;
  const days = Math.max(curDays, prevDays);
  const max = niceMax(Math.max(1, prevTotal, projected, curAt) * 1.04);
  const step = (W - padL - padR) / Math.max(1, days - 1);
  const x = (d: number) => padL + step * (d - 1); // d 는 1부터
  const y = (v: number) => H - padB - ((H - padT - padB) * v) / max;
  const base = H - padB;

  const curPts: [number, number][] = [];
  for (let d = 1; d <= cd; d++) curPts.push([x(d), y(data.current[d - 1] ?? 0)]);
  const prevPts: [number, number][] = [];
  for (let d = 1; d <= prevDays; d++) prevPts.push([x(d), y(data.previous[d - 1] ?? 0)]);
  const curLine = monotonePath(curPts);
  const prevLine = monotonePath(prevPts);
  const area = (line: string, pts: [number, number][]) =>
    pts.length ? `${line} L ${pts[pts.length - 1][0]} ${base} L ${pts[0][0]} ${base} Z` : "";

  const tx = x(cd);
  const tyCur = y(curAt);
  const tyPrev = y(prevAt);
  const up = diff > 0;

  const weekend = (d: number) => {
    const w = new Date(cy, cm - 1, d).getDay();
    return w === 0 || w === 6;
  };
  const ticks = [1, 5, 10, 15, 20, 25, days].filter((d, i, a) => d <= days && a.indexOf(d) === i);

  const onMove = (e: React.PointerEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    const px = ((e.clientX - r.left) / r.width) * W;
    setHi(Math.max(1, Math.min(days, Math.round((px - padL) / step) + 1)));
  };

  // 툴팁 내용
  const tip = (() => {
    if (hi === null) return null;
    const d = hi;
    const cur = d <= cd ? data.current[d - 1] ?? 0 : null;
    const curDay = d <= cd ? (data.current[d - 1] ?? 0) - (d > 1 ? data.current[d - 2] ?? 0 : 0) : null;
    const prev = d <= prevDays ? data.previous[d - 1] ?? 0 : null;
    const proj = d > cd && inProgress && d <= curDays ? Math.round(curAt + perDay * (d - cd)) : null;
    const wd = d <= curDays ? WD[new Date(cy, cm - 1, d).getDay()] : "";
    return { d, cur, curDay, prev, proj, wd };
  })();
  const tipFlip = hi !== null && x(hi) > W * 0.62;

  return (
    <div className="cmp-hero">
      {/* 답부터: 숫자 네 개 */}
      <div className="cmp-figures">
        <div className="cmp-fig main">
          <span className="cmp-cap">
            <i className="dot cur" /> {cm}월 {cd}일까지 {inProgress ? "" : "(말일)"}
          </span>
          <strong className="cmp-hero-num">{won(curAt)}</strong>
          <span className="muted small">하루 평균 {won(Math.round(perDay))}</span>
        </div>
        <div className="cmp-fig">
          <span className="cmp-cap">
            <i className="dot prev" /> {pm}월 같은 기간
          </span>
          <strong>{won(prevAt)}</strong>
          <span className="muted small">{pm}월 전체 {won(prevTotal)}</span>
        </div>
        <div className="cmp-fig">
          <span className="cmp-cap">차이</span>
          <strong className={diff === 0 ? "" : up ? "expense" : "income"}>
            {diff === 0 ? "같음" : `${up ? "▲" : "▼"} ${won(Math.abs(diff))}`}
          </strong>
          <span className="muted small">
            {diffPct === null ? "전월 데이터 없음" : `전월 대비 ${diffPct > 0 ? "+" : ""}${diffPct}% ${up ? "더 씀" : "덜 씀"}`}
          </span>
        </div>
        <div className="cmp-fig">
          <span className="cmp-cap">{inProgress ? "지금 속도면 월말" : `${cm}월 최종`}</span>
          <strong>{inProgress ? `약 ${won(projected)}` : won(curAt)}</strong>
          <span className="muted small">
            {projVsPrev === null
              ? `남은 ${curDays - cd}일`
              : `${pm}월 총액보다 ${Math.abs(projVsPrev)}% ${projVsPrev > 0 ? "많음" : "적음"}`}
          </span>
        </div>
      </div>

      <div ref={ref} className="cmp-plot">
        <svg
          viewBox={`0 0 ${W} ${H}`}
          className="chart cmp-chart"
          onPointerMove={onMove}
          onPointerLeave={() => setHi(null)}
        >
          <defs>
            <linearGradient id={`cg-${gid}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" style={{ stopColor: "var(--accent)", stopOpacity: 0.34 }} />
              <stop offset="100%" style={{ stopColor: "var(--accent)", stopOpacity: 0 }} />
            </linearGradient>
            <linearGradient id={`cl-${gid}`} x1="0" y1="0" x2="1" y2="0">
              <stop offset="0%" stopColor="#5b6cff" />
              <stop offset="100%" stopColor="#c04cf0" />
            </linearGradient>
          </defs>

          {/* 주말 띠 */}
          {Array.from({ length: curDays }, (_, i) => i + 1)
            .filter(weekend)
            .map((d) => (
              <rect
                key={d}
                x={x(d) - step / 2}
                y={padT}
                width={step}
                height={base - padT}
                className="cmp-weekend"
              />
            ))}

          {/* 눈금 */}
          {[0, 0.25, 0.5, 0.75, 1].map((f) => (
            <g key={f}>
              <line
                x1={padL} x2={W - padR} y1={y(max * f)} y2={y(max * f)}
                className={f === 0 ? "cmp-axis" : "cmp-grid"}
              />
              <text x={padL - 10} y={y(max * f) + 4} fontSize={11} textAnchor="end">
                {compact(max * f)}
              </text>
            </g>
          ))}
          {ticks.map((d) => (
            <text key={d} x={x(d)} y={H - 9} fontSize={11} textAnchor="middle">
              {d}일
            </text>
          ))}

          {/* 전월: 옅은 면 + 회색 기준선 */}
          <path d={area(prevLine, prevPts)} fill={C_PREV} opacity={0.1} />
          <path d={prevLine} fill="none" stroke={C_PREV} strokeWidth={2} strokeLinecap="round" />

          {/* 이번 달: 그라디언트 면 + 빛나는 선 */}
          <path d={area(curLine, curPts)} fill={`url(#cg-${gid})`} />
          <path
            d={curLine}
            fill="none"
            stroke={`url(#cl-${gid})`}
            strokeWidth={3}
            strokeLinecap="round"
            strokeLinejoin="round"
            className="cmp-cur-line"
          />

          {/* 지금 속도대로 월말까지의 예상선 */}
          {inProgress && (
            <g className="cmp-proj">
              <line x1={tx} y1={tyCur} x2={x(curDays)} y2={y(projected)} />
              <circle cx={x(curDays)} cy={y(projected)} r={4} />
              <text x={x(curDays) - 6} y={y(projected) - 10} fontSize={11} textAnchor="end">
                예상 {compact(projected)}
              </text>
            </g>
          )}

          {/* 전월 끝 라벨 */}
          <text
            x={x(prevDays) - 6}
            y={y(prevTotal) - 9}
            fontSize={11}
            textAnchor="end"
            className="cmp-prev-label"
          >
            {pm}월 총 {compact(prevTotal)}
          </text>

          {/* 오늘: 세로선 + 두 선 간격 막대 + 점 */}
          <line x1={tx} x2={tx} y1={padT - 6} y2={base} className="cmp-today" />
          <text x={tx} y={padT - 10} fontSize={11} textAnchor="middle" className="cmp-today-label">
            {inProgress ? `오늘 ${cd}일` : `${cd}일`}
          </text>
          {prevAt > 0 && diff !== 0 && (
            <line
              x1={tx} x2={tx} y1={tyPrev} y2={tyCur}
              className={`cmp-gap ${up ? "up" : "down"}`}
            />
          )}
          <circle cx={tx} cy={tyPrev} r={5} className="cmp-dot-prev" />
          <circle cx={tx} cy={tyCur} r={6.5} className="cmp-dot-cur" />

          {/* 마우스 십자선 */}
          {hi !== null && (
            <g pointerEvents="none">
              <line x1={x(hi)} x2={x(hi)} y1={padT} y2={base} className="lc-cross" />
              {tip?.cur !== null && tip?.cur !== undefined && (
                <circle cx={x(hi)} cy={y(tip.cur)} r={4.5} className="lc-hot" style={{ fill: "var(--accent)" }} />
              )}
              {tip?.prev !== null && tip?.prev !== undefined && (
                <circle cx={x(hi)} cy={y(tip.prev)} r={4} className="lc-hot" fill={C_PREV} />
              )}
            </g>
          )}
        </svg>

        {/* 오늘 지점의 차액 칩 */}
        {prevAt > 0 && diff !== 0 && hi === null && (
          <div
            className={`cmp-chip ${up ? "up" : "down"}`}
            style={{
              left: tx + (tx > W * 0.7 ? -14 : 14),
              top: (tyCur + tyPrev) / 2,
              transform: `translate(${tx > W * 0.7 ? "-100%" : "0"}, -50%)`,
            }}
          >
            {up ? "▲" : "▼"} {won(Math.abs(diff))}
            <span>{diffPct !== null ? ` ${diffPct > 0 ? "+" : ""}${diffPct}%` : ""}</span>
          </div>
        )}

        {tip && (
          <div className={`chart-tip${tipFlip ? " flip" : ""}`} style={{ left: x(tip.d) }}>
            <div className="ct-head">
              {cm}월 {tip.d}일{tip.wd && ` (${tip.wd})`}
            </div>
            {tip.cur !== null && (
              <div className="ct-row">
                <i style={{ background: "var(--accent)" }} />
                <strong>{won(tip.cur)}</strong>
                <span>이번 달 누적</span>
              </div>
            )}
            {tip.curDay !== null && tip.curDay > 0 && (
              <div className="ct-row sub">
                <i />
                <strong>{won(tip.curDay)}</strong>
                <span>그날 지출</span>
              </div>
            )}
            {tip.proj !== null && (
              <div className="ct-row">
                <i style={{ background: "var(--accent)", opacity: 0.5 }} />
                <strong>약 {won(tip.proj)}</strong>
                <span>지금 속도 예상</span>
              </div>
            )}
            {tip.prev !== null && (
              <div className="ct-row">
                <i style={{ background: C_PREV }} />
                <strong>{won(tip.prev)}</strong>
                <span>{pm}월 누적</span>
              </div>
            )}
            {tip.cur !== null && tip.prev !== null && tip.prev > 0 && (
              <div className={`ct-row diff ${tip.cur > tip.prev ? "up" : "down"}`}>
                <i />
                <strong>
                  {tip.cur > tip.prev ? "+" : "−"}
                  {won(Math.abs(tip.cur - tip.prev))}
                </strong>
                <span>전월보다</span>
              </div>
            )}
          </div>
        )}
      </div>

      <div className="cmp-legend2">
        <span><i className="dot cur" /> {cm}월 (이번 달)</span>
        <span><i className="dot prev" /> {pm}월 (전월)</span>
        {inProgress && <span><i className="dash" /> 지금 속도 예상</span>}
        <span><i className="wk" /> 주말</span>
      </div>
    </div>
  );
}
