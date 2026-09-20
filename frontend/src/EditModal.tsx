import { FormEvent, useState } from "react";
import { ClassifyTransaction } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { categoryOptions, formatAmount, parseAmount, Refs } from "./lib";

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

  return (
    <div className="modal-overlay" onClick={onClose}>
      <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={save}>
        <h3>거래 수정</h3>
        <div className="modal-grid">
          <label className="field">
            날짜
            <input
              type="date"
              value={f.date}
              onChange={(e) => setF({ ...f, date: e.target.value })}
            />
          </label>
          <label className="field">
            구분
            <select
              value={f.direction}
              onChange={(e) => setF({ ...f, direction: e.target.value, categoryId: "" })}
            >
              <option value="expense">지출</option>
              <option value="income">수입</option>
              <option value="transfer">이체</option>
            </select>
          </label>
          <label className="field">
            금액 (원)
            <input
              type="text"
              inputMode="numeric"
              value={f.amount}
              onChange={(e) => setF({ ...f, amount: formatAmount(e.target.value) })}
            />
          </label>
          <label className="field span2">
            내용 / 가맹점
            <input
              type="text"
              value={f.merchant}
              onChange={(e) => setF({ ...f, merchant: e.target.value })}
            />
          </label>
          <label className="field">
            귀속자
            <select
              value={f.memberId}
              onChange={(e) => setF({ ...f, memberId: e.target.value })}
            >
              <option value="">미지정</option>
              {refs.members.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </select>
          </label>
          <label className="field">
            카테고리
            <select
              value={f.categoryId}
              onChange={(e) => setF({ ...f, categoryId: e.target.value })}
            >
              <option value="">미지정</option>
              {cats.map((c) => (
                <option key={c.id} value={c.id}>{c.label}</option>
              ))}
            </select>
          </label>
          <label className="field">
            결제수단
            <select
              value={f.paymentMethodId}
              onChange={(e) => setF({ ...f, paymentMethodId: e.target.value })}
            >
              <option value="">미지정</option>
              {refs.paymentMethods.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
          </label>
          <label className="field span3">
            메모
            <input
              type="text"
              value={f.memo}
              onChange={(e) => setF({ ...f, memo: e.target.value })}
            />
          </label>
          <label className="field span3 check-field">
            <input
              type="checkbox"
              checked={f.excludePerf}
              onChange={(e) => setF({ ...f, excludePerf: e.target.checked })}
            />
            <span className="cf-text">카드 실적에서 제외</span>
            <span className="cf-hint muted">세금·공과금 등 — 지출 통계에는 그대로 집계</span>
          </label>
        </div>
        {err && <p className="error">{err}</p>}
        <div className="modal-actions">
          <button type="button" className="ghost" onClick={onClose}>취소</button>
          <button type="submit">저장</button>
        </div>
      </form>
    </div>
  );
}
