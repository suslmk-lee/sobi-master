import { useId } from "react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ReferenceArea,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { ChartTip, compact, cv, LegendDot, niceMax, useAxisTick } from "./charts";
import { useThemeColors } from "./appearance/AppearanceContext";

// 전월 동일시점 누적 비교 — 통계 탭의 대표 차트.
//
// "이번 달 지금까지 지난달 이맘때보다 더 썼나?"가 가계부에서 가장 자주 묻는 질문이라
// 맨 위에 크게 둔다. 숫자 네 개(이번 달·전월 같은 기간·차이·월말 예상)로 답을 먼저 주고,
// 차트는 그 답이 어떻게 만들어졌는지 보여 준다.
//   - 이번 달: 강조색 선 + 면. 오늘까지만 그린다(앞날은 0원이라 평평한 선이 거짓말을 한다).
//   - 전월: 옅은 기준선.
//   - 진행 중인 달이면 지금 속도로 월말까지 간다고 보고 점선(추정, §11.4) 예상선을 긋는다.
//   - 오늘 지점에서 두 선의 간격을 선분으로 잇고 차액을 붙인다.
//   - 주말은 옅은 띠.

const WD = ["일", "월", "화", "수", "목", "금", "토"];

function daysIn(ym: string) {
  const [y, m] = ym.split("-").map(Number);
  return new Date(y, m, 0).getDate();
}

type Row = { d: number; cur?: number; prev?: number; proj?: number };

