import { ReactNode, RefObject, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { DayPicker } from "react-day-picker";
import { ko } from "react-day-picker/locale";
import "react-day-picker/style.css";
import "./date-input.css";
import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import { GetHolidays } from "../../../wailsjs/go/main/App";
import Portal from "./Portal";

// 날짜·월 입력.
//
// <input type="date"> 는 macOS 의 WebView(WebKit)에서 아주 작은 기본 달력이 뜨고, type="month" 는 아예
// 달력 없이 글자 칸으로 떨어진다. 그래서 칸을 누르면 큰 달력(react-day-picker)을 띄워 고르게 한다.
// 일요일·공휴일은 빨강, 토요일은 파랑으로 칠하고, 그 달 공휴일 이름을 달력 아래에 적는다.
//
// 값 형식은 기존 입력과 같다: 날짜 "YYYY-MM-DD", 월 "YYYY-MM", 비어 있으면 "".
// 달력은 body 에 붙여(Portal) 팝업의 스크롤 영역에 잘리지 않게 하고, 칸 아래(자리가 없으면 위)에 띄운다.

const pad2 = (n: number) => String(n).padStart(2, "0");
const ymd = (d: Date) => `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
const WEEKDAY = ["일", "월", "화", "수", "목", "금", "토"];

function parseDate(s: string | undefined): Date | undefined {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s ?? "");
  if (!m) return undefined;
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  return Number.isNaN(d.getTime()) ? undefined : d;
}

// ---------- 공휴일 ----------
// 연도별로 한 번 받아 잠깐 기억한다(공휴일 설정을 고치면 1분 안에 달력에도 반영된다).
const HOLIDAY_TTL = 60_000;
const holidayCache = new Map<number, { at: number; p: Promise<Record<string, string>> }>();

function holidaysOf(year: number): Promise<Record<string, string>> {
  const hit = holidayCache.get(year);
  if (hit && Date.now() - hit.at < HOLIDAY_TTL) return hit.p;
  const p = GetHolidays(year)
    .then((h) => {
      const map: Record<string, string> = {};
      for (const e of h.entries ?? []) if (!e.off) map[e.date] = e.name;
      return map;
    })
    .catch(() => {
      holidayCache.delete(year);
      return {} as Record<string, string>;
    });
  holidayCache.set(year, { at: Date.now(), p });
  return p;
}

function useHolidays(month: Date, active: boolean): Record<string, string> {
  const [map, setMap] = useState<Record<string, string>>({});
  const y = month.getFullYear();
  const m = month.getMonth();
  useEffect(() => {
    if (!active) return;
    // 달력에 앞뒤 달 날짜도 보이므로 1월·12월이면 이웃 해도 같이 받는다
    const years = [y];
    if (m === 0) years.push(y - 1);
    if (m === 11) years.push(y + 1);
    let alive = true;
    Promise.all(years.map(holidaysOf)).then((all) => {
      if (alive) setMap(Object.assign({}, ...all));
    });
    return () => {
      alive = false;
    };
  }, [y, m, active]);
  return map;
}

// ---------- 띄우기 ----------
// 칸 바로 아래(자리가 모자라면 위)에 화면 안으로 맞춰 띄운다. 스크롤·창 크기가 바뀌면 따라간다.
// 바깥을 누르거나 Esc 로 닫는다. Esc 는 여기서 멈춰 뒤의 팝업까지 닫히지 않게 한다.
function usePopover(
  open: boolean,
  anchor: RefObject<HTMLElement>,
  pop: RefObject<HTMLElement>,
  close: () => void
) {
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);

  useLayoutEffect(() => {
    if (!open) {
      setPos(null);
      return;
    }
    const place = () => {
      const a = anchor.current;
      const p = pop.current;
      if (!a || !p) return;
      const r = a.getBoundingClientRect();
      const w = p.offsetWidth;
      const h = p.offsetHeight;
      const gap = 6;
      const edge = 8;
      const vw = window.innerWidth;
      const vh = window.innerHeight;
      let top = r.bottom + gap;
      if (top + h > vh - edge && r.top - gap - h >= edge) top = r.top - gap - h;
      top = Math.max(edge, Math.min(top, vh - h - edge));
      const left = Math.max(edge, Math.min(r.left, vw - w - edge));
      setPos((old) => (old && old.top === top && old.left === left ? old : { top, left }));
    };
    place();
    const ro = new ResizeObserver(place);
    if (pop.current) ro.observe(pop.current);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
    };
  }, [open, anchor, pop]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (anchor.current?.contains(t) || pop.current?.contains(t)) return;
      close();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.stopPropagation();
      close();
      anchor.current?.focus();
    };
    document.addEventListener("mousedown", onDown, true);
    window.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("mousedown", onDown, true);
      window.removeEventListener("keydown", onKey, true);
    };
  }, [open, anchor, pop, close]);

  return pos;
}

function PopoverPanel({
  open,
  anchor,
  close,
  label,
  children,
}: {
  open: boolean;
  anchor: RefObject<HTMLElement>;
  close: () => void;
  label: string;
  children: ReactNode;
}) {
  const pop = useRef<HTMLDivElement>(null);
  const pos = usePopover(open, anchor, pop, close);
  if (!open) return null;
  return (
    <Portal>
      <div
        ref={pop}
        role="dialog"
        aria-label={label}
        className="surface sobi-cal fixed z-[60] p-3 shadow-2xl"
        style={{ top: pos?.top ?? 0, left: pos?.left ?? 0, visibility: pos ? "visible" : "hidden" }}
      >
        {children}
      </div>
    </Portal>
  );
}

function Trigger({
  btnRef,
  open,
  text,
  placeholder,
  disabled,
  autoFocus,
  ariaLabel,
  className,
  onClick,
}: {
  btnRef: RefObject<HTMLButtonElement>;
  open: boolean;
  text: string;
  placeholder: string;
  disabled?: boolean;
  autoFocus?: boolean;
  ariaLabel?: string;
  className?: string;
  onClick: () => void;
}) {
  return (
    <button
      ref={btnRef}
      type="button"
      className={`input-field num flex items-center justify-between gap-2 text-left disabled:cursor-not-allowed disabled:opacity-50 ${className ?? ""}`}
      disabled={disabled}
      autoFocus={autoFocus}
      aria-label={ariaLabel}
      aria-haspopup="dialog"
      aria-expanded={open}
      onClick={onClick}
    >
      <span className={`truncate ${text ? "" : "text-zinc-600"}`}>{text || placeholder}</span>
      <CalendarDays className={`h-4 w-4 shrink-0 ${open ? "text-mint-300" : "text-zinc-500"}`} aria-hidden="true" />
    </button>
  );
}

// ---------- 날짜 ----------
export default function DateInput({
  value,
  onChange,
  min,
  max,
  disabled,
  clearable,
  placeholder = "날짜 고르기",
  autoFocus,
  className,
  "aria-label": ariaLabel,
}: {
  value: string;
  onChange: (v: string) => void;
  min?: string;
  max?: string;
  disabled?: boolean;
  /** 비워 둘 수 있는 칸이면 '지우기'를 보여 준다 */
  clearable?: boolean;
  placeholder?: string;
  autoFocus?: boolean;
  className?: string;
  "aria-label"?: string;
}) {
  const btn = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const selected = parseDate(value);
  const minD = parseDate(min);
  const maxD = parseDate(max);
  const [month, setMonth] = useState<Date>(() => selected ?? new Date());
  const holidays = useHolidays(month, open);
  const close = useCallback(() => setOpen(false), []);

  // 열 때마다 고른 날(없으면 오늘)의 달에서 시작한다
  const toggle = () => {
    if (!open) setMonth(selected ?? new Date());
    setOpen((o) => !o);
  };
  const pick = (d: Date | undefined) => {
    if (!d) return; // 고른 날을 다시 눌러도 비우지 않는다(비우기는 '지우기'로)
    onChange(ymd(d));
    close();
    btn.current?.focus();
  };

  const now = new Date();
  const todayOff = (minD && ymd(now) < ymd(minD)) || (maxD && ymd(now) > ymd(maxD));
  const text = selected ? `${value} (${WEEKDAY[selected.getDay()]})` : "";

  const holidayDates = useMemo(() => Object.keys(holidays).map((s) => parseDate(s)!).filter(Boolean), [holidays]);
  const monthKey = `${month.getFullYear()}-${pad2(month.getMonth() + 1)}-`;
  const monthHolidays = Object.entries(holidays)
    .filter(([d]) => d.startsWith(monthKey))
    .sort(([a], [b]) => a.localeCompare(b));

  const disabledDays = [
    ...(minD ? [{ before: minD }] : []),
    ...(maxD ? [{ after: maxD }] : []),
  ];
  const startMonth = minD ?? new Date(now.getFullYear() - 20, 0, 1);
  const endMonth = maxD ?? new Date(now.getFullYear() + 10, 11, 1);

  return (
    <>
      <Trigger
        btnRef={btn}
        open={open}
        text={text}
        placeholder={placeholder}
        disabled={disabled}
        autoFocus={autoFocus}
        ariaLabel={ariaLabel}
        className={className}
        onClick={toggle}
      />
      <PopoverPanel open={open} anchor={btn} close={close} label="날짜 고르기">
        <DayPicker
          mode="single"
          locale={ko}
          weekStartsOn={0}
          selected={selected}
          onSelect={pick}
          month={month}
          onMonthChange={setMonth}
          captionLayout="dropdown"
          startMonth={startMonth}
          endMonth={endMonth}
          showOutsideDays
          fixedWeeks
          autoFocus
          disabled={disabledDays}
          modifiers={{ sunday: { dayOfWeek: [0] }, saturday: { dayOfWeek: [6] }, holiday: holidayDates }}
          modifiersClassNames={{ sunday: "sobi-red", holiday: "sobi-red", saturday: "sobi-blue" }}
          formatters={{ formatYearDropdown: (y) => `${y.getFullYear()}년` }}
        />
        <div className="sobi-cal-foot">
          <p className="min-w-0 flex-1 text-xs leading-relaxed text-rose-300/90">
            {monthHolidays.map(([d, name]) => `${Number(d.slice(8))}일 ${name}`).join(" · ")}
          </p>
          <div className="flex shrink-0 gap-1.5">
            {clearable && value && (
              <button
                type="button"
                className="btn-ghost !px-3 !py-1.5 text-xs"
                onClick={() => {
                  onChange("");
                  close();
                  btn.current?.focus();
                }}
              >
                지우기
              </button>
            )}
            <button type="button" className="btn-ghost !px-3 !py-1.5 text-xs" disabled={!!todayOff} onClick={() => pick(now)}>
              오늘
            </button>
          </div>
        </div>
      </PopoverPanel>
    </>
  );
}

// ---------- 월 ----------
const MONTHS = Array.from({ length: 12 }, (_, i) => i + 1);

export function MonthInput({
  value,
  onChange,
  disabled,
  placeholder = "월 고르기",
  className,
  "aria-label": ariaLabel,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
  className?: string;
  "aria-label"?: string;
}) {
  const btn = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const m = /^(\d{4})-(\d{2})$/.exec(value);
  const selY = m ? Number(m[1]) : 0;
  const selM = m ? Number(m[2]) : 0;
  const now = new Date();
  const [year, setYear] = useState(selY || now.getFullYear());
  const close = useCallback(() => setOpen(false), []);

  const toggle = () => {
    if (!open) setYear(selY || now.getFullYear());
    setOpen((o) => !o);
  };
  const pick = (y: number, mm: number) => {
    onChange(`${y}-${pad2(mm)}`);
    close();
    btn.current?.focus();
  };

  return (
    <>
      <Trigger
        btnRef={btn}
        open={open}
        text={selY ? `${selY}년 ${selM}월` : ""}
        placeholder={placeholder}
        disabled={disabled}
        ariaLabel={ariaLabel}
        className={className}
        onClick={toggle}
      />
      <PopoverPanel open={open} anchor={btn} close={close} label="월 고르기">
        <div className="w-72">
          <div className="mb-3 flex items-center justify-between">
            <button type="button" className="btn-icon" onClick={() => setYear((y) => y - 1)} aria-label="이전 해">
              <ChevronLeft className="h-4 w-4" />
            </button>
            <span className="num text-base font-semibold text-white">{year}년</span>
            <button type="button" className="btn-icon" onClick={() => setYear((y) => y + 1)} aria-label="다음 해">
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
          <div className="grid grid-cols-3 gap-1.5">
            {MONTHS.map((mm) => {
              const sel = year === selY && mm === selM;
              const cur = year === now.getFullYear() && mm === now.getMonth() + 1;
              return (
                <button
                  key={mm}
                  type="button"
                  autoFocus={sel}
                  onClick={() => pick(year, mm)}
                  className={`num sobi-cal-month ${sel ? "is-selected" : ""} ${cur ? "is-current" : ""}`}
                >
                  {mm}월
                </button>
              );
            })}
          </div>
          <div className="sobi-cal-foot">
            <span className="flex-1" />
            <button type="button" className="btn-ghost !px-3 !py-1.5 text-xs" onClick={() => pick(now.getFullYear(), now.getMonth() + 1)}>
              이번 달
            </button>
          </div>
        </div>
      </PopoverPanel>
    </>
  );
}
