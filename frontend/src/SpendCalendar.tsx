import { useEffect, useState } from "react";
import { GetHolidays } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { compact, cv } from "./charts";

// 지출 달력 — 이번 달을 달력으로 펼쳐 날마다 쓴 금액을 색 진하기로 보여 준다.
// 요일별 평균("언제 쓰나")을 날짜 단위로 펼친 것이라 같은 일~토 7칸 구조로 둔다.
// 날짜를 누르면 그날 쓴 내역이 열린다(onPickDay).
//
// 일별 금액은 전월 비교용 누적 데이터의 하루 차이로 구한다 — 따로 조회하지 않는다.
// 색 진하기는 제곱근 척도(강조색 알파 단계): 큰 날 하루가 나머지를 모두 흐리게 만들지 않게 한다.

const WD = ["일", "월", "화", "수", "목", "금", "토"];

export default function SpendCalendar({
  data,
  onPickDay,
}: {
  data: store.CumCompare;
  onPickDay: (date: string, label: string) => void;
}) {
  const [y, m] = data.curMonth.split("-").map(Number);
  const days = new Date(y, m, 0).getDate();
  const offset = new Date(y, m - 1, 1).getDay(); // 1일의 요일 → 앞 빈칸 수
  const upto = Math.min(data.compareDay, days); // 이 날까지만 실제 데이터(진행 중인 달의 앞날은 미래)
  const inProgress = upto < days;

  const [holidays, setHolidays] = useState<Record<string, string>>({});
  useEffect(() => {
    GetHolidays(y)
      .then((h) => {
        const map: Record<string, string> = {};
        for (const e of h.entries ?? []) if (!e.off) map[e.date] = e.name;
        setHolidays(map);
      })
      .catch(() => setHolidays({}));
  }, [y]);

  const daily = Array.from({ length: days }, (_, i) => {
    const cur = data.current[i] ?? 0;
    const prev = i > 0 ? data.current[i - 1] ?? 0 : 0;
    return Math.max(0, cur - prev);
  });
  const spent = daily.slice(0, upto);
  const max = Math.max(1, ...spent);
  const topDay = spent.indexOf(Math.max(...spent)) + 1;
  const spendDays = spent.filter((v) => v > 0).length;
  const noSpend = upto - spendDays;
  const avgOnSpendDays = spendDays ? Math.round(spent.reduce((a, b) => a + b, 0) / spendDays) : 0;

  const iso = (d: number) => `${data.curMonth}-${String(d).padStart(2, "0")}`;

  return (
    <div>
      <div className="grid grid-cols-7 gap-[2px] mb-[2px]">
        {WD.map((w, i) => (
          <span
            key={w}
            className={`label-mono text-center py-1 ${i === 0 ? "!text-rose-400" : i === 6 ? "!text-sky-400" : ""}`}
          >
            {w}
          </span>
        ))}
      </div>
      <div className="grid grid-cols-7 gap-[2px]">
        {Array.from({ length: offset }, (_, i) => (
          <span key={`b${i}`} />
        ))}
        {daily.map((amt, i) => {
          const d = i + 1;
          const wd = (offset + i) % 7;
          const date = iso(d);
          const holiday = holidays[date];
          const future = d > upto;
          const lv = amt > 0 ? Math.sqrt(amt / max) : 0;
          const label = `${m}월 ${d}일 (${WD[wd]})`;
          const numColor = wd === 0 || holiday ? "text-rose-400" : wd === 6 ? "text-sky-400" : "text-zinc-300";
          const ring =
            inProgress && d === upto
              ? "ring-2 ring-mint-400"
              : d === topDay && amt > 0
              ? "ring-1 ring-amber-400"
              : "";
          return (
            <button
              key={d}
              type="button"
              disabled={future || amt === 0}
              className={`relative flex flex-col items-start justify-between gap-0.5 min-h-[58px] p-1.5 rounded-[4px] text-left transition-colors ${
                amt > 0 ? "hover:brightness-125 cursor-pointer" : "bg-white/[0.03] cursor-default"
              } ${future ? "opacity-40" : ""} ${ring}`}
              style={amt > 0 ? { background: cv("mint-400", Number((0.08 + 0.5 * lv).toFixed(2))) } : undefined}
              title={`${label}${holiday ? ` · ${holiday}` : ""}${amt > 0 ? ` · ${won(amt)}` : future ? "" : " · 지출 없음"}`}
              onClick={() => onPickDay(date, label)}
            >
              <span className={`num text-[11px] font-medium ${numColor}`}>{d}</span>
              {holiday && (
                <span className="text-[9px] leading-none text-rose-400 truncate max-w-full">
                  {holiday.replace(/\(.*\)/, "")}
                </span>
              )}
              {amt > 0 && <span className="num text-[11px] font-semibold text-white">{compact(amt)}</span>}
            </button>
          );
        })}
      </div>
      <div className="flex flex-wrap gap-x-5 gap-y-1 mt-4 text-xs text-zinc-500">
        <span>
          쓴 날 <strong className="num font-semibold text-white">{spendDays}일</strong> · 무지출{" "}
          <strong className="num font-semibold text-white">{noSpend}일</strong>
        </span>
        {spendDays > 0 && (
          <span>
            가장 많이 쓴 날 <strong className="num font-semibold text-white">{m}/{topDay}</strong>{" "}
            <span className="num">{won(spent[topDay - 1])}</span>
          </span>
        )}
        {spendDays > 0 && (
          <span>
            쓴 날 하루 평균 <strong className="num font-semibold text-white">{won(avgOnSpendDays)}</strong>
          </span>
        )}
      </div>
    </div>
  );
}
