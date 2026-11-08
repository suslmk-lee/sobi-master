import { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import IconTile from "./IconTile";

// 통계 화면들(전체 통계·카테고리 상세·구성원 분석)이 같이 쓰는 작은 부품.

// 세그먼트 토글 버튼 그룹
export function Seg<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { key: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="glass-pill flex gap-1 overflow-x-auto p-1" role="group">
      {options.map((o) => (
        <button
          key={o.key}
          className={`pill-item ${value === o.key ? "pill-item-active" : ""}`}
          aria-pressed={value === o.key}
          onClick={() => onChange(o.key)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

// 카드 내부 섹션 헤더 (IconTile + 제목 + 부제 + 우측 컨트롤)
export function SectionHead({
  icon,
  title,
  sub,
  children,
}: {
  icon: LucideIcon;
  title: string;
  sub?: string;
  children?: ReactNode;
}) {
  return (
    <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <IconTile icon={icon} tone="mint" size="sm" />
        <div className="min-w-0">
          <h3 className="section-title">{title}</h3>
          {sub && <p className="label-mono mt-0.5 !normal-case">{sub}</p>}
        </div>
      </div>
      {children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
    </div>
  );
}

// 증감률 표시 (늘면 rose ▲, 줄면 emerald ▼ — 지출 기준)
export function PctDelta({ pct, suffix }: { pct: number; suffix?: string }) {
  const color = pct > 0 ? "text-rose-300" : pct < 0 ? "text-emerald-300" : "text-zinc-300";
  return (
    <span className={`num text-xs ${color}`}>
      {pct > 0 ? "▲" : pct < 0 ? "▼" : ""} {Math.abs(pct)}%
      {suffix && <span className="text-zinc-500"> {suffix}</span>}
    </span>
  );
}
