import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  AddTransaction,
  ApplyRulesToUnclassified,
  BatchClassify,
  BatchDelete,
  CanUndo,
  ClassifyTransaction,
  DeleteTransaction,
  ExportCSV,
  ListTransactions,
  UndoDelete,
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
        이미 학습된 가맹점·금액이면 규칙대로 자동 분류됩니다(기본 귀속자보다 규칙이 우선).
        새 거래는 카테고리까지 골라 등록하면 다음부터 자동 분류되도록 규칙으로 학습됩니다.
        규칙과 다르게 처리하려면 등록 후 해당 행을 수정하세요.
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
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [canUndo, setCanUndo] = useState(false);
  const [msg, setMsg] = useState("");

  // 필터 상태
  const [q, setQ] = useState("");
  const [direction, setDirection] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [memberId, setMemberId] = useState("");
  const [pmId, setPmId] = useState("");
  const [amountMin, setAmountMin] = useState("");
  const [amountMax, setAmountMax] = useState("");
  const [sort, setSort] = useState("date_desc");

  // 일괄 분류 선택값
  const [bulkMember, setBulkMember] = useState("");
  const [bulkCategory, setBulkCategory] = useState("");
  const [bulkLearn, setBulkLearn] = useState(false);

  const buildFilter = useCallback(
    () =>
      new store.TxFilter({
        month,
        unclassifiedOnly,
        query: q.trim(),
        direction,
        categoryId: categoryId ? Number(categoryId) : 0,
        memberId: memberId ? Number(memberId) : 0,
        paymentMethodId: pmId ? Number(pmId) : 0,
        amountMin: amountMin ? parseAmount(amountMin) : 0,
        amountMax: amountMax ? parseAmount(amountMax) : 0,
        sort,
      }),
    [month, unclassifiedOnly, q, direction, categoryId, memberId, pmId, amountMin, amountMax, sort]
  );

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setTxs(await ListTransactions(buildFilter()));
      setSelected(new Set());
      setCanUndo(await CanUndo());
    } finally {
      setLoading(false);
    }
  }, [buildFilter]);

  // 필터 변경 시 250ms 디바운스로 재조회(타이핑마다 쿼리하지 않도록)
  useEffect(() => {
    const id = setTimeout(load, 250);
    return () => clearTimeout(id);
  }, [load]);

  const resetFilters = () => {
    setQ("");
    setDirection("");
    setCategoryId("");
    setMemberId("");
    setPmId("");
    setAmountMin("");
    setAmountMax("");
    setSort("date_desc");
    setUnclassifiedOnly(false);
  };

  const remove = async (t: store.Transaction) => {
    await DeleteTransaction(t.id);
    load();
  };

  const applyRules = async () => {
    const n = await ApplyRulesToUnclassified(month);
    setMsg(n > 0 ? `규칙으로 ${n}건을 자동 분류했습니다.` : "규칙에 맞는 미분류 거래가 없습니다.");
    load();
  };

  const exportCsv = async () => {
    const r = await ExportCSV(buildFilter());
    if (r.path) setMsg(`${r.count}건을 내보냈습니다: ${r.path}`);
  };

  const undo = async () => {
    const n = await UndoDelete();
    setMsg(n > 0 ? `${n}건을 복원했습니다.` : "복원할 항목이 없습니다.");
    load();
  };

  const toggle = (id: number) =>
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  const allChecked = txs.length > 0 && selected.size === txs.length;
  const toggleAll = () =>
    setSelected(allChecked ? new Set() : new Set(txs.map((t) => t.id)));

  const bulkClassify = async () => {
    if (selected.size === 0) return;
    await BatchClassify(
      [...selected],
      bulkMember ? Number(bulkMember) : 0,
      bulkCategory ? Number(bulkCategory) : 0,
      bulkLearn
    );
    setBulkMember("");
    setBulkCategory("");
    load();
  };

  const bulkDelete = async () => {
    if (selected.size === 0) return;
    if (!window.confirm(`${selected.size}건을 삭제할까요? (되돌리기 가능)`)) return;
    await BatchDelete([...selected]);
    setMsg(`${selected.size}건을 삭제했습니다.`);
    load();
  };

  return (
    <div>
      <ManualForm refs={refs} onAdded={load} />

      <div className="toolbar wrap">
        <input type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
        <input
          type="text"
          className="search"
          placeholder="가맹점·메모 검색"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <select value={direction} onChange={(e) => setDirection(e.target.value)}>
          <option value="">구분 전체</option>
          <option value="expense">지출</option>
          <option value="income">수입</option>
          <option value="transfer">이체</option>
        </select>
        <select value={categoryId} onChange={(e) => setCategoryId(e.target.value)}>
          <option value="">카테고리 전체</option>
          {refs.categories.map((c) => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
        <select value={memberId} onChange={(e) => setMemberId(e.target.value)}>
          <option value="">귀속자 전체</option>
          {refs.members.map((m) => (
            <option key={m.id} value={m.id}>{m.name}</option>
          ))}
        </select>
        <select value={pmId} onChange={(e) => setPmId(e.target.value)}>
          <option value="">결제수단 전체</option>
          {refs.paymentMethods.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <input
          type="text"
          inputMode="numeric"
          className="amt"
          placeholder="최소액"
          value={amountMin}
          onChange={(e) => setAmountMin(formatAmount(e.target.value))}
        />
        <input
          type="text"
          inputMode="numeric"
          className="amt"
          placeholder="최대액"
          value={amountMax}
          onChange={(e) => setAmountMax(formatAmount(e.target.value))}
        />
        <select value={sort} onChange={(e) => setSort(e.target.value)}>
          <option value="date_desc">최신순</option>
          <option value="date_asc">오래된순</option>
          <option value="amount_desc">금액↓</option>
          <option value="amount_asc">금액↑</option>
        </select>
        <label className="check">
          <input
            type="checkbox"
            checked={unclassifiedOnly}
            onChange={(e) => setUnclassifiedOnly(e.target.checked)}
          />
          미분류만
        </label>
        <button className="ghost" onClick={resetFilters}>필터 초기화</button>
      </div>

      <div className="toolbar wrap">
        <span className="muted">{txs.length}건</span>
        <button className="warn" onClick={applyRules}>규칙으로 미분류 분류</button>
        <button className="ghost-btn" onClick={exportCsv}>CSV 내보내기</button>
        {canUndo && (
          <button className="ghost-btn" onClick={undo}>↩ 삭제 취소</button>
        )}
      </div>

      {msg && <p className="muted small toast-line">{msg}</p>}

      {selected.size > 0 && (
        <div className="bulk-bar">
          <span><strong>{selected.size}건</strong> 선택됨</span>
          <select value={bulkMember} onChange={(e) => setBulkMember(e.target.value)}>
            <option value="">귀속자 변경</option>
            {refs.members.map((m) => (
              <option key={m.id} value={m.id}>{m.name}</option>
            ))}
          </select>
          <select value={bulkCategory} onChange={(e) => setBulkCategory(e.target.value)}>
            <option value="">카테고리 변경</option>
            {refs.categories.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <label className="check small">
            <input
              type="checkbox"
              checked={bulkLearn}
              onChange={(e) => setBulkLearn(e.target.checked)}
            />
            규칙 학습
          </label>
          <button onClick={bulkClassify} disabled={!bulkMember && !bulkCategory}>일괄 적용</button>
          <button className="warn" onClick={bulkDelete}>일괄 삭제</button>
          <button className="ghost" onClick={() => setSelected(new Set())}>선택 해제</button>
        </div>
      )}

      <table className="tx-table">
        <thead>
          <tr>
            <th className="chk">
              <input type="checkbox" checked={allChecked} onChange={toggleAll} />
            </th>
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
          {loading && txs.length === 0 && <TableSkeleton cols={9} />}
          {txs.map((t) => (
            <tr
              key={t.id}
              className={`${!t.memberId || !t.categoryId ? "row-unclassified" : ""} ${
                selected.has(t.id) ? "row-selected" : ""
              }`}
            >
              <td className="chk">
                <input
                  type="checkbox"
                  checked={selected.has(t.id)}
                  onChange={() => toggle(t.id)}
                />
              </td>
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
              <td colSpan={9} className="muted center">거래가 없습니다</td>
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
