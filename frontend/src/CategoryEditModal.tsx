import { FormEvent, useEffect, useState } from "react";
import { Pencil, X } from "lucide-react";
import { RenameCategory } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import Portal from "./components/ui/Portal";

const KIND_LABEL: Record<string, string> = { income: "수입", expense: "지출", transfer: "이체" };

// 카테고리 수정 팝업 — 이름을 바꾼다. 거래·규칙·예산은 카테고리 id 로 이어져 있어 그대로 따라온다.
// 상위(주) 이동은 목록의 선택 상자에서, 순서는 끌어서 바꾼다.
export default function CategoryEditModal({
  cat,
  subCount,
  onClose,
  onDone,
}: {
  cat: store.Category;
  subCount: number;
  onClose: () => void;
  onDone: () => void;
}) {
  const [name, setName] = useState(cat.name);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const trimmed = name.trim();
  const changed = trimmed !== "" && trimmed !== cat.name;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!changed || busy) return;
    setBusy(true);
    setErr("");
    try {
      await RenameCategory(cat.id, trimmed);
      onDone();
    } catch (ex) {
      setErr(String(ex));
      setBusy(false);
    }
  };

  const where = cat.parent
    ? `${cat.parent} 아래 부 카테고리`
    : `${KIND_LABEL[cat.kind] ?? cat.kind} 주 카테고리${subCount > 0 ? ` · 부 ${subCount}개` : ""}`;

  return (
    <Portal>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 backdrop-blur-sm sm:p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget) onClose();
        }}
      >
        <form
          role="dialog"
          aria-modal="true"
          aria-label={`${cat.fullName} 수정`}
          onSubmit={submit}
          className="surface w-full max-w-md space-y-4 p-5 sm:p-6"
        >
          <div className="flex items-start gap-3">
            <span className="icon-tile icon-tile-mint !h-9 !w-9 shrink-0 !rounded-xl" aria-hidden="true">
              <Pencil className="h-[18px] w-[18px]" strokeWidth={1.75} />
            </span>
            <div className="min-w-0 flex-1">
              <h3 className="section-title">카테고리 수정</h3>
              <p className="mt-1.5 break-words text-sm text-zinc-400">{where}</p>
            </div>
            <button type="button" className="btn-icon" aria-label="닫기" onClick={onClose}>
              <X className="h-4 w-4" />
            </button>
          </div>
          <label className="block">
            <span className="field-label">이름</span>
            <input
              type="text"
              className="input-field"
              value={name}
              autoFocus
              onFocus={(e) => e.target.select()}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <p className="text-xs text-zinc-500">
            이 카테고리로 분류된 거래·규칙·예산은 새 이름으로 그대로 이어집니다.
          </p>
          {/* 자금 계획은 이름이 "급여"인 카테고리로 월급을 찾는다(cashplan.go) */}
          {cat.name === "급여" && changed && (
            <p className="text-xs text-amber-300">
              자금 계획은 '급여' 카테고리로 월급을 찾습니다. 이름을 바꾸면 월급을 전체 수입 평균으로 계산합니다.
            </p>
          )}
          {err && <p className="text-sm text-rose-300">{err}</p>}
          <div className="flex items-center justify-end gap-2">
            <button type="button" className="btn-ghost" onClick={onClose}>
              취소
            </button>
            <button type="submit" className="btn-primary" disabled={!changed || busy}>
              {busy ? "저장 중…" : "저장"}
            </button>
          </div>
        </form>
      </div>
    </Portal>
  );
}
