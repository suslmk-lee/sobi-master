import { useEffect, useRef, useState } from "react";

// 월 선택기.
//
// <input type="month"> 는 Chromium 계열(Windows 의 WebView2)에서만 달력이 뜬다.
// macOS 의 WebView(WebKit)는 이 타입을 지원하지 않아 값만 보이는 텍스트 칸으로 떨어져
// 월을 고를 수단이 사라진다. 그래서 직접 만들어 어느 플랫폼에서나 같게 동작하게 한다.
//
// 이전·다음 화살표로 한 달씩 넘기고, 가운데 라벨을 누르면 연도 이동 + 12개월 격자가
// 열려 몇 달 떨어진 달도 한 번에 고를 수 있다.
//
// value 는 "YYYY-MM", onChange 도 같은 형식으로 돌려준다.

const pad2 = (n: number) => String(n).padStart(2, "0");
const ymOf = (y: number, m: number) => `${y}-${pad2(m)}`;

function parseYm(value: string): { y: number; m: number } {
  const y = Number(value.slice(0, 4));
  const m = Number(value.slice(5, 7));
  if (!y || !m || m < 1 || m > 12) {
    const d = new Date(); // 값이 비었거나 깨졌으면 이번 달로 본다
    return { y: d.getFullYear(), m: d.getMonth() + 1 };
  }
  return { y, m };
}

const MONTHS = Array.from({ length: 12 }, (_, i) => i + 1);

export default function MonthPicker({
  value,
  onChange,
  className,
}: {
  value: string;
  onChange: (ym: string) => void;
  className?: string;
}) {
  const { y, m } = parseYm(value);
  const [open, setOpen] = useState(false);
  // 격자에서 보고 있는 연도(선택한 연도와 별개로 넘겨볼 수 있다)
  const [panelYear, setPanelYear] = useState(y);
  const boxRef = useRef<HTMLDivElement>(null);

  // 열 때마다 현재 선택 연도에서 시작한다
  useEffect(() => {
    if (open) setPanelYear(y);
  }, [open, y]);

  // 바깥 클릭 / Esc 로 닫는다
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  // delta 개월 이동. 12월 → 다음 해 1월 같은 연도 넘김은 Date 가 알아서 처리한다.
  const shift = (delta: number) => {
    const d = new Date(y, m - 1 + delta, 1);
    onChange(ymOf(d.getFullYear(), d.getMonth() + 1));
  };

  const now = new Date();
  const thisY = now.getFullYear();
  const thisM = now.getMonth() + 1;
  const isThisMonth = y === thisY && m === thisM;

  const pick = (mm: number) => {
    onChange(ymOf(panelYear, mm));
    setOpen(false);
  };

  return (
    <div className={className ? `month-picker ${className}` : "month-picker"} ref={boxRef}>
      <button
        type="button"
        className="mp-arrow"
        onClick={() => shift(-1)}
        aria-label="이전 달"
        title="이전 달"
      >
        ‹
      </button>

      <button
        type="button"
        className={`mp-label ${open ? "open" : ""}`}
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-haspopup="dialog"
        title="달 고르기"
      >
        <span className="mp-y">{y}년</span>
        <span className="mp-m">{m}월</span>
        {isThisMonth && <span className="mp-badge">이번 달</span>}
        <span className="mp-caret" aria-hidden="true">▾</span>
      </button>

      <button
        type="button"
        className="mp-arrow"
        onClick={() => shift(1)}
        aria-label="다음 달"
        title="다음 달"
      >
        ›
      </button>

      {open && (
        <div className="mp-pop" role="dialog" aria-label="달 선택">
          <div className="mp-pop-head">
            <button
              type="button"
              className="mp-arrow"
              onClick={() => setPanelYear((py) => py - 1)}
              aria-label="이전 해"
            >
              ‹
            </button>
            <strong>{panelYear}년</strong>
            <button
              type="button"
              className="mp-arrow"
              onClick={() => setPanelYear((py) => py + 1)}
              aria-label="다음 해"
            >
              ›
            </button>
          </div>

          <div className="mp-grid">
            {MONTHS.map((mm) => {
              const selected = panelYear === y && mm === m;
              const isNow = panelYear === thisY && mm === thisM;
              return (
                <button
                  key={mm}
                  type="button"
                  className={`mp-m-btn${selected ? " sel" : ""}${isNow ? " now" : ""}`}
                  onClick={() => pick(mm)}
                  title={isNow ? "이번 달" : undefined}
                >
                  {mm}월
                </button>
              );
            })}
          </div>

          <button
            type="button"
            className="mp-today"
            onClick={() => {
              onChange(ymOf(thisY, thisM));
              setOpen(false);
            }}
          >
            이번 달로 이동
          </button>
        </div>
      )}
    </div>
  );
}
