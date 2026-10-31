import { FormEvent, useState } from "react";
import { Pencil, X } from "lucide-react";
import { ClassifyTransaction } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { categoryOptions, formatAmount, parseAmount, Refs } from "./lib";
import Portal from "./components/ui/Portal";

// 거래 한 건 수정 모달. 거래내역 탭과 통계의 월별 상세 드릴다운에서 같이 쓴다.
// 저장은 ClassifyTransaction 을 거치므로, 카테고리를 직접 고르면 규칙 학습까지 이어진다.
export default function EditModal({
  tx,
  refs,
  onClose,
  onSaved,
}: {
  tx: store.Transaction;
  refs: Refs;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [f, setF] = useState({
    date: tx.date,
    direction: tx.direction,
    amount: formatAmount(String(tx.amount)),
    merchant: tx.merchant,
    memo: tx.memo,
    memberId: tx.memberId ? String(tx.memberId) : "",
    categoryId: tx.categoryId ? String(tx.categoryId) : "",
    paymentMethodId: tx.paymentMethodId ? String(tx.paymentMethodId) : "",
    excludePerf: tx.excludePerf,
  });
  const [err, setErr] = useState("");

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    const amount = parseAmount(f.amount);
    if (!f.date || !amount || amount <= 0) {
      setErr("날짜와 금액(양수)을 입력하세요.");
      return;
    }
    try {
      await ClassifyTransaction(
        new store.Transaction({
          ...tx,
          date: f.date,
          direction: f.direction,
          amount,
          merchant: f.merchant,
          memo: f.memo,
          memberId: f.memberId ? Number(f.memberId) : undefined,
          categoryId: f.categoryId ? Number(f.categoryId) : undefined,
          paymentMethodId: f.paymentMethodId ? Number(f.paymentMethodId) : undefined,
          excludePerf: f.excludePerf,
        })
      );
      onSaved();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const cats = categoryOptions(refs.categories, f.direction);

  // 모달은 Portal 로 body 에 띄운다. 페이지 안에 두면 조상의 transform/filter 가
  // position:fixed 의 기준 상자를 바꿔 모달이 화면 밖으로 밀려날 수 있다.
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
          aria-label="거래 수정"
          className="surface max-h-[92vh] w-full max-w-2xl overflow-y-auto p-4 sm:p-6"
          onSubmit={save}
        >
          <div className="mb-4 flex items-start justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <span className="icon-tile icon-tile-mint !h-8 !w-8 !rounded-lg">
                <Pencil className="h-4 w-4" strokeWidth={1.75} />
              </span>
              <h3 className="section-title">거래 수정</h3>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="btn-icon shrink-0"
              aria-label="닫기"
              title="닫기"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <label className="block">
              <span className="field-label">날짜</span>
              <input
                type="date"
                className="input-field num"
                value={f.date}
                onChange={(e) => setF({ ...f, date: e.target.value })}
              />
            </label>
            <label className="block">
              <span className="field-label">구분</span>
              <select
                className="input-field"
                value={f.direction}
                onChange={(e) => setF({ ...f, direction: e.target.value, categoryId: "" })}
              >
                <option value="expense">지출</option>
                <option value="income">수입</option>
                <option value="transfer">이체</option>
              </select>
            </label>
            <label className="block">
              <span className="field-label">금액 (원)</span>
              <input
                type="text"
                inputMode="numeric"
                className="input-field num"
                value={f.amount}
                onChange={(e) => setF({ ...f, amount: formatAmount(e.target.value) })}
              />
            </label>
            <label className="block sm:col-span-2">
              <span className="field-label">내용 / 가맹점</span>
              <input
                type="text"
                className="input-field"
                value={f.merchant}
                onChange={(e) => setF({ ...f, merchant: e.target.value })}
              />
            </label>
            <label className="block">
              <span className="field-label">귀속자</span>
              <select
                className="input-field"
                value={f.memberId}
                onChange={(e) => setF({ ...f, memberId: e.target.value })}
              >
                <option value="">미지정</option>
                {refs.members.map((m) => (
                  <option key={m.id} value={m.id}>{m.name}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="field-label">카테고리</span>
              <select
                className="input-field"
                value={f.categoryId}
                onChange={(e) => setF({ ...f, categoryId: e.target.value })}
              >
                <option value="">미지정</option>
                {cats.map((c) => (
                  <option key={c.id} value={c.id}>{c.label}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="field-label">결제수단</span>
              <select
                className="input-field"
                value={f.paymentMethodId}
                onChange={(e) => setF({ ...f, paymentMethodId: e.target.value })}
              >
                <option value="">미지정</option>
                {refs.paymentMethods.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            </label>
            <label className="block sm:col-span-3">
              <span className="field-label">메모</span>
              <input
                type="text"
                className="input-field"
                value={f.memo}
                onChange={(e) => setF({ ...f, memo: e.target.value })}
              />
            </label>
            <label className="surface-inset flex cursor-pointer flex-wrap items-center gap-x-2.5 gap-y-1 p-3 sm:col-span-3">
              <input
                type="checkbox"
                className="h-4 w-4 accent-mint-400"
                checked={f.excludePerf}
                onChange={(e) => setF({ ...f, excludePerf: e.target.checked })}
              />
              <span className="text-sm text-zinc-200">카드 실적에서 제외</span>
              <span className="text-xs text-zinc-500">세금·공과금 등 — 지출 통계에는 그대로 집계</span>
            </label>
          </div>

          {err && <p className="mt-3 text-xs text-rose-300">{err}</p>}
          <div className="mt-5 flex items-center justify-end gap-2">
            <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
            <button type="submit" className="btn-primary">저장</button>
          </div>
        </form>
      </div>
    </Portal>
  );
}
