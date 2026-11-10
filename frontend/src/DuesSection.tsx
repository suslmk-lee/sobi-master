import { FormEvent, ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import {
  AddDuesPayment,
  ClearDuesLink,
  DeleteSubscription,
  ListDues,
  SaveSubscription,
  SetDuesLink,
} from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { History, Pencil, Plus, Trash2, Users, Wallet } from "lucide-react";
import IconTile from "./components/ui/IconTile";
import Modal, { Field } from "./components/ui/Modal";
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

// ---------- 회비 ----------
// 친목회·동호회처럼 매달 정해진 돈을 내는 회비. 정기결제처럼 달마다 대조하지 않고 장부로 본다:
// "어디까지 냈나"에서 출발해 그 뒤에 낸 돈으로 다음 달부터 채우고, 납부일이 지난 빈 달이 미납이다.

const pad2 = (n: number) => String(n).padStart(2, "0");

// "YYYY-MM" 에 n 개월을 더한다.
function addYM(ym: string, n: number): string {
  const y = Number(ym.slice(0, 4));
  const m = Number(ym.slice(5, 7));
  const d = new Date(y, m - 1 + n, 1);
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}`;
}

const thisYM = () => today().slice(0, 7);

// "2026-07" → "7월" (올해가 아니면 "2025년 12월")
function ymLabel(ym: string): string {
  if (!ym) return "";
  const y = ym.slice(0, 4);
  const m = Number(ym.slice(5, 7));
  return y === thisYM().slice(0, 4) ? `${m}월` : `${y}년 ${m}월`;
}

// 기간 → "7~11월분", 한 달이면 "11월분", 해가 다르면 "2025년 12월~2026년 2월분"
function rangeLabel(from: string, to: string): string {
  if (!from) return "";
  if (!to || from === to) return `${ymLabel(from)}분`;
  if (from.slice(0, 4) === to.slice(0, 4)) {
    const y = from.slice(0, 4) === thisYM().slice(0, 4) ? "" : `${from.slice(0, 4)}년 `;
    return `${y}${Number(from.slice(5, 7))}~${Number(to.slice(5, 7))}월분`;
  }
  return `${from.slice(0, 4)}년 ${Number(from.slice(5, 7))}월~${to.slice(0, 4)}년 ${Number(to.slice(5, 7))}월분`;
}

const shortDate = (d: string) => (d ? `${Number(d.slice(5, 7))}/${Number(d.slice(8, 10))}` : "");

function dday(d: string): string {
  const a = new Date(`${today()}T00:00:00`).getTime();
  const b = new Date(`${d}T00:00:00`).getTime();
  const n = Math.round((b - a) / 86400000);
  return n === 0 ? "오늘" : n > 0 ? `D-${n}` : `D+${-n}`;
}

// 그 달 회비(요금 변경 예약 반영) — 백엔드 Subscription.AmountAt 과 같다
const feeAt = (s: store.Subscription, ym: string) =>
  s.nextAmountYm && ym >= s.nextAmountYm ? s.nextAmount : s.amount;

// 지금 장부에 extra 를 더 내면 어디까지 채워지는지(백엔드 BuildDuesStatus 1단계와 같은 계산)
function simulate(st: store.DuesStatus, extra: number) {
  const s = st.sub;
  let pool = st.credit + extra;
  let through = st.paidThrough;
  let ym = addYM(through, 1);
  const from = ym;
  for (let i = 0; i < 240; i++) {
    if (s.endYm && ym > s.endYm) break;
    const fee = feeAt(s, ym);
    if (fee <= 0 || pool < fee) break;
    pool -= fee;
    through = ym;
    ym = addYM(ym, 1);
  }
  return { from, through, rest: pool };
}

// 첫 미납 달부터 k 개월치(남은 돈은 빼고)
function monthsAmount(st: store.DuesStatus, k: number): number {
  let sum = 0;
  let ym = addYM(st.paidThrough, 1);
  for (let i = 0; i < k; i++) {
    if (st.sub.endYm && ym > st.sub.endYm) break;
    sum += feeAt(st.sub, ym);
    ym = addYM(ym, 1);
  }
  return Math.max(0, sum - st.credit);
}

// 연·월 고르기(네이티브 select 두 개 — 팝업 안에서도 잘리지 않는다)
function YmSelect({ value, onChange, label }: { value: string; onChange: (v: string) => void; label: string }) {
  const y = Number(value.slice(0, 4)) || new Date().getFullYear();
  const m = Number(value.slice(5, 7)) || 1;
  const cur = new Date().getFullYear();
  const years: number[] = [];
  for (let i = Math.min(cur - 5, y); i <= Math.max(cur + 3, y); i++) years.push(i);
  return (
    <div className="flex items-center gap-2" role="group" aria-label={label}>
      <select className="input-field num !w-auto" value={y} onChange={(e) => onChange(`${e.target.value}-${pad2(m)}`)} aria-label={`${label} 연도`}>
        {years.map((v) => (
          <option key={v} value={v}>{v}년</option>
        ))}
      </select>
      <select className="input-field num !w-auto" value={m} onChange={(e) => onChange(`${y}-${pad2(Number(e.target.value))}`)} aria-label={`${label} 월`}>
        {Array.from({ length: 12 }, (_, i) => i + 1).map((v) => (
          <option key={v} value={v}>{v}월</option>
        ))}
      </select>
    </div>
  );
}

const idOrUndef = (s: string) => (s ? Number(s) : undefined);

// ---------- 회비 등록/수정 ----------
function DuesForm({
  sub,
  refs,
  onClose,
  onSaved,
}: {
  sub: store.Subscription | null;
  refs: Refs;
  onClose: () => void;
  onSaved: () => void;
}) {
  // 새 회비는 '회비' 들어간 카테고리를 기본으로(기본 카테고리 "회비/경조사")
  const defaultCat = useMemo(() => {
    if (sub) return sub.categoryId ? String(sub.categoryId) : "";
    const c = refs.categories.find((x) => x.kind === "expense" && x.name.includes("회비"));
    return c ? String(c.id) : "";
  }, [sub, refs.categories]);
  const [f, setF] = useState(() => ({
    name: sub?.name ?? "",
    merchant: sub?.merchant ?? "",
    amount: sub ? formatAmount(String(sub.amount)) : "",
    day: String(sub?.billingDay || 10),
    paidThrough: sub?.paidThrough || addYM(thisYM(), -1),
    categoryId: defaultCat,
    memberId: sub?.memberId ? String(sub.memberId) : "",
    paymentMethodId: sub?.paymentMethodId ? String(sub.paymentMethodId) : "",
    changeOn: !!sub?.nextAmountYm,
    nextAmount: sub?.nextAmount ? formatAmount(String(sub.nextAmount)) : "",
    nextAmountYm: sub?.nextAmountYm || addYM(thisYM(), 1),
    endOn: !!sub?.endYm,
    endYm: sub?.endYm || thisYM(),
    memo: sub?.memo ?? "",
    active: sub?.active ?? true,
  }));
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const set = (p: Partial<typeof f>) => setF((prev) => ({ ...prev, ...p }));
  const throughChanged = !!sub && sub.paidThrough !== f.paidThrough;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    const amount = parseAmount(f.amount) || 0;
    if (!f.name.trim()) return setErr("모임 이름을 입력하세요.");
    if (amount <= 0) return setErr("월 회비를 입력하세요.");
    setBusy(true);
    try {
      await SaveSubscription(
        new store.Subscription({
          ...(sub ?? {}),
          id: sub?.id ?? 0,
          kind: "dues",
          name: f.name.trim(),
          merchant: f.merchant.trim(),
          amount,
          cycle: "monthly",
          billingDay: Math.min(31, Math.max(1, Number(f.day) || 1)),
          billingMonth: 0,
          dayRule: "day",
          weekNth: sub?.weekNth ?? 1,
          weekday: sub?.weekday ?? 5,
          // 회비는 주말이어도 그날까지 내면 된다
          holidayRule: sub?.holidayRule || "none",
          paidThrough: f.paidThrough,
          startYm: "",
          endYm: f.endOn ? f.endYm : "",
          nextAmount: f.changeOn ? parseAmount(f.nextAmount) || 0 : 0,
          nextAmountYm: f.changeOn ? f.nextAmountYm : "",
          categoryId: idOrUndef(f.categoryId),
          memberId: idOrUndef(f.memberId),
          paymentMethodId: idOrUndef(f.paymentMethodId),
          memo: f.memo,
          active: f.active,
          autoPost: false,
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
    <Modal title={sub ? "회비 수정" : "회비 추가"} icon={Users} tone="amber" onClose={onClose} onSubmit={submit}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="모임 이름">
          <input className="input-field" placeholder="예: 아파트 모임" value={f.name} autoFocus onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <Field label="받는 사람">
          <input className="input-field" placeholder="비우면 모임 이름으로 찾습니다" value={f.merchant} onChange={(e) => set({ merchant: e.target.value })} />
          <span className="label-mono mt-1 block whitespace-normal">이체하면 거래내역에 찍히는 이름(총무·모임통장). 이 이름으로 낸 돈을 찾습니다</span>
        </Field>
        <Field label="월 회비">
          <input className="input-field num" inputMode="numeric" placeholder="0" value={f.amount} onChange={(e) => set({ amount: formatAmount(e.target.value) })} />
        </Field>
        <Field label="납부일">
          <div className="flex items-center gap-2">
            <span className="text-sm text-zinc-400">매월</span>
            <select className="input-field num !w-auto" value={f.day} onChange={(e) => set({ day: e.target.value })}>
              {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                <option key={d} value={d}>{d}일</option>
              ))}
            </select>
            <span className="text-sm text-zinc-400">까지</span>
          </div>
        </Field>
        <div className="sm:col-span-2">
          <span className="field-label">어디까지 냈나요</span>
          <div className="flex flex-wrap items-center gap-2">
            <YmSelect label="낸 마지막 달" value={f.paidThrough} onChange={(v) => set({ paidThrough: v })} />
            <span className="text-sm text-zinc-300">분까지 냈음</span>
          </div>
          <p className={`label-mono mt-1.5 whitespace-normal ${throughChanged ? "!text-amber-300" : ""}`}>
            {throughChanged
              ? "바꾸면 오늘부터 다시 셉니다. 지금까지 기록된 납부는 이 값에 들어 있는 것으로 봅니다."
              : sub
              ? `${sub.paidThroughAt}에 밝힌 값입니다. 그 뒤에 낸 돈은 장부에 더해집니다.`
              : "지금까지 낸 마지막 달을 고르세요. 오늘 이후 낸 돈부터 장부에 더합니다. 밀렸다면 마지막으로 낸 달을 고르면 미납이 계산됩니다."}
          </p>
        </div>
        <Field label="카테고리">
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
        <Field label="귀속자">
          <select className="input-field" value={f.memberId} onChange={(e) => set({ memberId: e.target.value })}>
            <option value="">선택 안 함</option>
            {refs.members.map((m) => (
              <option key={m.id} value={m.id}>{m.name}</option>
            ))}
          </select>
        </Field>
        <Field label="결제수단" className="sm:col-span-2">
          <select className="input-field" value={f.paymentMethodId} onChange={(e) => set({ paymentMethodId: e.target.value })}>
            <option value="">선택 안 함</option>
            {selectableMethods(refs.paymentMethods, f.paymentMethodId).map((p) => (
              <option key={p.id} value={p.id}>{methodLabel(p)}</option>
            ))}
          </select>
          {isClosed(refs.paymentMethods.find((p) => String(p.id) === f.paymentMethodId)) && (
            <p className="mt-1 text-xs text-amber-300">{CLOSED_HINT}</p>
          )}
        </Field>

        <div className="surface-inset flex flex-wrap items-center gap-3 px-3.5 py-2.5 sm:col-span-2">
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input type="checkbox" className="accent-mint-500" checked={f.changeOn} onChange={(e) => set({ changeOn: e.target.checked })} />
            회비가 바뀜
          </label>
          {f.changeOn ? (
            <>
              <YmSelect label="바뀌는 달" value={f.nextAmountYm} onChange={(v) => set({ nextAmountYm: v })} />
              <span className="text-sm text-zinc-400">부터</span>
              <input
                className="input-field num !w-32"
                inputMode="numeric"
                placeholder="0"
                value={f.nextAmount}
                onChange={(e) => set({ nextAmount: formatAmount(e.target.value) })}
                aria-label="바뀐 회비"
              />
            </>
          ) : (
            <span className="text-xs text-zinc-500">인상·인하가 정해졌으면 그 달부터 새 금액으로 셉니다</span>
          )}
        </div>
        <div className="surface-inset flex flex-wrap items-center gap-3 px-3.5 py-2.5 sm:col-span-2">
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input type="checkbox" className="accent-mint-500" checked={f.endOn} onChange={(e) => set({ endOn: e.target.checked })} />
            그만둠
          </label>
          {f.endOn ? (
            <>
              <YmSelect label="마지막 달" value={f.endYm} onChange={(v) => set({ endYm: v })} />
              <span className="text-sm text-zinc-400">분까지 내고 끝</span>
            </>
          ) : (
            <span className="text-xs text-zinc-500">탈퇴하거나 모임이 끝나면 마지막으로 낼 달을 정합니다</span>
          )}
        </div>

        <Field label="메모" className="sm:col-span-2">
          <input className="input-field" value={f.memo} onChange={(e) => set({ memo: e.target.value })} />
        </Field>
        {sub && (
          <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" checked={f.active} onChange={(e) => set({ active: e.target.checked })} />
            사용 중 (끄면 미납·다음 납부를 계산하지 않고 자금 계획에서도 빠집니다)
          </label>
        )}
      </div>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>{sub ? "저장" : "추가"}</button>
      </div>
    </Modal>
  );
}

// ---------- 납부 기록 ----------
function PayModal({ st, onClose, onSaved }: { st: store.DuesStatus; onClose: () => void; onSaved: () => void }) {
  const s = st.sub;
  const fee = feeAt(s, addYM(st.paidThrough, 1)); // 첫 미납 달 회비
  const [date, setDate] = useState(today());
  const [amount, setAmount] = useState(formatAmount(String(st.overdue > 0 ? st.overdue : st.nextDue || st.fee)));
  const [memo, setMemo] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const paid = parseAmount(amount) || 0;
  const sim = simulate(st, paid);
  const filled = sim.through !== st.paidThrough;
  const autoMemo = filled ? `${s.name} ${rangeLabel(sim.from, sim.through)}` : `${s.name} 회비`;

  // 빠른 금액: 밀린 것 전부, 1·3·6·12개월치
  const quick = useMemo(() => {
    const out: { label: string; v: number }[] = [];
    if (st.overdueMonths > 0) out.push({ label: `밀린 ${st.overdueMonths}개월`, v: st.overdue });
    for (const k of [1, 3, 6, 12]) {
      if (k === st.overdueMonths) continue;
      const v = monthsAmount(st, k);
      if (v > 0 && !out.some((o) => o.v === v)) out.push({ label: `${k}개월`, v });
    }
    return out;
  }, [st]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!date || paid <= 0) return setErr("날짜와 금액을 입력하세요.");
    setBusy(true);
    try {
      await AddDuesPayment(s.id, date, paid, memo.trim() || autoMemo);
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  // 낸 뒤에도 남는 미납(납부일이 지난 달 중 못 채운 달)
  const stillOverdue = st.overdueMonths > 0 && st.overdueTo > sim.through;

  return (
    <Modal title={`${s.name} · 납부 기록`} icon={Wallet} tone="amber" onClose={onClose} onSubmit={submit}>
      <p className="mb-4 text-sm text-zinc-400">
        지금 <span className="text-zinc-200">{ymLabel(st.paidThrough)}분</span>까지 냄
        {st.overdueMonths > 0 && (
          <>
            {" · "}
            <span className="num text-rose-300">미납 {st.overdueMonths}개월 {won(st.overdue)}</span>
          </>
        )}
        {" · "}월 <span className="num">{won(fee)}</span>
      </p>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="낸 날">
          <input type="date" className="input-field" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
        <Field label="낸 금액">
          <input className="input-field num" inputMode="numeric" autoFocus value={amount} placeholder="0" onChange={(e) => setAmount(formatAmount(e.target.value))} />
        </Field>
      </div>
      {quick.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {quick.map((q) => (
            <button
              key={q.label}
              type="button"
              className={`chip num ${paid === q.v ? "!border-mint-400/50 !text-mint-200" : ""}`}
              onClick={() => setAmount(formatAmount(String(q.v)))}
            >
              {q.label} {won(q.v)}
            </button>
          ))}
        </div>
      )}
      <div className="surface-inset mt-4 rounded-xl p-3 text-sm">
        {paid <= 0 ? (
          <span className="text-zinc-500">금액을 넣으면 어디까지 채워지는지 보여 드립니다.</span>
        ) : filled ? (
          <>
            <span className="text-zinc-300">
              {rangeLabel(sim.from, sim.through)} → <strong className="font-semibold text-emerald-300">{ymLabel(sim.through)}분까지</strong> 냄
            </span>
            {sim.rest > 0 && <span className="num ml-1 text-zinc-400">· 남는 {won(sim.rest)}은 다음 달 몫으로</span>}
            {stillOverdue && <span className="ml-1 text-amber-300">· 아직 {ymLabel(addYM(sim.through, 1))}분부터 밀림</span>}
          </>
        ) : (
          <span className="num text-zinc-400">한 달치({won(feeAt(s, sim.from))})에 못 미쳐 다음 달 몫으로 모아 둡니다.</span>
        )}
      </div>
      <Field label="메모" className="mt-3">
        <input className="input-field" value={memo} placeholder={autoMemo} onChange={(e) => setMemo(e.target.value)} />
      </Field>
      <p className="mt-3 text-xs leading-relaxed text-zinc-500">
        받는 사람·카테고리·귀속자·결제수단은 회비 설정대로 지출 거래가 됩니다. 계좌 이체가 거래내역에 들어오면
        받는 사람 이름으로 저절로 반영되니, 여기서는 현금으로 냈거나 거래내역에 없는 납부만 기록하세요.
      </p>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>기록</button>
      </div>
    </Modal>
  );
}

// ---------- 내역 ----------
function HistoryModal({ st, onClose, onChanged }: { st: store.DuesStatus; onClose: () => void; onChanged: () => void }) {
  const s = st.sub;
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(0);
  const pays = st.payments ?? [];
  const review = pays.filter((p) => p.state === "review");
  const rest = pays.filter((p) => p.state !== "review");

  const act = async (p: store.DuesPayment, f: () => Promise<void>) => {
    setErr("");
    setBusy(p.transactionId);
    try {
      await f();
      onChanged();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(0);
    }
  };
  const include = (p: store.DuesPayment) => act(p, () => SetDuesLink(s.id, p.transactionId, "include"));
  const exclude = (p: store.DuesPayment) => act(p, () => SetDuesLink(s.id, p.transactionId, "exclude"));
  const reset = (p: store.DuesPayment) => act(p, () => ClearDuesLink(p.transactionId));

  const row = (p: store.DuesPayment, actions: ReactNode) => (
    <li
      key={p.transactionId}
      className={`surface-inset flex items-center justify-between gap-3 rounded-xl px-3 py-2 ${p.state === "excluded" ? "opacity-60" : ""}`}
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="num text-sm text-white">{p.date}</span>
          {p.state === "counted" && (
            <span className="badge bg-emerald-500/15 text-emerald-300">{p.manual ? "직접 넣음" : "자동 반영"}</span>
          )}
          {p.state === "excluded" && <span className="badge bg-white/[0.06] text-zinc-300">회비 아님</span>}
          {p.state === "review" && <span className="badge bg-amber-500/15 text-amber-300">확인 필요</span>}
        </div>
        <div className="label-mono mt-1 truncate">
          {[p.merchant, p.memo, p.paymentMethodName].filter(Boolean).join(" · ")}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <span className={`num text-sm font-semibold ${p.state === "counted" ? "text-emerald-300" : "text-zinc-300"}`}>
          {won(p.amount)}
        </span>
        {actions}
      </div>
    </li>
  );
  const btn = (label: string, onClick: () => void, id: number) => (
    <button type="button" className="btn-ghost !px-2.5 !py-1 text-xs" disabled={busy === id} onClick={onClick}>
      {label}
    </button>
  );

  return (
    <Modal title={`${s.name} · 내역`} icon={History} tone="amber" onClose={onClose} wide>
      <div className="mb-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
        {[
          { l: "시작", v: `${ymLabel(s.paidThrough)}분까지`, c: "text-zinc-200" },
          { l: "그 뒤 낸 돈", v: won(st.paidTotal), c: "text-emerald-300" },
          { l: "지금", v: `${ymLabel(st.paidThrough)}분까지`, c: "text-white" },
          {
            l: "미납",
            v: st.overdueMonths > 0 ? `${st.overdueMonths}개월 ${won(st.overdue)}` : "없음",
            c: st.overdueMonths > 0 ? "text-rose-300" : "text-emerald-300",
          },
        ].map((x) => (
          <div key={x.l} className="surface-inset rounded-xl p-3">
            <div className="label-mono">{x.l}</div>
            <div className={`num mt-1 text-base font-semibold ${x.c}`}>{x.v}</div>
          </div>
        ))}
      </div>
      <p className="mb-4 text-xs leading-relaxed text-zinc-500">
        {s.paidThroughAt}에 {ymLabel(s.paidThrough)}분까지 냈다고 등록했습니다. 그 뒤에 "{s.merchant || s.name}"(으)로
        보낸 돈 중 회비 몇 달치로 딱 떨어지는 금액은 자동으로 반영합니다.
        {st.credit > 0 && <> 한 달치에 못 미치는 {won(st.credit)}은 다음 달 몫으로 모아 두었습니다.</>}
      </p>
      {err && <p className="mb-2 text-sm text-rose-300">{err}</p>}

      {review.length > 0 && (
        <div className="mb-4">
          <h4 className="mb-1 text-sm font-medium text-amber-200">확인이 필요한 이체 {review.length}건</h4>
          <p className="mb-2 text-xs text-zinc-500">
            받는 사람은 맞는데 금액이 회비({won(st.fee)}) 몇 달치로 떨어지지 않아 아직 장부에 넣지 않았습니다. 회식비처럼
            회비가 아니면 "회비 아님"을 누르세요.
          </p>
          <ul className="space-y-1.5">
            {review.map((p) =>
              row(
                p,
                <>
                  {btn("회비로 넣기", () => include(p), p.transactionId)}
                  {btn("회비 아님", () => exclude(p), p.transactionId)}
                </>
              )
            )}
          </ul>
        </div>
      )}

      {rest.length === 0 && review.length === 0 && (
        <p className="label-mono whitespace-normal">아직 낸 기록이 없습니다. [납부 기록]으로 넣거나, 이체가 거래내역에 들어오면 자동으로 반영됩니다.</p>
      )}
      {rest.length > 0 && (
        <ul className="space-y-1.5">
          {rest.map((p) =>
            row(
              p,
              p.state === "excluded"
                ? btn("되돌리기", () => reset(p), p.transactionId)
                : btn("회비 아님", () => exclude(p), p.transactionId)
            )
          )}
        </ul>
      )}
      <p className="mt-4 text-xs text-zinc-500">납부 거래 자체를 지우려면 거래내역에서 삭제하세요.</p>
    </Modal>
  );
}

// ---------- 섹션 ----------
type ModalState =
  | { t: "form"; sub: store.Subscription | null }
  | { t: "pay"; id: number }
  | { t: "history"; id: number }
  | null;

export default function DuesSection({ refs }: { refs: Refs }) {
  const [list, setList] = useState<store.DuesStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState<ModalState>(null);
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(async () => {
    try {
      setList((await ListDues()) ?? []);
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

  const close = useCallback(() => setModal(null), []);
  const find = (id: number) => list.find((d) => d.sub.id === id);
  const cur = modal && modal.t !== "form" ? find(modal.id) : undefined;

  const active = list.filter((d) => d.sub.active && !d.ended);
  const monthly = active.reduce((a, d) => a + d.fee, 0);
  const overdue = active.reduce((a, d) => a + d.overdue, 0);

  const remove = async (d: store.DuesStatus) => {
    if (
      !(await confirm({
        title: `"${d.sub.name}" 회비를 삭제할까요?`,
        danger: true,
        confirmLabel: "삭제",
        message: "낸 회비 거래는 지출로 그대로 남고, 회비 장부만 지워집니다.",
      }))
    )
      return;
    try {
      await DeleteSubscription(d.sub.id);
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  return (
    <div className="surface p-5">
      {confirmUI}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={Users} tone="amber" size="sm" />
          <div className="min-w-0">
            <h3 className="section-title">회비</h3>
            <p className="label-mono num">
              {loading
                ? "불러오는 중…"
                : active.length === 0
                ? "친목회·동호회 월 회비"
                : `월 ${won(monthly)}${overdue > 0 ? ` · 미납 ${won(overdue)}` : " · 미납 없음"}`}
            </p>
          </div>
        </div>
        <button className="btn-ghost" onClick={() => setModal({ t: "form", sub: null })}>
          <Plus className="h-4 w-4" />
          회비 추가
        </button>
      </div>
      {err && <p className="mb-3 text-sm text-rose-300">{err}</p>}
      {!loading && !err && list.length === 0 && (
        <p className="text-sm leading-relaxed text-zinc-500">
          친목회·동호회처럼 매달 내는 회비를 등록하면, 어디까지 냈는지와 밀린 달을 장부로 보여 드립니다.
          몇 달치를 한 번에 내도 나눠서 채우고, 밀린 돈은 자금 계획의 다음 납부일에 넣습니다.
        </p>
      )}

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {list.map((d) => {
          const s = d.sub;
          const reviewN = (d.payments ?? []).filter((p) => p.state === "review").length;
          const off = !s.active;
          return (
            <div key={s.id} className={`surface-inset flex flex-col rounded-2xl p-4 ${off || d.ended ? "opacity-60" : ""}`}>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <strong className="block truncate text-sm font-semibold text-white">{s.name}</strong>
                  <span className="label-mono num block truncate">
                    {[s.merchant, `매월 ${s.billingDay}일`, `월 ${won(d.fee)}`].filter(Boolean).join(" · ")}
                  </span>
                </div>
                <div className="flex shrink-0 gap-1">
                  <button className="btn-icon" title="수정" aria-label="수정" onClick={() => setModal({ t: "form", sub: s })}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() => remove(d)}>
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="mt-4 flex-1">
                {d.overdueMonths > 0 ? (
                  <>
                    <div className="label-mono !text-rose-300">미납 {d.overdueMonths}개월</div>
                    <div className="num text-2xl font-semibold text-rose-300">{won(d.overdue)}</div>
                    <p className="num mt-1 text-xs text-zinc-400">
                      {rangeLabel(d.overdueFrom, d.overdueTo)} · {ymLabel(d.paidThrough)}분까지 냄
                    </p>
                  </>
                ) : (
                  <>
                    <div className="label-mono">납부 현황</div>
                    <div className={`text-2xl font-semibold ${d.aheadMonths > 0 ? "text-emerald-300" : "text-white"}`}>
                      {ymLabel(d.paidThrough)}분까지 냄
                    </div>
                    <p className="num mt-1 text-xs text-zinc-400">
                      {d.ended
                        ? "마지막 달까지 다 냈습니다"
                        : off
                        ? "꺼 둠 · 미납을 계산하지 않습니다"
                        : d.aheadMonths > 0
                        ? `${d.aheadMonths}개월 미리 냄`
                        : "밀린 달 없음"}
                    </p>
                  </>
                )}
                {!off && !d.ended && d.nextYm && (
                  <p className="num mt-2 text-xs text-zinc-300">
                    다음 {shortDate(d.nextDate)}까지 {ymLabel(d.nextYm)}분 {won(d.nextDue)}{" "}
                    <span className="text-zinc-500">({dday(d.nextDate)})</span>
                  </p>
                )}
                {reviewN > 0 && (
                  <button
                    type="button"
                    className="badge mt-2 bg-amber-500/15 text-amber-300 hover:bg-amber-500/25"
                    onClick={() => setModal({ t: "history", id: s.id })}
                  >
                    확인이 필요한 이체 {reviewN}건
                  </button>
                )}
              </div>

              <div className="mt-4 flex flex-wrap gap-2">
                <button className="btn-primary" onClick={() => setModal({ t: "pay", id: s.id })} disabled={d.ended}>
                  <Wallet className="h-4 w-4" />
                  납부 기록
                </button>
                <button className="btn-ghost" onClick={() => setModal({ t: "history", id: s.id })}>
                  <History className="h-4 w-4" />
                  내역
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {modal?.t === "form" && <DuesForm sub={modal.sub} refs={refs} onClose={close} onSaved={load} />}
      {modal?.t === "pay" && cur && <PayModal st={cur} onClose={close} onSaved={load} />}
      {modal?.t === "history" && cur && <HistoryModal st={cur} onClose={close} onChanged={load} />}
    </div>
  );
}
