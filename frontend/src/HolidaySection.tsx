import { FormEvent, useCallback, useEffect, useState } from "react";
import { CalendarDays, ChevronLeft, ChevronRight, Plus, RotateCcw, X } from "lucide-react";
import { DeleteCustomHoliday, GetHolidays, SaveCustomHoliday } from "../wailsjs/go/main/App";
import { holiday, main } from "../wailsjs/go/models";
import IconTile from "./components/ui/IconTile";

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
    <div className="surface p-5 sm:p-6">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <IconTile icon={CalendarDays} tone="amber" size="sm" />
          <h3 className="section-title">공휴일</h3>
        </div>
        <div className="glass-pill flex items-center gap-1 p-1">
          <button
            type="button"
            className="btn-icon"
            aria-label="이전 해"
            onClick={() => setYear((y) => y - 1)}
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
          <strong className="num px-2 text-sm font-medium text-white">{year}년</strong>
          <button
            type="button"
            className="btn-icon"
            aria-label="다음 해"
            onClick={() => setYear((y) => y + 1)}
          >
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      </div>
      <p className="text-xs text-zinc-500">
        정기결제 자동 등록에서 결제일이 주말이나 여기 있는 날이면 다음 업무일로 미룹니다.
        임시공휴일이 새로 정해지면 아래에서 추가하고, 표가 틀린 날은 "업무일로 정정"하세요.
        근로자의 날(5/1)은 법정 공휴일이 아니라 넣지 않았습니다 — 은행 휴무로 결제일이 밀린다면 추가하세요.
      </p>

      {data && !data.covered && (
        <div className="mt-3 rounded-xl border border-amber-300/30 bg-amber-400/10 p-3">
          <strong className="block text-sm font-medium text-amber-200">
            {year}년 공휴일 표가 없습니다
          </strong>
          <span className="text-xs text-zinc-400">
            양력 고정 공휴일만 적용됩니다. 설·추석·부처님오신날과 대체공휴일은 직접 추가해야 결제일이 맞게 밀립니다.
          </span>
        </div>
      )}
      {err && <p className="mt-3 text-xs text-rose-300">{err}</p>}

      <ul className="mt-3 divide-y divide-white/[0.05]">
        {entries.map((e) => (
          <li
            key={e.date}
            className={`flex flex-wrap items-center gap-3 py-2.5 ${e.off ? "opacity-60" : ""}`}
          >
            <span className="num w-32 shrink-0 text-sm text-zinc-300">{label(e.date)}</span>
            <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5 text-sm text-zinc-100">
              <span className={e.off ? "line-through" : ""}>{e.name || "공휴일"}</span>
              {!e.builtin && <span className="badge bg-mint-500/10 text-mint-300">직접 추가</span>}
              {e.off && <span className="badge bg-white/[0.06] text-zinc-400">업무일로 정정됨</span>}
              {isWeekend(e.date) && !e.off && (
                <span className="text-xs text-zinc-500"> · 주말과 겹침</span>
              )}
            </span>
            {e.builtin ? (
              e.off ? (
                <button
                  type="button"
                  className="btn-ghost !px-3 !py-1.5 !text-xs"
                  onClick={() => run(() => DeleteCustomHoliday(e.date))}
                >
                  <RotateCcw className="h-3 w-3" />
                  되돌리기
                </button>
              ) : (
                <button
                  type="button"
                  className="btn-ghost !px-3 !py-1.5 !text-xs"
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
              <button
                type="button"
                className="btn-icon hover:!border-rose-400/40 hover:!text-rose-300"
                title="삭제"
                aria-label="삭제"
                onClick={() => run(() => DeleteCustomHoliday(e.date))}
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </li>
        ))}
      </ul>

      <form className="mt-4 flex flex-wrap items-end gap-2" onSubmit={add}>
        <input
          type="date"
          className="input-field num !w-auto"
          aria-label="날짜"
          value={date}
          onChange={(e) => setDate(e.target.value)}
        />
        <input
          type="text"
          className="input-field !w-auto min-w-[12rem] flex-1"
          placeholder="이름 (예: 임시공휴일)"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <button type="submit" className="btn-primary">
          <Plus className="h-4 w-4" />
          공휴일 추가
        </button>
      </form>
    </div>
  );
}

