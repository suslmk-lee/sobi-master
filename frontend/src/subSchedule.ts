// 정기결제 결제 일정 표기 도우미. 백엔드의 dayRule/weekNth/weekday/holidayRule 계약을 한글로 풀어 쓴다.

export const NTH_LABELS: { value: number; label: string }[] = [
  { value: 1, label: "첫째" },
  { value: 2, label: "둘째" },
  { value: 3, label: "셋째" },
  { value: 4, label: "넷째" },
  { value: 5, label: "다섯째" },
  { value: -1, label: "마지막" },
];

// 월요일부터 보여 주되 값은 0=일 … 6=토.
export const WEEKDAY_OPTIONS: { value: number; label: string }[] = [
  { value: 1, label: "월" },
  { value: 2, label: "화" },
  { value: 3, label: "수" },
  { value: 4, label: "목" },
  { value: 5, label: "금" },
  { value: 6, label: "토" },
  { value: 0, label: "일" },
];

export const HOLIDAY_RULES: { value: string; label: string }[] = [
  { value: "next", label: "다음 영업일" },
  { value: "prev", label: "이전 영업일" },
  { value: "none", label: "그대로" },
];

export interface ScheduleLike {
  cycle?: string;
  dayRule?: string;
  billingDay?: number;
  billingMonth?: number;
  weekNth?: number;
  weekday?: number;
  holidayRule?: string;
}

export function describeSchedule(s: ScheduleLike): string {
  const yearly = s.cycle === "yearly";
  const head = yearly ? `매년 ${s.billingMonth || 1}월` : "매월";
  if (s.dayRule === "weekday") {
    const nth = NTH_LABELS.find((n) => n.value === (s.weekNth || 1))?.label ?? "첫째";
    const wd = WEEKDAY_OPTIONS.find((w) => w.value === (s.weekday ?? 5))?.label ?? "금";
    return nth === "마지막" ? `${head} 마지막 주 ${wd}요일` : `${head} ${nth} 주 ${wd}요일`;
  }
  return `${head} ${s.billingDay || 1}일`;
}

export function describeHolidayRule(rule?: string): string {
  if (rule === "prev") return "휴일이면 이전 영업일";
  if (rule === "none") return "휴일에도 그대로";
  return "휴일이면 다음 영업일";
}

// 자동 등록 안내 문구에 쓰는 짧은 형태.
export function holidayHint(rule?: string): string {
  if (rule === "prev") return "주말·공휴일이면 이전 업무일";
  if (rule === "none") return "주말·공휴일이어도 그 날짜 그대로";
  return "주말·공휴일이면 다음 업무일";
}
