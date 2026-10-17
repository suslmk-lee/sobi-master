import { useEffect, useState } from "react";
import { GetHolidays } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { compact } from "./charts";

// 지출 달력 — 이번 달을 달력으로 펼쳐 날마다 쓴 금액을 색 진하기로 보여 준다.
// 요일별 평균("언제 쓰나")을 날짜 단위로 펼친 것이라 같은 일~토 7칸 구조로 둔다.
// 날짜를 누르면 그날 쓴 내역이 열린다(onPickDay).
//
// 일별 금액은 전월 비교용 누적 데이터의 하루 차이로 구한다 — 따로 조회하지 않는다.
// 색 진하기는 제곱근 척도: 큰 날 하루가 나머지를 모두 흐리게 만들지 않게 한다.

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
    <div className="spend-cal">
      <div className="sc-week">
        {WD.map((w, i) => (
          <span key={w} className={i === 0 ? "sun" : i === 6 ? "sat" : ""}>{w}</span>
        ))}
      </div>
      <div className="sc-grid">
        {Array.from({ length: offset }, (_, i) => (
          <span key={`b${i}`} className="sc-blank" />
        ))}
        {daily.map((amt, i) => {
          const d = i + 1;
          const wd = (offset + i) % 7;
          const date = iso(d);
          const holiday = holidays[date];
          const future = d > upto;
          const lv = amt > 0 ? Math.sqrt(amt / max) : 0;
          const cls = [
            "sc-day",
            future ? "future" : "",
            amt > 0 ? "has" : "",
            d === topDay && amt > 0 ? "top" : "",
            inProgress && d === upto ? "today" : "",
            wd === 0 || holiday ? "red" : wd === 6 ? "blue" : "",
          ]
            .filter(Boolean)
            .join(" ");
          const label = `${m}월 ${d}일 (${WD[wd]})`;
          return (
            <button
              key={d}
              className={cls}
              disabled={future || amt === 0}
              style={amt > 0 ? { background: `rgba(91, 108, 255, ${0.07 + 0.5 * lv})` } : undefined}
              title={`${label}${holiday ? ` · ${holiday}` : ""}${amt > 0 ? ` · ${won(amt)}` : future ? "" : " · 지출 없음"}`}
              onClick={() => onPickDay(date, label)}
            >
              <span className="sc-num">{d}</span>
              {holiday && <span className="sc-hol">{holiday.replace(/\(.*\)/, "")}</span>}
              {amt > 0 && <span className="sc-amt">{compact(amt)}</span>}
            </button>
          );
        })}
      </div>
      <div className="sc-foot">
        <span>
          쓴 날 <strong>{spendDays}일</strong> · 무지출 <strong>{noSpend}일</strong>
        </span>
        {spendDays > 0 && (
          <span>
            가장 많이 쓴 날 <strong>{m}/{topDay}</strong> {won(spent[topDay - 1])}
          </span>
        )}
        {spendDays > 0 && (
          <span>
            쓴 날 하루 평균 <strong>{won(avgOnSpendDays)}</strong>
          </span>
        )}
      </div>
    </div>
  );
}
