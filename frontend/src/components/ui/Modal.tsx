import { FormEvent, ReactNode, useEffect } from "react";
import type { LucideIcon } from "lucide-react";
import { X } from "lucide-react";
import IconTile from "./IconTile";
import Portal from "./Portal";

// 팝업 공통 틀: Portal + Esc/바깥 클릭/X 로 닫기. onSubmit 을 주면 form 으로 감싼다.
export default function Modal({
  title,
  icon: Icon,
  tone = "mint",
  onClose,
  wide,
  children,
  onSubmit,
}: {
  title: string;
  icon: LucideIcon;
  tone?: "mint" | "sky" | "amber" | "warm";
  onClose: () => void;
  wide?: boolean;
  children: ReactNode;
  onSubmit?: (e: FormEvent) => void;
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  const cls = `surface max-h-[92vh] w-full ${wide ? "max-w-3xl" : "max-w-xl"} overflow-y-auto p-5 sm:p-6`;
  const inner = (
    <>
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={Icon} tone={tone} size="sm" />
          <h3 className="section-title truncate">{title}</h3>
        </div>
        <button type="button" onClick={onClose} className="btn-icon shrink-0" aria-label="닫기" title="닫기 (Esc)">
          <X className="h-4 w-4" />
        </button>
      </div>
      {children}
    </>
  );
  return (
    <Portal>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 backdrop-blur-sm sm:p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget) onClose();
        }}
      >
        {onSubmit ? (
          <form role="dialog" aria-modal="true" aria-label={title} className={cls} onSubmit={onSubmit}>
            {inner}
          </form>
        ) : (
          <div role="dialog" aria-modal="true" aria-label={title} className={cls}>
            {inner}
          </div>
        )}
      </div>
    </Portal>
  );
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { v: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="glass-pill inline-flex items-center gap-1 p-1">
      {options.map((o) => (
        <button
          key={o.v}
          type="button"
          className={`pill-item ${value === o.v ? "pill-item-active" : ""}`}
          onClick={() => onChange(o.v)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Field({ label, children, className = "" }: { label: string; children: ReactNode; className?: string }) {
  return (
    <label className={`block ${className}`}>
      <span className="field-label">{label}</span>
      {children}
    </label>
  );
}
