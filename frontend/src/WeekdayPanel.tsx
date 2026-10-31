import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { ChartEmpty, compact } from "./charts";

// 요일별 평균 지출 — "무슨 요일에 돈을 쓰나".
//
// 옆의 지출 달력과 같은 데이터(전월 비교용 일별 누적)에서 계산한다. 두 카드의 숫자가
// 늘 맞아떨어지고, 진행 중인 달에서는 "이미 지나간 날"만 센다. 아직 오지 않은 목요일까지
// 나눗셈에 넣으면 평균이 실제보다 낮게 나온다(10/18 기준 목요일은 5번이 아니라 3번).
// 세로 막대는 달력의 일~토 7칸과 같은 방향이라 두 카드를 나란히 읽을 수 있다.

const NAMES = ["일요일", "월요일", "화요일", "수요일", "목요일", "금요일", "토요일"];
const SHORT = ["일", "월", "화", "수", "목", "금", "토"];

export default function WeekdayPanel({ data }: { data: store.CumCompare }) {
  const [y, m] = data.curMonth.split("-").map(Number);
  const days = new Date(y, m, 0).getDate();
  const upto = Math.min(data.compareDay, days);

  const stat = SHORT.map(() => ({ total: 0, count: 0, spendDays: 0 }));
  for (let d = 1; d <= upto; d++) {
    const amt = Math.max(0, (data.current[d - 1] ?? 0) - (d > 1 ? data.current[d - 2] ?? 0 : 0));
    const w = new Date(y, m - 1, d).getDay();
    stat[w].total += amt;
    stat[w].count += 1;
    if (amt > 0) stat[w].spendDays += 1;
  }
  const avg = stat.map((s) => (s.count ? Math.round(s.total / s.count) : 0));
  const max = Math.max(1, ...avg);
  const top = avg.indexOf(Math.max(...avg));

  const sum = (ws: number[]) => ws.reduce((a, w) => a + stat[w].total, 0);
  const cnt = (ws: number[]) => ws.reduce((a, w) => a + stat[w].count, 0);
  const weekdayAvg = cnt([1, 2, 3, 4, 5]) ? Math.round(sum([1, 2, 3, 4, 5]) / cnt([1, 2, 3, 4, 5])) : 0;
  const weekendAvg = cnt([0, 6]) ? Math.round(sum([0, 6]) / cnt([0, 6])) : 0;
  const ratio = weekdayAvg > 0 ? weekendAvg / weekdayAvg : 0;

  // 가장 많이 쓰는 요일이 나머지 요일 평균의 몇 배인지
  const others = avg.filter((_, i) => i !== top && stat[i].count > 0);
  const othersAvg = others.length ? others.reduce((a, b) => a + b, 0) / others.length : 0;
  const topRatio = othersAvg > 0 ? avg[top] / othersAvg : 0;
  const hasData = avg.some((v) => v > 0);

  return (
    <div className="flex flex-col h-full gap-4">
      <div className="grid grid-cols-2 gap-3">
        <div className="surface-inset p-3 flex flex-col gap-1">
          <span className="label-mono">평일 하루 평균</span>
          <strong className="num text-base font-semibold text-white">{won(weekdayAvg)}</strong>
        </div>
        <div className="surface-inset p-3 flex flex-col gap-1">
          <span className="label-mono">주말 하루 평균</span>
          <strong className="num text-base font-semibold text-white">{won(weekendAvg)}</strong>
          {weekdayAvg > 0 && weekendAvg > 0 && (
            <em className={`not-italic num text-[11px] ${ratio > 1 ? "text-rose-400" : "text-emerald-400"}`}>
              평일의 {ratio.toFixed(1)}배
            </em>
          )}
        </div>
      </div>

      {!hasData ? (
        <ChartEmpty>이번 달 지출이 없습니다.</ChartEmpty>
      ) : (
        <div className="grid grid-cols-7 gap-1.5 flex-1 min-h-[150px]">
          {avg.map((v, i) => {
            const s = stat[i];
            return (
              <div
                key={i}
                className="flex flex-col items-center gap-1.5 min-w-0"
                title={`${NAMES[i]} · 지난 ${s.count}번 중 ${s.spendDays}번 지출 · 합계 ${won(s.total)} · 하루 평균 ${won(v)}`}
              >
                <span className={`num text-[10px] ${i === top ? "text-white font-semibold" : "text-zinc-500"}`}>
                  {v > 0 ? compact(v) : "—"}
                </span>
                <div className="flex-1 w-full flex items-end rounded-[4px] bg-white/[0.03] min-h-[80px]">
                  <div
                    className={`w-full rounded-[4px] ${i === top ? "bg-mint-400" : "bg-mint-400/40"}`}
                    style={{ height: `${Math.max(v > 0 ? 3 : 0, (v / max) * 100)}%` }}
                  />
                </div>
                <span className={`text-[11px] font-medium ${i === 0 ? "text-rose-400" : i === 6 ? "text-sky-400" : "text-zinc-300"}`}>
                  {SHORT[i]}
                </span>
                <span className="num text-[10px] text-zinc-500">{s.spendDays}/{s.count}일</span>
              </div>
            );
          })}
        </div>
      )}

      {hasData && (
        <p className="text-xs text-zinc-400">
          <strong className="font-semibold text-white">{NAMES[top]}</strong>에 가장 많이 써요 · 하루 평균 <span className="num">{won(avg[top])}</span>
          {topRatio >= 1.2 && <>, 다른 요일 평균의 <strong className="num font-semibold text-white">{topRatio.toFixed(1)}배</strong></>}
        </p>
      )}
    </div>
  );
}
