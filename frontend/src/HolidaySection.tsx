import { FormEvent, useCallback, useEffect, useState } from "react";
import { DeleteCustomHoliday, GetHolidays, SaveCustomHoliday } from "../wailsjs/go/main/App";
import { holiday, main } from "../wailsjs/go/models";

// 공휴일 설정. 정기결제 자동 등록이 결제일을 다음 업무일로 미룰 때 이 달력을 쓴다.
// 내장 표에는 음력 공휴일·대체공휴일·선거일까지 들어 있지만, 임시공휴일은 그때그때
// 정해지고 표가 틀릴 수도 있으니 여기서 날짜를 추가하거나 업무일로 정정한다.

const WD = ["일", "월", "화", "수", "목", "금", "토"];
const label = (d: string) => {
  const t = new Date(d + "T00:00:00");
  return `${t.getMonth() + 1}월 ${t.getDate()}일 (${WD[t.getDay()]})`;
};
const isWeekend = (d: string) => {
  const w = new Date(d + "T00:00:00").getDay();
  return w === 0 || w === 6;
};

export default function HolidaySection() {
  const [year, setYear] = useState(new Date().getFullYear());
  const [data, setData] = useState<main.HolidayYear | null>(null);
  const [date, setDate] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    try {
      setData(await GetHolidays(year));
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    }
  }, [year]);

  useEffect(() => {
    load();
  }, [load]);

  const run = async (fn: () => Promise<void>) => {
    setErr("");
    try {
      await fn();
      await load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const add = (e: FormEvent) => {
    e.preventDefault();
    if (!date) return setErr("날짜를 고르세요.");
    run(async () => {
      await SaveCustomHoliday(
        new holiday.Override({ date, name: name.trim() || "임시공휴일", isHoliday: true })
      );
      setDate("");
      setName("");
    });
  };

  const entries = data?.entries ?? [];

  return (
    <div className="card">
      <div className="stats-head">
        <h3>공휴일</h3>
        <div className="seg-row">
          <button className="ghost" onClick={() => setYear((y) => y - 1)}>‹</button>
          <strong>{year}년</strong>
          <button className="ghost" onClick={() => setYear((y) => y + 1)}>›</button>
        </div>
      </div>
      <p className="muted small">
        정기결제 자동 등록에서 결제일이 주말이나 여기 있는 날이면 다음 업무일로 미룹니다.
        임시공휴일이 새로 정해지면 아래에서 추가하고, 표가 틀린 날은 "업무일로 정정"하세요.
        근로자의 날(5/1)은 법정 공휴일이 아니라 넣지 않았습니다 — 은행 휴무로 결제일이 밀린다면 추가하세요.
      </p>

      {data && !data.covered && (
        <div className="alert warn">
          <strong>{year}년 공휴일 표가 없습니다</strong>
          <span className="muted small">
            양력 고정 공휴일만 적용됩니다. 설·추석·부처님오신날과 대체공휴일은 직접 추가해야 결제일이 맞게 밀립니다.
          </span>
        </div>
      )}
      {err && <p className="error">{err}</p>}

      <ul className="item-list holiday-list">
        {entries.map((e) => (
          <li key={e.date} className={e.off ? "off" : ""}>
            <span className="hl-date">{label(e.date)}</span>
            <span className="hl-name">
              {e.name || "공휴일"}
              {!e.builtin && <span className="badge auto">직접 추가</span>}
              {e.off && <span className="badge">업무일로 정정됨</span>}
              {isWeekend(e.date) && !e.off && <span className="muted small"> · 주말과 겹침</span>}
            </span>
            {e.builtin ? (
              e.off ? (
                <button className="ghost" onClick={() => run(() => DeleteCustomHoliday(e.date))}>
                  되돌리기
                </button>
              ) : (
                <button
                  className="ghost"
                  title="이 날을 공휴일에서 빼고 업무일로 봅니다"
                  onClick={() =>
                    run(() =>
                      SaveCustomHoliday(new holiday.Override({ date: e.date, name: e.name, isHoliday: false }))
                    )
                  }
                >
                  업무일로 정정
                </button>
              )
            ) : (
              <button className="ghost" title="삭제" onClick={() => run(() => DeleteCustomHoliday(e.date))}>
                ✕
              </button>
            )}
          </li>
        ))}
      </ul>

      <form className="form-row" onSubmit={add}>
        <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        <input
          type="text"
          placeholder="이름 (예: 임시공휴일)"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <button type="submit">공휴일 추가</button>
      </form>
    </div>
  );
}
