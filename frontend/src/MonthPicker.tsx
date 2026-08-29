import { useMemo } from "react";

// 월 선택기.
//
// 원래는 <input type="month"> 를 썼는데, 이건 Chromium 계열(Windows 의 WebView2)에서만
// 달력이 뜬다. macOS 의 WebView(WebKit)는 이 타입을 지원하지 않아 값만 보이는 텍스트
// 칸으로 떨어져 월을 고를 수단이 사라진다. 그래서 연/월 select 와 이전·다음 버튼으로
// 직접 만들어 어느 플랫폼에서나 같게 동작하게 한다.
//
// value 는 "YYYY-MM", onChange 도 같은 형식으로 돌려준다(기존 input 과 호환).

const pad2 = (n: number) => String(n).padStart(2, "0");
const ymOf = (y: number, m: number) => `${y}-${pad2(m)}`;

// 과거 연도를 얼마나 거슬러 고를 수 있게 할지(내년까지는 미리 선택 가능).
const YEARS_BACK = 10;

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

  // 연도 후보. 지금 값이 범위 밖이면(오래된 데이터를 보는 중이면) 그 연도도 넣어 준다.
  const years = useMemo(() => {
    const now = new Date().getFullYear();
    const list: number[] = [];
    for (let i = now - YEARS_BACK; i <= now + 1; i++) list.push(i);
    if (!list.includes(y)) {
      list.push(y);
      list.sort((a, b) => a - b);
    }
    return list;
  }, [y]);

  // delta 개월 이동. 12월 → 다음 해 1월 같은 연도 넘김은 Date 가 알아서 처리한다.
  const shift = (delta: number) => {
    const d = new Date(y, m - 1 + delta, 1);
    onChange(ymOf(d.getFullYear(), d.getMonth() + 1));
  };

  return (
    <div className={className ? `month-picker ${className}` : "month-picker"}>
      <button
        type="button"
        className="mp-arrow"
        onClick={() => shift(-1)}
        aria-label="이전 달"
        title="이전 달"
      >
        ‹
      </button>
      <select
        value={y}
        onChange={(e) => onChange(ymOf(Number(e.target.value), m))}
        aria-label="연도"
      >
        {years.map((yy) => (
          <option key={yy} value={yy}>
            {yy}년
          </option>
        ))}
      </select>
      <select
        value={m}
        onChange={(e) => onChange(ymOf(y, Number(e.target.value)))}
        aria-label="월"
      >
        {MONTHS.map((mm) => (
          <option key={mm} value={mm}>
            {mm}월
          </option>
        ))}
      </select>
      <button
        type="button"
        className="mp-arrow"
        onClick={() => shift(1)}
        aria-label="다음 달"
        title="다음 달"
      >
        ›
      </button>
    </div>
  );
}
