import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  AddTransaction,
  ClassifyTransaction,
  DeleteTransaction,
  ListTransactions,
} from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import { DIRECTION_LABEL, formatAmount, parseAmount, Refs, today, won } from "../lib";
import PmChip from "../PmChip";
import { TableSkeleton } from "../Skeleton";

const EMPTY_FORM = {
  date: today(),
  direction: "expense",
  amount: "",
  merchant: "",
  memo: "",
  memberId: "",
  categoryId: "",
  paymentMethodId: "",
};

// 수동 등록 폼: 현금 지출, 회비, 경조사처럼 내역서에 안 잡히는 거래를 직접 입력한다.
function ManualForm({ refs, onAdded }: { refs: Refs; onAdded: () => void }) {
  const [f, setF] = useState({ ...EMPTY_FORM });
  const [err, setErr] = useState("");

  // 귀속자 목록이 로드되면 "아빠"를 기본 선택해 둔다.
  const dadId = refs.members.find((m) => m.name === "아빠")?.id;
  useEffect(() => {
    if (dadId) {
      setF((prev) => (prev.memberId ? prev : { ...prev, memberId: String(dadId) }));
    }
  }, [dadId]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    const amount = parseAmount(f.amount);
    if (!f.date || !amount || amount <= 0) {
      setErr("날짜와 금액(양수)을 입력하세요.");
      return;
    }
    try {
      await AddTransaction(
        new store.Transaction({
          id: 0,
          date: f.date,
          amount,
          direction: f.direction,
          merchant: f.merchant,
          memo: f.memo,
          memberId: f.memberId ? Number(f.memberId) : undefined,
          categoryId: f.categoryId ? Number(f.categoryId) : undefined,
          paymentMethodId: f.paymentMethodId ? Number(f.paymentMethodId) : undefined,
          source: "manual",
        })
      );
      // 금액만 바꿔 연속 입력하는 경우가 많아 금액만 비우고 나머지는 유지한다
      setF({ ...f, amount: "" });
      onAdded();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const cats = refs.categories.filter((c) => c.kind === f.direction);

  return (
    <form className="card manual-form" onSubmit={submit}>
      <h3>수동 등록</h3>
      <div className="form-row">
        <input
          type="date"
          value={f.date}
          onChange={(e) => setF({ ...f, date: e.target.value })}
        />
        <select
          value={f.direction}
          onChange={(e) => setF({ ...f, direction: e.target.value, categoryId: "" })}
        >
          <option value="expense">지출</option>
          <option value="income">수입</option>
          <option value="transfer">이체</option>
        </select>
        <input
          type="text"
          inputMode="numeric"
          placeholder="금액 (원)"
          value={f.amount}
          onChange={(e) => setF({ ...f, amount: formatAmount(e.target.value) })}
        />
        <input
          type="text"
          placeholder="내용 / 가맹점 (예: 동창회 회비)"
          value={f.merchant}
          onChange={(e) => setF({ ...f, merchant: e.target.value })}
        />
      </div>
      <div className="form-row">
        <select
          value={f.memberId}
          onChange={(e) => setF({ ...f, memberId: e.target.value })}
        >
          <option value="">귀속자 선택</option>
          {refs.members.map((m) => (
            <option key={m.id} value={m.id}>{m.name}</option>
          ))}
        </select>
        <select
          value={f.categoryId}
          onChange={(e) => setF({ ...f, categoryId: e.target.value })}
        >
          <option value="">카테고리 선택</option>
          {cats.map((c) => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
        <select
          value={f.paymentMethodId}
          onChange={(e) => setF({ ...f, paymentMethodId: e.target.value })}
        >
          <option value="">결제수단 선택</option>
          {refs.paymentMethods.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <input
          type="text"
          placeholder="메모"
          value={f.memo}
          onChange={(e) => setF({ ...f, memo: e.target.value })}
        />
        <button type="submit">등록</button>
      </div>
      {err && <p className="error">{err}</p>}
      <p className="muted small">
        귀속자/카테고리를 지정해서 등록하면 같은 내용·비슷한 금액의 거래가 다음부터 자동
        분류됩니다.
      </p>
    </form>
  );
}

// 거래 수정 모달: 모든 항목을 한 번에 고친다. 저장 시 분류 규칙도 함께 학습된다.
function EditModal({
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
        })
      );
      onSaved();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const cats = refs.categories.filter((c) => c.kind === f.direction);

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
                <option key={c.id} value={c.id}>{c.name}</option>
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

export default function Transactions({
  refs,
  month,
  setMonth,
  unclassifiedOnly,
  setUnclassifiedOnly,
}: {
  refs: Refs;
  month: string;
  setMonth: (m: string) => void;
  unclassifiedOnly: boolean;
  setUnclassifiedOnly: (v: boolean) => void;
}) {
  const [txs, setTxs] = useState<store.Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [editTx, setEditTx] = useState<store.Transaction | null>(null);

  const load = useCallback(async () => {
    try {
      setTxs(await ListTransactions(month, unclassifiedOnly));
    } finally {
      setLoading(false);
    }
  }, [month, unclassifiedOnly]);

  useEffect(() => {
    load();
  }, [load]);

  const remove = async (t: store.Transaction) => {
    await DeleteTransaction(t.id);
    load();
  };

  return (
    <div>
      <ManualForm refs={refs} onAdded={load} />

      <div className="toolbar">
        <input type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
        <label className="check">
          <input
            type="checkbox"
            checked={unclassifiedOnly}
            onChange={(e) => setUnclassifiedOnly(e.target.checked)}
          />
          미분류만 보기
        </label>
        <span className="muted">{txs.length}건</span>
      </div>

      <table className="tx-table">
        <thead>
          <tr>
            <th>날짜</th>
            <th>내용</th>
            <th className="num">금액</th>
            <th>구분</th>
            <th>귀속자</th>
            <th>카테고리</th>
            <th>결제수단</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {loading && txs.length === 0 && <TableSkeleton cols={8} />}
          {txs.map((t) => (
            <tr key={t.id} className={!t.memberId || !t.categoryId ? "row-unclassified" : ""}>
              <td>{t.date}</td>
              <td>
                {t.merchant || "-"}
                {t.autoClassified && <span className="badge auto">자동</span>}
                {t.source === "manual" && <span className="badge manual">수동</span>}
                {t.memo && <div className="muted small">{t.memo}</div>}
              </td>
              <td className={`num ${t.direction}`}>{won(t.amount)}</td>
              <td>{DIRECTION_LABEL[t.direction]}</td>
              <td>{t.memberName || <span className="muted">미지정</span>}</td>
              <td>{t.categoryName || <span className="muted">미분류</span>}</td>
              <td>
                <PmChip
                  name={t.paymentMethodName}
                  type={refs.paymentMethods.find((p) => p.id === t.paymentMethodId)?.type}
                  color={refs.paymentMethods.find((p) => p.id === t.paymentMethodId)?.color}
                />
              </td>
              <td className="actions">
                <button className="ghost" onClick={() => setEditTx(t)} title="수정">✎</button>
                <button className="ghost" onClick={() => remove(t)} title="삭제">✕</button>
              </td>
            </tr>
          ))}
          {!loading && txs.length === 0 && (
            <tr>
              <td colSpan={8} className="muted center">거래가 없습니다</td>
            </tr>
          )}
        </tbody>
      </table>

      {editTx && (
        <EditModal
          tx={editTx}
          refs={refs}
          onClose={() => setEditTx(null)}
          onSaved={() => {
            setEditTx(null);
            load();
          }}
        />
      )}
    </div>
  );
}