export default function CompareChart({ data }: { data: store.CumCompare }) {
  const tc = useThemeColors();
  const tick = useAxisTick();
  const gid = useId().replace(/[^a-zA-Z0-9_-]/g, "");

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
  const up = diff > 0;

  const days = Math.max(curDays, prevDays);
  const max = niceMax(Math.max(1, prevTotal, projected, curAt) * 1.04);

  const rows: Row[] = Array.from({ length: days }, (_, i) => {
    const d = i + 1;
    const r: Row = { d };
    if (d <= cd) r.cur = data.current[d - 1] ?? 0;
    if (d <= prevDays) r.prev = data.previous[d - 1] ?? 0;
    // 추정선: 오늘 점에서 시작해 월말까지(경계 점을 실제 구간과 공유)
    if (inProgress && d >= cd && d <= curDays) r.proj = Math.round(curAt + perDay * (d - cd));
    return r;
  });

  const isWeekend = (d: number) => {
    const w = new Date(cy, cm - 1, d).getDay();
    return w === 0 || w === 6;
  };
  const weekendDays = Array.from({ length: curDays }, (_, i) => i + 1).filter(isWeekend);
  const ticks = [1, 5, 10, 15, 20, 25, days].filter((d, i, a) => d <= days && a.indexOf(d) === i);

  const prevColor = tc.axis;
  const gapColor = up ? cv("rose-400") : cv("emerald-400");

  return (
    <div>
      {/* 답부터: 숫자 네 개 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-5">
        <div className="surface-inset p-4 flex flex-col gap-1">
          <span className="label-mono inline-flex items-center gap-1.5">
            <LegendDot color={tc.accent} /> {cm}월 {cd}일까지 {inProgress ? "" : "(말일)"}
          </span>
          <strong className="num text-2xl font-semibold text-white">{won(curAt)}</strong>
          <span className="text-xs text-zinc-500">하루 평균 <span className="num">{won(Math.round(perDay))}</span></span>
        </div>
        <div className="surface-inset p-4 flex flex-col gap-1">
          <span className="label-mono inline-flex items-center gap-1.5">
            <LegendDot color={prevColor} /> {pm}월 같은 기간
          </span>
          <strong className="num text-xl font-semibold text-white">{won(prevAt)}</strong>
          <span className="text-xs text-zinc-500">{pm}월 전체 <span className="num">{won(prevTotal)}</span></span>
        </div>
        <div className="surface-inset p-4 flex flex-col gap-1">
          <span className="label-mono">차이</span>
          <strong className={`num text-xl font-semibold ${diff === 0 ? "text-white" : up ? "text-rose-400" : "text-emerald-400"}`}>
            {diff === 0 ? "같음" : `${up ? "▲" : "▼"} ${won(Math.abs(diff))}`}
          </strong>
          <span className="text-xs text-zinc-500">
            {diffPct === null ? "전월 데이터 없음" : `전월 대비 ${diffPct > 0 ? "+" : ""}${diffPct}% ${up ? "더 씀" : "덜 씀"}`}
          </span>
        </div>
        <div className="surface-inset p-4 flex flex-col gap-1">
          <span className="label-mono">{inProgress ? "지금 속도면 월말" : `${cm}월 최종`}</span>
          <strong className="num text-xl font-semibold text-white">{inProgress ? `약 ${won(projected)}` : won(curAt)}</strong>
          <span className="text-xs text-zinc-500">
            {projVsPrev === null
              ? `남은 ${curDays - cd}일`
              : `${pm}월 총액보다 ${Math.abs(projVsPrev)}% ${projVsPrev > 0 ? "많음" : "적음"}`}
          </span>
        </div>
      </div>

      <div className="h-80">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={rows} margin={{ top: 22, right: 28, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id={`${gid}-c`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={tc.accent} stopOpacity={0.32} />
                <stop offset="100%" stopColor={tc.accent} stopOpacity={0} />
              </linearGradient>
              <linearGradient id={`${gid}-p`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={prevColor} stopOpacity={0.14} />
                <stop offset="100%" stopColor={prevColor} stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid stroke={tc.grid} strokeOpacity={0.4} vertical={false} />
            {/* 주말 띠 */}
            {weekendDays.map((d) => (
              <ReferenceArea key={d} x1={d - 0.5} x2={d + 0.5} fill={tc.text} fillOpacity={0.04} stroke="none" ifOverflow="extendDomain" />
            ))}
            <XAxis
              dataKey="d"
              type="number"
              domain={[1, days]}
              ticks={ticks}
              tickFormatter={(d: number) => `${d}일`}
              tick={tick}
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
              domain={[0, max]}
              allowDataOverflow
            />
            <Tooltip
              cursor={{ stroke: tc.axis, strokeDasharray: "3 3" }}
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                const d = (payload[0].payload as Row).d;
                const cur = d <= cd ? data.current[d - 1] ?? 0 : null;
                const curDay = d <= cd ? (data.current[d - 1] ?? 0) - (d > 1 ? data.current[d - 2] ?? 0 : 0) : null;
                const prev = d <= prevDays ? data.previous[d - 1] ?? 0 : null;
                const proj = d > cd && inProgress && d <= curDays ? Math.round(curAt + perDay * (d - cd)) : null;
                const wd = d <= curDays ? WD[new Date(cy, cm - 1, d).getDay()] : "";
                const tipRows = [];
                if (cur !== null) tipRows.push({ color: tc.accent, value: won(cur), name: "이번 달 누적" });
                if (curDay !== null && curDay > 0) tipRows.push({ value: won(curDay), name: "그날 지출", sub: true });
                if (proj !== null) tipRows.push({ color: tc.accent, value: `약 ${won(proj)}`, name: "지금 속도 예상 (추정)", faint: true });
                if (prev !== null) tipRows.push({ color: prevColor, value: won(prev), name: `${pm}월 누적` });
                if (cur !== null && prev !== null && prev > 0) {
                  tipRows.push({
                    color: cur > prev ? cv("rose-400") : cv("emerald-400"),
                    value: `${cur > prev ? "+" : "−"}${won(Math.abs(cur - prev))}`,
                    name: "전월보다",
                  });
                }
                return <ChartTip head={`${cm}월 ${d}일${wd && ` (${wd})`}`} rows={tipRows} />;
              }}
            />
            {/* 전월: 옅은 면 + 기준선 */}
            <Area type="monotone" dataKey="prev" stroke={prevColor} strokeWidth={1.75} fill={`url(#${gid}-p)`} dot={false} activeDot={{ r: 4, fill: prevColor }} isAnimationActive={false} connectNulls={false} />
            {/* 이번 달: 실제 */}
            <Area type="monotone" dataKey="cur" stroke={tc.accent} strokeWidth={1.75} fill={`url(#${gid}-c)`} dot={false} activeDot={{ r: 4.5, fill: tc.accent }} isAnimationActive={false} connectNulls={false} />
            {/* 지금 속도대로 월말까지: 추정 구간 */}
            {inProgress && (
              <Line type="linear" dataKey="proj" stroke={tc.accent} strokeOpacity={0.55} strokeWidth={1.5} strokeDasharray="4 3" dot={false} activeDot={false} isAnimationActive={false} connectNulls={false} legendType="none" />
            )}
            {inProgress && (
              <ReferenceDot
                x={curDays}
                y={projected}
                r={4}
                fill={tc.accent}
                fillOpacity={0.55}
                stroke="none"
                ifOverflow="extendDomain"
                label={{ value: `예상 ${compact(projected)}`, position: "top", fill: tc.axis, fontSize: 11, fontFamily: "var(--font-mono)" }}
              />
            )}
            {/* 전월 끝 라벨 */}
            <ReferenceDot
              x={prevDays}
              y={prevTotal}
              r={0}
              ifOverflow="extendDomain"
              label={{ value: `${pm}월 총 ${compact(prevTotal)}`, position: "top", fill: tc.axis, fontSize: 11, fontFamily: "var(--font-mono)" }}
            />
            {/* 오늘: 세로선 + 두 선 간격 + 점 */}
            <ReferenceLine
              x={cd}
              stroke={tc.axis}
              strokeDasharray="3 3"
              label={{ value: inProgress ? `오늘 ${cd}일` : `${cd}일`, position: "top", fill: tc.text, fontSize: 11, fontFamily: "var(--font-mono)" }}
            />
            {prevAt > 0 && diff !== 0 && (
              <ReferenceLine
                segment={[{ x: cd, y: prevAt }, { x: cd, y: curAt }]}
                stroke={gapColor}
                strokeWidth={3}
                strokeLinecap="round"
                ifOverflow="extendDomain"
                label={{
                  value: `${up ? "▲" : "▼"} ${won(Math.abs(diff))}${diffPct !== null ? ` ${diffPct > 0 ? "+" : ""}${diffPct}%` : ""}`,
                  position: cd > days * 0.7 ? "left" : "right",
                  fill: gapColor,
                  fontSize: 11,
                  fontFamily: "var(--font-mono)",
                }}
              />
            )}
            <ReferenceDot x={cd} y={prevAt} r={5} fill={prevColor} stroke={tc.surface} strokeWidth={2} ifOverflow="extendDomain" />
            <ReferenceDot x={cd} y={curAt} r={6.5} fill={tc.accent} stroke={tc.surface} strokeWidth={2} ifOverflow="extendDomain" />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      <div className="flex flex-wrap gap-x-4 gap-y-1 mt-3 text-[11px]" style={{ color: tc.muted }}>
        <span className="inline-flex items-center gap-1.5"><LegendDot color={tc.accent} /> {cm}월 (이번 달)</span>
        <span className="inline-flex items-center gap-1.5"><LegendDot color={prevColor} /> {pm}월 (전월)</span>
        {inProgress && <span className="inline-flex items-center gap-1.5"><LegendDot color={tc.accent} dashed /> 지금 속도 예상</span>}
        <span className="inline-flex items-center gap-1.5"><i className="inline-block w-3 h-2.5 rounded-sm bg-white/10" /> 주말</span>
      </div>
    </div>
  );
}
