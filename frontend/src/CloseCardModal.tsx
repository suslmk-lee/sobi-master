import { FormEvent, useEffect, useState } from "react";
import { Archive, X } from "lucide-react";
import { SetPaymentMethodClosed } from "../wailsjs/go/main/App";
import { today } from "./lib";
import Portal from "./components/ui/Portal";

// 결제수단 해지 모달 — 해지일을 받아 SetPaymentMethodClosed 로 보관 처리한다.
// 카드 탭과 설정의 결제수단 목록에서 같이 쓴다.
export default function CloseCardModal({
  id,
  name,
  onClose,
  onDone,
}: {
  id: number;
  name: string;
  onClose: () => void;
  onDone: () => void;
}) {
  const [date, setDate] = useState(today());
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!date || busy) return;
    setBusy(true);
    setErr("");
    try {
      await SetPaymentMethodClosed(id, date);
      onDone();
    } catch (ex) {
      setErr(String(ex));
      setBusy(false);
    }
  };

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
          aria-label={`${name} 해지`}
          onSubmit={submit}
          className="surface w-full max-w-md space-y-4 p-5 sm:p-6"
        >
          <div className="flex items-start gap-3">
            <span
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-amber-400/30 bg-amber-500/10 text-amber-300"
              aria-hidden="true"
            >
              <Archive className="h-[18px] w-[18px]" strokeWidth={1.75} />
            </span>
            <div className="min-w-0 flex-1">
              <h3 className="section-title">"{name}" 해지</h3>
              <p className="mt-1.5 break-words text-sm text-zinc-400">
                과거 거래와 혜택 자료는 그대로 남고, 새 거래를 고를 때 목록에서 빠집니다. 해지 전에 쓴
                카드값은 자금 계획에 계속 보입니다.
              </p>
            </div>
            <button type="button" className="btn-icon" aria-label="닫기" onClick={onClose}>
              <X className="h-4 w-4" />
            </button>
          </div>
          <label className="block">
            <span className="field-label">해지일</span>
            <input
              type="date"
              className="input-field"
              value={date}
              autoFocus
              onChange={(e) => setDate(e.target.value)}
            />
          </label>
          {err && <p className="text-sm text-rose-300">{err}</p>}
          <div className="flex items-center justify-end gap-2">
            <button type="button" className="btn-ghost" onClick={onClose}>
              취소
            </button>
            <button type="submit" className="btn-primary" disabled={!date || busy}>
              해지
            </button>
          </div>
        </form>
      </div>
    </Portal>
  );
}
