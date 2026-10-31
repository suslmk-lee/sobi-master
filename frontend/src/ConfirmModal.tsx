import { ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { AlertTriangle, HelpCircle } from "lucide-react";
import Portal from "./components/ui/Portal";

// 확인 모달. 브라우저 기본 confirm() 은 운영체제 대화상자라 앱 디자인과 동떨어지고
// 무엇을 지우는지 보여 줄 수 없어서 따로 둔다.
//
//   const [confirm, confirmUI] = useConfirm();
//   if (!(await confirm({ title: "삭제할까요?", danger: true }))) return;
//   ...
//   return <>{...}{confirmUI}</>;
//
// Enter = 확인, Esc·바깥 클릭 = 취소.

export type ConfirmOpts = {
  title: string;
  message?: ReactNode; // 본문 설명
  detail?: ReactNode; // 지울 대상 요약(카드 모양으로 강조)
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean; // 되돌리기 어려운 동작이면 빨간 확인 버튼
};

export function useConfirm(): [(o: ConfirmOpts) => Promise<boolean>, JSX.Element | null] {
  const [opts, setOpts] = useState<ConfirmOpts | null>(null);
  const resolver = useRef<((v: boolean) => void) | null>(null);

  const confirm = useCallback(
    (o: ConfirmOpts) =>
      new Promise<boolean>((resolve) => {
        resolver.current = resolve;
        setOpts(o);
      }),
    []
  );

  const close = useCallback((v: boolean) => {
    resolver.current?.(v);
    resolver.current = null;
    setOpts(null);
  }, []);

  const ui = opts ? <ConfirmDialog opts={opts} onClose={close} /> : null;
  return [confirm, ui];
}

function ConfirmDialog({ opts, onClose }: { opts: ConfirmOpts; onClose: (v: boolean) => void }) {
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    boxRef.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose(false);
      } else if (e.key === "Enter" && !(e.target instanceof HTMLButtonElement)) {
        // 버튼에 포커스가 있으면(Tab 으로 '취소'를 고른 경우 등) 그 버튼의 기본 동작에 맡긴다
        e.preventDefault();
        onClose(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const danger = !!opts.danger;
  const Icon = danger ? AlertTriangle : HelpCircle;

  return (
    <Portal>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 backdrop-blur-sm sm:p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget) onClose(false);
        }}
      >
        <div
          ref={boxRef}
          tabIndex={-1}
          role="alertdialog"
          aria-modal="true"
          aria-label={opts.title}
          className="surface w-full max-w-md p-5 outline-none sm:p-6"
        >
          <div className="flex items-start gap-3">
            <span
              className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border ${
                danger
                  ? "border-rose-400/30 bg-rose-500/10 text-rose-300"
                  : "border-mint-400/30 bg-mint-500/10 text-mint-300"
              }`}
              aria-hidden="true"
            >
              <Icon className="h-[18px] w-[18px]" strokeWidth={1.75} />
            </span>
            <div className="min-w-0 flex-1">
              <h3 className="section-title">{opts.title}</h3>
              {opts.message && (
                <p className="mt-1.5 break-words text-sm text-zinc-400">{opts.message}</p>
              )}
            </div>
          </div>
          {opts.detail && (
            // 요약 칸은 호출부가 cf-d-main(이름) · cf-d-amt(금액, 방향 클래스) · cf-d-sub(부가 정보)로 채운다
            <div
              className={[
                "surface-inset mt-4 grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-3 gap-y-1 p-3.5 text-sm text-zinc-200",
                "[&_.cf-d-main]:truncate [&_.cf-d-main]:font-semibold [&_.cf-d-main]:text-white",
                "[&_.cf-d-amt]:whitespace-nowrap [&_.cf-d-amt]:font-semibold [&_.cf-d-amt]:tabular-nums [&_.cf-d-amt]:text-white",
                "[&_.cf-d-amt.expense]:text-rose-300 [&_.cf-d-amt.income]:text-emerald-300 [&_.cf-d-amt.transfer]:text-sky-300",
                "[&_.cf-d-sub]:col-span-2 [&_.cf-d-sub]:text-xs [&_.cf-d-sub]:text-zinc-400",
              ].join(" ")}
            >
              {opts.detail}
            </div>
          )}
          <div className="mt-5 flex items-center justify-end gap-2">
            <button type="button" className="btn-ghost" onClick={() => onClose(false)}>
              {opts.cancelLabel ?? "취소"}
            </button>
            <button
              type="button"
              className={danger ? "btn-danger" : "btn-primary"}
              onClick={() => onClose(true)}
            >
              {opts.confirmLabel ?? "확인"}
            </button>
          </div>
          <p className="label-mono mt-3 text-right !normal-case">Enter 확인 · Esc 취소</p>
        </div>
      </div>
    </Portal>
  );
}
