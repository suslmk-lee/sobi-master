import { HOLIDAY_RULES, NTH_LABELS, WEEKDAY_OPTIONS, describeHolidayRule, describeSchedule } from "./subSchedule";

// 정기결제 결제 일정 편집기. 문자열 상태(폼용)를 받아 부분 변경(patch)만 올려 보낸다.
export interface ScheduleValue {
  cycle: string;
  dayRule: string;
  billingDay: string;
  billingMonth: string;
  weekNth: string;
  weekday: string;
  holidayRule: string;
}

export const SCHEDULE_DEFAULTS = {
  dayRule: "day",
  weekNth: "1",
  weekday: "5",
  holidayRule: "next",
};

// 백엔드 값을 폼 상태로. 비어 있으면 기본값.
export function scheduleFromSub(s: {
  dayRule?: string;
  weekNth?: number;
  weekday?: number;
  holidayRule?: string;
}) {
  const byWeekday = s.dayRule === "weekday";
  return {
    dayRule: byWeekday ? "weekday" : "day",
    weekNth: String(s.weekNth ? s.weekNth : 1),
    weekday: String(byWeekday && s.weekday != null ? s.weekday : 5),
    holidayRule: s.holidayRule === "prev" || s.holidayRule === "none" ? s.holidayRule : "next",
  };
}

export default function ScheduleFields({
  value,
  onChange,
}: {
  value: ScheduleValue;
  onChange: (patch: Partial<ScheduleValue>) => void;
}) {
  const v = value;
  const byWeekday = v.dayRule === "weekday";
  const preview = describeSchedule({
    cycle: v.cycle,
    dayRule: v.dayRule,
    billingDay: Number(v.billingDay) || 1,
    billingMonth: Number(v.billingMonth) || 1,
    weekNth: Number(v.weekNth) || 1,
    weekday: Number(v.weekday),
  });

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div className="block">
          <span className="field-label">결제일 방식</span>
          <div className="glass-pill inline-flex gap-1 p-1" role="group" aria-label="결제일 방식">
            {[
              { k: "day", l: "날짜" },
              { k: "weekday", l: "요일" },
            ].map((o) => (
              <button
                key={o.k}
                type="button"
                aria-pressed={v.dayRule === o.k}
                className={`pill-item ${v.dayRule === o.k ? "pill-item-active" : ""}`}
                onClick={() => onChange({ dayRule: o.k })}
              >
                {o.l}
              </button>
            ))}
          </div>
        </div>

        {v.cycle === "yearly" && (
          <label className="block">
            <span className="field-label">결제 월</span>
            <select
              className="input-field"
              value={v.billingMonth}
              onChange={(e) => onChange({ billingMonth: e.target.value })}
            >
              {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
                <option key={m} value={m}>{m}월</option>
              ))}
            </select>
          </label>
        )}

        {!byWeekday ? (
          <label className="block">
            <span className="field-label">결제일</span>
            <select
              className="input-field num"
              value={v.billingDay}
              onChange={(e) => onChange({ billingDay: e.target.value })}
            >
              {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                <option key={d} value={d}>{d}일</option>
              ))}
            </select>
            <span className="mt-1 block text-xs text-zinc-500">말일보다 크면 말일</span>
          </label>
        ) : (
          <>
            <label className="block">
              <span className="field-label">몇째 주</span>
              <select
                className="input-field"
                value={v.weekNth}
                onChange={(e) => onChange({ weekNth: e.target.value })}
              >
                {NTH_LABELS.map((n) => (
                  <option key={n.value} value={n.value}>{n.label}</option>
                ))}
              </select>
              {v.weekNth === "5" && (
                <span className="mt-1 block text-xs text-zinc-500">다섯째 주가 없는 달은 마지막 주</span>
              )}
            </label>
            <label className="block">
              <span className="field-label">요일</span>
              <select
                className="input-field"
                value={v.weekday}
                onChange={(e) => onChange({ weekday: e.target.value })}
              >
                {WEEKDAY_OPTIONS.map((w) => (
                  <option key={w.value} value={w.value}>{w.label}요일</option>
                ))}
              </select>
            </label>
          </>
        )}
      </div>

      <div className="block">
        <span className="field-label">휴일 처리</span>
        <div className="glass-pill inline-flex flex-wrap gap-1 p-1" role="group" aria-label="휴일 처리">
          {HOLIDAY_RULES.map((o) => (
            <button
              key={o.value}
              type="button"
              aria-pressed={v.holidayRule === o.value}
              className={`pill-item ${v.holidayRule === o.value ? "pill-item-active" : ""}`}
              onClick={() => onChange({ holidayRule: o.value })}
            >
              {o.label}
            </button>
          ))}
        </div>
      </div>

      <p className="label-mono">
        {preview} · {describeHolidayRule(v.holidayRule)}
      </p>
    </div>
  );
}
