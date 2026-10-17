import { ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

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

  return createPortal(
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div
        ref={boxRef}
        tabIndex={-1}
        role="alertdialog"
        aria-modal="true"
        aria-label={opts.title}
        className={`modal confirm-modal${opts.danger ? " danger" : ""}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="cf-icon" aria-hidden="true">{opts.danger ? "!" : "?"}</div>
        <h3>{opts.title}</h3>
        {opts.detail && <div className="cf-detail">{opts.detail}</div>}
        {opts.message && <p className="cf-msg">{opts.message}</p>}
        <div className="modal-actions">
          <button type="button" className="ghost" onClick={() => onClose(false)}>
            {opts.cancelLabel ?? "취소"}
          </button>
          <button
            type="button"
            className={opts.danger ? "btn-danger" : undefined}
            onClick={() => onClose(true)}
          >
            {opts.confirmLabel ?? "확인"}
          </button>
        </div>
        <span className="cf-keys">Enter 확인 · Esc 취소</span>
      </div>
    </div>,
    document.body
  );
}
