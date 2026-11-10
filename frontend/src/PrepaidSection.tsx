import { FormEvent, ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import {
  AddPrepaidCharge,
  DeletePrepaidPass,
  DeletePrepaidUse,
  GetPrepaidHistory,
  LinkPrepaidCharge,
  ListPrepaidStatuses,
  SavePrepaidPass,
  SavePrepaidUse,
  SuggestPrepaidCharges,
  UnlinkPrepaidCharge,
} from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { History, Link2, MinusCircle, Pencil, Plus, PlusCircle, Ticket, Trash2 } from "lucide-react";
import IconTile from "./components/ui/IconTile";
import Modal, { Field, Segmented } from "./components/ui/Modal";
import CountUp from "./components/CountUp";
import { useConfirm } from "./ConfirmModal";
import {
  categoryOptions,
  CLOSED_HINT,
  formatAmount,
  isClosed,
  memberForCategory,
  methodLabel,
  parseAmount,
  Refs,
  selectableMethods,
  today,
  won,
} from "./lib";

const QUICK_MEMOS = ["커트", "펌", "염색", "클리닉"];

// "10" → 1000 (만분율), 1000 → "10"
const pctToBp = (s: string) => Math.round((Number(s) || 0) * 100);
const bpToPct = (bp: number) => (bp ? String(bp / 100) : "");

// 충전 보너스 / 차감 할인이 붙는지 ("both" 는 둘 다)
const hasBonus = (mode: string) => mode === "bonus" || mode === "both";
const hasDiscount = (mode: string) => mode === "discount" || mode === "both";
const bonusOf = (p: store.PrepaidPass, paid: number) =>
  hasBonus(p.discountMode) ? Math.round((paid * p.bonusBp) / 10000) : 0;
const deductOf = (p: store.PrepaidPass, list: number) =>
  hasDiscount(p.discountMode) ? Math.round((list * (10000 - p.discountBp)) / 10000) : list;

const idOrUndef = (s: string) => (s ? Number(s) : undefined);

// ---------- 선불권 등록/수정 ----------
function PassForm({
  pass,
  refs,
  onClose,
  onSaved,
}: {
  pass: store.PrepaidPass | null;
  refs: Refs;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [f, setF] = useState(() => ({
    name: pass?.name ?? "",
    merchant: pass?.merchant ?? "",
    mode: pass?.discountMode || "bonus",
    bonusPct: bpToPct(pass ? pass.bonusBp : 1000),
    discountPct: bpToPct(pass ? pass.discountBp : 1000),
    opening: pass?.openingBalance ? formatAmount(String(pass.openingBalance)) : "",
    openingDate: pass?.openingDate ?? "",
    expiresAt: pass?.expiresAt ?? "",
    categoryId: pass?.categoryId ? String(pass.categoryId) : "",
    paymentMethodId: pass?.paymentMethodId ? String(pass.paymentMethodId) : "",
    memberId: pass?.memberId ? String(pass.memberId) : "",
    memo: pass?.memo ?? "",
    active: pass?.active ?? true,
  }));
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const set = (p: Partial<typeof f>) => setF((prev) => ({ ...prev, ...p }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!f.name.trim()) {
      setErr("이름을 입력하세요.");
      return;
    }
    setBusy(true);
    try {
      await SavePrepaidPass(
        new store.PrepaidPass({
          id: pass?.id ?? 0,
          name: f.name.trim(),
          merchant: f.merchant.trim(),
          discountMode: f.mode,
          bonusBp: hasBonus(f.mode) ? pctToBp(f.bonusPct) : 0,
          discountBp: hasDiscount(f.mode) ? pctToBp(f.discountPct) : 0,
          openingBalance: parseAmount(f.opening) || 0,
          // 처음 잔액이 없으면 기준일도 비운다 — 기준일이 있으면 그 전의 충전(예: 지난달 충전 거래 연결)이 잔액에서 빠진다
          openingDate: (parseAmount(f.opening) || 0) > 0 ? f.openingDate || today() : "",
          expiresAt: f.expiresAt,
          categoryId: idOrUndef(f.categoryId),
          paymentMethodId: idOrUndef(f.paymentMethodId),
          memberId: idOrUndef(f.memberId),
          memo: f.memo,
          active: f.active,
        })
      );
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={pass ? "선불권 수정" : "선불권 추가"} icon={Ticket} onClose={onClose} onSubmit={submit}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="이름">
          <input className="input-field" placeholder="예: OO헤어 회원권" value={f.name} autoFocus onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <Field label="가게 (가맹점)">
          <input className="input-field" placeholder="충전 거래의 내용과 맞춰 찾습니다" value={f.merchant} onChange={(e) => set({ merchant: e.target.value })} />
        </Field>
        <div className="sm:col-span-2">
          <span className="field-label">할인 방식</span>
          <div className="flex flex-wrap items-center gap-3">
            <Segmented
              value={f.mode}
              onChange={(v) => set({ mode: v })}
              options={[
                { v: "bonus", label: "충전 보너스" },
                { v: "discount", label: "차감 할인" },
                { v: "both", label: "보너스 + 할인" },
                { v: "none", label: "할인 없음" },
              ]}
            />
            {hasBonus(f.mode) && (
              <label className="flex items-center gap-2">
                <span className="text-sm text-zinc-400">보너스</span>
                <input
                  className="input-field num !w-20"
                  inputMode="decimal"
                  placeholder="10"
                  value={f.bonusPct}
                  onChange={(e) => set({ bonusPct: e.target.value })}
                  aria-label="충전 보너스율 (%)"
                />
                <span className="text-sm text-zinc-400">%</span>
              </label>
            )}
            {hasDiscount(f.mode) && (
              <label className="flex items-center gap-2">
                <span className="text-sm text-zinc-400">할인</span>
                <input
                  className="input-field num !w-20"
                  inputMode="decimal"
                  placeholder="10"
                  value={f.discountPct}
                  onChange={(e) => set({ discountPct: e.target.value })}
                  aria-label="차감 할인율 (%)"
                />
                <span className="text-sm text-zinc-400">%</span>
              </label>
            )}
          </div>
          <p className="label-mono mt-1.5 whitespace-normal">
            {f.mode === "bonus" && "충전하면 결제액에 보너스를 더 적립합니다 (30만 원 충전 → 33만 원 적립). 차감은 정가 그대로."}
            {f.mode === "discount" && "적립은 결제액 그대로, 쓸 때 정가에서 할인율만큼 깎아 차감합니다."}
            {f.mode === "both" && "충전하면 보너스를 더 적립하고, 쓸 때도 정가에서 할인율만큼 깎아 차감합니다 (예: 보너스 10%·할인 5%면 30만 원 충전 → 33만 원 적립, 정가 5만 원 → 4.75만 원 차감)."}
            {f.mode === "none" && "결제액만큼 적립하고 정가 그대로 차감합니다."}
          </p>
        </div>
        <Field label="처음 잔액">
          <input
            className="input-field num"
            inputMode="numeric"
            placeholder="0"
            value={f.opening}
            onChange={(e) => set({ opening: formatAmount(e.target.value) })}
          />
          <span className="label-mono mt-1 block whitespace-normal">앱 쓰기 전부터 남아 있던 잔액</span>
        </Field>
        <Field label="기준일">
          <input
            type="date"
            className="input-field disabled:opacity-50"
            value={f.openingDate}
            disabled={!(parseAmount(f.opening) > 0)}
            onChange={(e) => set({ openingDate: e.target.value })}
          />
          <span className="label-mono mt-1 block whitespace-normal">
            처음 잔액을 넣을 때만 씁니다(비우면 오늘). 이 날 전의 충전·차감은 처음 잔액에 이미 들어 있는 것으로 봅니다
          </span>
        </Field>
        <Field label="유효기간 (선택)">
          <input type="date" className="input-field" value={f.expiresAt} onChange={(e) => set({ expiresAt: e.target.value })} />
        </Field>
        <Field label="귀속자 (기본)">
          <select className="input-field" value={f.memberId} onChange={(e) => set({ memberId: e.target.value })}>
            <option value="">선택 안 함</option>
            {refs.members.map((m) => (
              <option key={m.id} value={m.id}>{m.name}</option>
            ))}
          </select>
        </Field>
        <Field label="충전 카테고리 (기본)">
          <select
            className="input-field"
            value={f.categoryId}
            onChange={(e) =>
              set({
                categoryId: e.target.value,
                memberId: memberForCategory(refs.categories, f.categoryId, e.target.value, f.memberId),
              })
            }
          >
            <option value="">선택 안 함</option>
            {categoryOptions(refs.categories, "expense").map((c) => (
              <option key={c.id} value={c.id}>{c.label}</option>
            ))}
          </select>
        </Field>
        <Field label="충전 결제수단 (기본)">
          <select className="input-field" value={f.paymentMethodId} onChange={(e) => set({ paymentMethodId: e.target.value })}>
            <option value="">선택 안 함</option>
            {selectableMethods(refs.paymentMethods, f.paymentMethodId).map((p) => (
              <option key={p.id} value={p.id}>{methodLabel(p)}</option>
            ))}
          </select>
          {isClosed(refs.paymentMethods.find((p) => String(p.id) === String(f.paymentMethodId))) && (
            <p className="mt-1 text-xs text-amber-300">{CLOSED_HINT}</p>
          )}
        </Field>
        <Field label="메모" className="sm:col-span-2">
          <input className="input-field" value={f.memo} onChange={(e) => set({ memo: e.target.value })} />
        </Field>
        <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
          <input type="checkbox" checked={f.active} onChange={(e) => set({ active: e.target.checked })} />
          사용 중 (다 쓰고 정리하면 끄세요. 목록 아래로 내려갑니다)
        </label>
      </div>
      <p className="surface-inset mt-4 rounded-xl p-3 text-xs text-zinc-400">
        충전한 금액은 결제한 날 지출로 잡힙니다 (통계·카드값·자금 계획에 반영). 쓸 때(차감)는 지출이 아니라 선불권 잔액만 줄어듭니다.
      </p>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>{pass ? "저장" : "추가"}</button>
      </div>
    </Modal>
  );
}

// ---------- 사용(차감) 기록 ----------
function UseModal({
  status,
  use,
  refs,
  onClose,
  onSaved,
}: {
  status: store.PrepaidStatus;
  use: store.PrepaidUse | null;
  refs: Refs;
  onClose: () => void;
  onSaved: () => void;
}) {
  const p = status.pass;
  const [date, setDate] = useState(use?.date ?? today());
  const [list, setList] = useState(use ? formatAmount(String(use.listPrice || use.deducted)) : "");
  const [deducted, setDeducted] = useState(use ? formatAmount(String(use.deducted)) : "");
  const [manual, setManual] = useState(!!use); // 차감액을 직접 고쳤는지
  const [memberId, setMemberId] = useState(use ? (use.memberId ? String(use.memberId) : "") : p.memberId ? String(p.memberId) : "");
  const [memo, setMemo] = useState(use?.memo ?? "");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const listN = parseAmount(list) || 0;
  const dedN = parseAmount(deducted) || 0;
  // 수정 중이면 기존 차감액은 이미 잔액에서 빠져 있으니 되돌려 놓고 계산한다
  const after = status.balance + (use?.deducted ?? 0) - dedN;

  const onList = (v: string) => {
    const s = formatAmount(v);
    setList(s);
    if (!manual) setDeducted(s ? formatAmount(String(deductOf(p, parseAmount(s)))) : "");
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!date || dedN <= 0) {
      setErr("날짜와 차감액(양수)을 입력하세요.");
      return;
    }
    setBusy(true);
    try {
      await SavePrepaidUse(
        new store.PrepaidUse({
          id: use?.id ?? 0,
          passId: p.id,
          date,
          listPrice: listN || dedN,
          deducted: dedN,
          memberId: idOrUndef(memberId),
          memo,
        })
      );
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={`${p.name} · ${use ? "사용 기록 수정" : "사용 기록"}`} icon={MinusCircle} tone="sky" onClose={onClose} onSubmit={submit}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="날짜">
          <input type="date" className="input-field" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
        <Field label="귀속자">
          <select className="input-field" value={memberId} onChange={(e) => setMemberId(e.target.value)}>
            <option value="">선택 안 함</option>
            {refs.members.map((m) => (
              <option key={m.id} value={m.id}>{m.name}</option>
            ))}
          </select>
        </Field>
        <Field label="정가">
          <input className="input-field num" inputMode="numeric" autoFocus value={list} onChange={(e) => onList(e.target.value)} placeholder="0" />
        </Field>
        <Field label={hasDiscount(p.discountMode) ? `차감액 (${bpToPct(p.discountBp)}% 할인 적용)` : "차감액"}>
          <input
            className="input-field num"
            inputMode="numeric"
            value={deducted}
            placeholder="0"
            onChange={(e) => {
              setManual(true);
              setDeducted(formatAmount(e.target.value));
            }}
          />
        </Field>
        <div className="sm:col-span-2">
          <Field label="메모">
            <input className="input-field" value={memo} onChange={(e) => setMemo(e.target.value)} placeholder="예: 커트" />
          </Field>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {QUICK_MEMOS.map((m) => (
              <button key={m} type="button" className="chip hover:text-white" onClick={() => setMemo(m)}>
                {m}
              </button>
            ))}
          </div>
        </div>
      </div>
      <div className="surface-inset mt-4 flex items-center justify-between gap-3 rounded-xl p-3">
        <span className="label-mono">차감 후 잔액</span>
        <span className={`num text-lg font-semibold ${after < 0 ? "text-rose-300" : "text-emerald-300"}`}>{won(after)}</span>
      </div>
      {after < 0 && <p className="mt-1.5 text-xs text-rose-300">잔액 부족 — 그래도 기록할 수 있습니다.</p>}
      <p className="mt-3 text-xs text-zinc-500">
        사용은 지출이 아니라 선불권 잔액만 줄입니다. 그래서 통계에는 잡히지 않아요 (충전할 때 이미 지출로 잡혔습니다).
      </p>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>저장</button>
      </div>
    </Modal>
  );
}

// ---------- 충전 ----------
function ChargeModal({
  status,
  refs,
  onClose,
  onSaved,
}: {
  status: store.PrepaidStatus;
  refs: Refs;
  onClose: () => void;
  onSaved: () => void;
}) {
  const p = status.pass;
  const [tab, setTab] = useState<"new" | "link">("new");
  const [date, setDate] = useState(today());
  const [amount, setAmount] = useState("");
  const [pmId, setPmId] = useState(p.paymentMethodId ? String(p.paymentMethodId) : "");
  const [memberId, setMemberId] = useState(p.memberId ? String(p.memberId) : "");
  const [memo, setMemo] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [sug, setSug] = useState<store.Transaction[] | null>(null);
  const [sugErr, setSugErr] = useState("");

  useEffect(() => {
    if (tab !== "link" || sug) return;
    SuggestPrepaidCharges(p.id)
      .then((l) => setSug(l ?? []))
      .catch((e) => setSugErr(String(e)));
  }, [tab, sug, p.id]);

  const paid = parseAmount(amount) || 0;
  const bonus = bonusOf(p, paid);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!date || paid <= 0) {
      setErr("날짜와 결제액(양수)을 입력하세요.");
      return;
    }
    setBusy(true);
    try {
      await AddPrepaidCharge(p.id, date, paid, Number(pmId) || 0, Number(memberId) || 0, memo);
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const link = async (t: store.Transaction) => {
    setErr("");
    setBusy(true);
    try {
      await LinkPrepaidCharge(p.id, t.id);
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
      setBusy(false);
    }
  };

  return (
    <Modal
      title={`${p.name} · 충전`}
      icon={PlusCircle}
      onClose={onClose}
      onSubmit={tab === "new" ? submit : undefined}
    >
      <div className="mb-4">
        <Segmented
          value={tab}
          onChange={setTab}
          options={[
            { v: "new", label: "새로 기록" },
            { v: "link", label: "기존 거래 연결" },
          ]}
        />
      </div>
      {tab === "new" ? (
        <>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="날짜">
              <input type="date" className="input-field" value={date} onChange={(e) => setDate(e.target.value)} />
            </Field>
            <Field label="결제액">
              <input className="input-field num" inputMode="numeric" autoFocus value={amount} placeholder="0" onChange={(e) => setAmount(formatAmount(e.target.value))} />
            </Field>
            <Field label="결제수단">
              <select className="input-field" value={pmId} onChange={(e) => setPmId(e.target.value)}>
                <option value="">선택 안 함</option>
                {selectableMethods(refs.paymentMethods, pmId).map((m) => (
                  <option key={m.id} value={m.id}>{methodLabel(m)}</option>
                ))}
              </select>
              {isClosed(refs.paymentMethods.find((m) => String(m.id) === pmId)) && (
                <p className="mt-1 text-xs text-amber-300">{CLOSED_HINT}</p>
              )}
            </Field>
            <Field label="귀속자">
              <select className="input-field" value={memberId} onChange={(e) => setMemberId(e.target.value)}>
                <option value="">선택 안 함</option>
                {refs.members.map((m) => (
                  <option key={m.id} value={m.id}>{m.name}</option>
                ))}
              </select>
            </Field>
            <Field label="메모" className="sm:col-span-2">
              <input className="input-field" value={memo} onChange={(e) => setMemo(e.target.value)} />
            </Field>
          </div>
          <div className="surface-inset mt-4 flex items-center justify-between gap-3 rounded-xl p-3">
            <span className="label-mono">적립액</span>
            <span className="num text-lg font-semibold text-emerald-300">
              {won(paid + bonus)}
              {hasBonus(p.discountMode) && <span className="ml-2 text-xs font-normal text-emerald-300/80">(보너스 {won(bonus)})</span>}
            </span>
          </div>
          <p className="mt-3 text-xs text-zinc-500">충전 금액은 결제한 날 지출로 잡힙니다.</p>
          {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
          <div className="mt-4 flex justify-end gap-2">
            <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
            <button type="submit" className="btn-primary" disabled={busy}>충전 기록</button>
          </div>
        </>
      ) : (
        <div>
          <p className="mb-3 text-xs text-zinc-500">
            이미 등록한 지출 거래를 이 선불권의 충전으로 연결합니다. 가맹점 "{p.merchant || p.name}"과 맞는 최근 거래입니다.
          </p>
          {sugErr && <p className="text-sm text-rose-300">{sugErr}</p>}
          {!sug && !sugErr && <p className="label-mono">불러오는 중…</p>}
          {sug && sug.length === 0 && <p className="label-mono whitespace-normal">연결할 만한 거래가 없습니다. "새로 기록"으로 충전을 등록하세요.</p>}
          {sug && sug.length > 0 && (
            <ul className="space-y-1.5">
              {sug.map((t) => (
                <li key={t.id} className="surface-inset flex items-center justify-between gap-3 rounded-xl px-3 py-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm text-white">{t.merchant || "-"}</div>
                    <div className="label-mono num">
                      {t.date}
                      {t.paymentMethodName ? ` · ${t.paymentMethodName}` : ""}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-3">
                    <span className="num text-sm font-medium text-rose-300">{won(t.amount)}</span>
                    <button type="button" className="btn-ghost !px-2.5 !py-1 text-xs" disabled={busy} onClick={() => link(t)}>
                      <Link2 className="h-3.5 w-3.5" />
                      연결
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
          {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
        </div>
      )}
    </Modal>
  );
}

// ---------- 내역 ----------
type Row =
  | { kind: "charge"; date: string; c: store.PrepaidCharge }
  | { kind: "use"; date: string; u: store.PrepaidUse };

function HistoryModal({
  status,
  onClose,
  onChanged,
  onEditUse,
}: {
  status: store.PrepaidStatus;
  onClose: () => void;
  onChanged: () => void;
  onEditUse: (u: store.PrepaidUse) => void;
}) {
  const p = status.pass;
  const [h, setH] = useState<store.PrepaidHistory | null>(null);
  const [err, setErr] = useState("");
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(() => {
    GetPrepaidHistory(p.id)
      .then((r) => {
        setH(r);
        setErr("");
      })
      .catch((e) => setErr(String(e)));
  }, [p.id]);
  useEffect(load, [load]);

  const rows = useMemo<Row[]>(() => {
    if (!h) return [];
    const r: Row[] = [
      ...(h.charges ?? []).map((c): Row => ({ kind: "charge", date: c.date, c })),
      ...(h.uses ?? []).map((u): Row => ({ kind: "use", date: u.date, u })),
    ];
    return r.sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0));
  }, [h]);

  const unlink = async (c: store.PrepaidCharge) => {
    if (
      !(await confirm({
        title: "충전 연결을 해제할까요?",
        danger: true,
        confirmLabel: "연결 해제",
        message: "거래는 지출로 그대로 남고, 이 선불권의 적립에서만 빠집니다.",
        detail: `${c.date} ${won(c.paid)}`,
      }))
    )
      return;
    try {
      await UnlinkPrepaidCharge(c.transactionId);
      load();
      onChanged();
    } catch (e: any) {
      setErr(String(e));
    }
  };
  const delUse = async (u: store.PrepaidUse) => {
    if (
      !(await confirm({
        title: "사용 기록을 삭제할까요?",
        danger: true,
        confirmLabel: "삭제",
        detail: `${u.date} ${u.memo || ""} −${won(u.deducted)}`,
      }))
    )
      return;
    try {
      await DeletePrepaidUse(u.id);
      load();
      onChanged();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  return (
    <Modal title={`${p.name} · 내역`} icon={History} onClose={onClose} wide>
      {confirmUI}
      <div className="mb-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
        {[
          { l: "충전 결제", v: status.totalPaid, c: "text-rose-300" },
          { l: "적립", v: status.totalCredit, c: "text-emerald-300" },
          { l: "사용(차감)", v: status.totalUsed, c: "text-sky-300" },
          { l: "잔액", v: status.balance, c: status.balance < 0 ? "text-rose-300" : "text-emerald-300" },
        ].map((x) => (
          <div key={x.l} className="surface-inset rounded-xl p-3">
            <div className="label-mono">{x.l}</div>
            <div className={`num mt-1 text-base font-semibold ${x.c}`}>{won(x.v)}</div>
          </div>
        ))}
      </div>
      {err && <p className="mb-2 text-sm text-rose-300">{err}</p>}
      {!h && !err && <p className="label-mono">불러오는 중…</p>}
      {h && rows.length === 0 && <p className="label-mono">아직 충전·사용 내역이 없습니다.</p>}
      {rows.length > 0 && (
        <ul className="space-y-1.5">
          {rows.map((r) =>
            r.kind === "charge" ? (
              <li key={`c${r.c.transactionId}`} className="surface-inset flex items-center justify-between gap-3 rounded-xl px-3 py-2">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="badge bg-emerald-500/15 text-emerald-300">충전</span>
                    <span className="num text-sm text-white">{r.date}</span>
                  </div>
                  <div className="label-mono mt-1 whitespace-normal">
                    결제 <span className="num">{won(r.c.paid)}</span>
                    {r.c.paymentMethodName ? ` · ${r.c.paymentMethodName}` : ""}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                  <span className="num text-sm font-semibold text-emerald-300">+{won(r.c.credit)}</span>
                  <button type="button" className="btn-ghost !px-2.5 !py-1 text-xs" onClick={() => unlink(r.c)}>
                    연결 해제
                  </button>
                </div>
              </li>
            ) : (
              <li key={`u${r.u.id}`} className="surface-inset flex items-center justify-between gap-3 rounded-xl px-3 py-2">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="badge bg-sky-500/10 text-sky-300">사용</span>
                    <span className="num text-sm text-white">{r.date}</span>
                    {r.u.memo && <span className="text-sm text-zinc-300">{r.u.memo}</span>}
                  </div>
                  <div className="label-mono mt-1 whitespace-normal">
                    {r.u.listPrice > r.u.deducted && (
                      <>
                        정가 <span className="num">{won(r.u.listPrice)}</span> ·{" "}
                        <span className="text-emerald-300">
                          <span className="num">{won(r.u.listPrice - r.u.deducted)}</span> 절약
                        </span>
                        {" · "}
                      </>
                    )}
                    {r.u.memberName || "귀속자 없음"}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <span className="num mr-2 text-sm font-semibold text-sky-300">−{won(r.u.deducted)}</span>
                  <button type="button" className="btn-icon" title="수정" aria-label="수정" onClick={() => onEditUse(r.u)}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button type="button" className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() => delUse(r.u)}>
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </li>
            )
          )}
        </ul>
      )}
    </Modal>
  );
}

// ---------- 섹션 ----------
type ModalState =
  | { t: "pass"; pass: store.PrepaidPass | null }
  | { t: "use"; id: number; use: store.PrepaidUse | null }
  | { t: "charge"; id: number }
  | { t: "history"; id: number }
  | null;

export default function PrepaidSection({ refs }: { refs: Refs }) {
  const [list, setList] = useState<store.PrepaidStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState<ModalState>(null);
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(async () => {
    try {
      setList((await ListPrepaidStatuses()) ?? []);
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  const sorted = useMemo(
    () => [...list].sort((a, b) => Number(!a.pass.active) - Number(!b.pass.active)),
    [list]
  );
  const find = (id: number) => list.find((s) => s.pass.id === id);
  const close = useCallback(() => setModal(null), []);

  const remove = async (s: store.PrepaidStatus) => {
    if (
      !(await confirm({
        title: `"${s.pass.name}" 선불권을 삭제할까요?`,
        danger: true,
        confirmLabel: "삭제",
        message: "충전 거래는 지출로 그대로 남고, 선불권과 차감 기록만 지워집니다.",
      }))
    )
      return;
    try {
      await DeletePrepaidPass(s.pass.id);
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const useSt = modal && modal.t !== "pass" ? find(modal.id) : undefined;

  return (
    <div className="space-y-3">
      {confirmUI}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={Ticket} tone="sky" />
          <div className="min-w-0">
            <h3 className="section-title">선불권 · 회원권</h3>
            <p className="label-mono num">{loading ? "불러오는 중…" : `${list.length} passes`}</p>
          </div>
        </div>
        <button className="btn-primary" onClick={() => setModal({ t: "pass", pass: null })}>
          <Plus className="h-4 w-4" />
          선불권 추가
        </button>
      </div>
      {err && <p className="text-sm text-rose-300">{err}</p>}
      {!loading && !err && list.length === 0 && (
        <p className="label-mono whitespace-normal">
          등록된 선불권이 없습니다. 미용실 회원권처럼 미리 충전해 두고 쓸 때마다 차감하는 결제를 관리할 수 있어요.
        </p>
      )}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {sorted.map((s) => {
          const p = s.pass;
          return (
            <div key={p.id} className={`surface p-5 ${p.active ? "" : "opacity-60"}`}>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <strong className="block truncate text-sm font-semibold text-white">{p.name}</strong>
                  {p.merchant && <span className="label-mono block truncate">{p.merchant}</span>}
                </div>
                <div className="flex shrink-0 gap-1">
                  <button className="btn-icon" title="수정" aria-label="수정" onClick={() => setModal({ t: "pass", pass: p })}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() => remove(s)}>
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="mt-3 flex flex-wrap items-center gap-1.5">
                {hasBonus(p.discountMode) && p.bonusBp > 0 && (
                  <span className="chip">
                    충전 보너스 <span className="num">{bpToPct(p.bonusBp)}%</span>
                  </span>
                )}
                {hasDiscount(p.discountMode) && p.discountBp > 0 && (
                  <span className="chip">
                    차감 할인 <span className="num">{bpToPct(p.discountBp)}%</span>
                  </span>
                )}
                {s.low && <span className="badge bg-amber-500/15 text-amber-300">재충전 필요</span>}
                {s.expired && <span className="badge bg-rose-500/15 text-rose-300">만료</span>}
                {!p.active && <span className="badge bg-white/[0.06] text-zinc-300">사용 종료</span>}
              </div>

              <div className="mt-4">
                <div className="label-mono">잔액</div>
                <CountUp
                  value={s.balance}
                  format={won}
                  className={`num text-2xl font-semibold ${s.balance < 0 ? "text-rose-300" : s.low ? "text-amber-300" : "text-emerald-300"}`}
                />
              </div>

              <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-1.5 text-xs">
                <Stat k="누적 절약" v={<span className="text-emerald-300">{won(s.saved)}</span>} />
                <Stat k="사용" v={s.useCount > 0 ? `${s.useCount}회 · 평균 ${won(s.avgUseAmount)}` : "-"} />
                <Stat k="남은 횟수" v={s.usesLeft > 0 ? `약 ${s.usesLeft}회` : "-"} />
                <Stat k="소진 예상" v={s.runOutDate || "-"} />
                <Stat k="마지막 사용" v={s.lastUse || "-"} />
                <Stat k="유효기간" v={p.expiresAt || "없음"} />
              </dl>

              <div className="mt-4 flex flex-wrap gap-2">
                <button className="btn-primary" onClick={() => setModal({ t: "use", id: p.id, use: null })}>
                  <MinusCircle className="h-4 w-4" />
                  사용 기록
                </button>
                <button className="btn-ghost" onClick={() => setModal({ t: "charge", id: p.id })}>
                  <PlusCircle className="h-4 w-4" />
                  충전
                </button>
                <button className="btn-ghost" onClick={() => setModal({ t: "history", id: p.id })}>
                  <History className="h-4 w-4" />
                  내역
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {modal?.t === "pass" && <PassForm pass={modal.pass} refs={refs} onClose={close} onSaved={load} />}
      {modal?.t === "use" && useSt && <UseModal status={useSt} use={modal.use} refs={refs} onClose={close} onSaved={load} />}
      {modal?.t === "charge" && useSt && <ChargeModal status={useSt} refs={refs} onClose={close} onSaved={load} />}
      {modal?.t === "history" && useSt && (
        <HistoryModal
          status={useSt}
          onClose={close}
          onChanged={load}
          onEditUse={(u) => setModal({ t: "use", id: useSt.pass.id, use: u })}
        />
      )}
    </div>
  );
}

function Stat({ k, v }: { k: string; v: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="label-mono">{k}</dt>
      <dd className="num truncate text-zinc-200">{v}</dd>
    </div>
  );
}
