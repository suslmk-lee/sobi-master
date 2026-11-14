import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  AddPaymentMethod,
  DeleteSubscription,
  ListTransit,
  SaveSubscription,
  SaveTransitMonth,
} from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { CreditCard, Pencil, Plus, TrainFront, Trash2 } from "lucide-react";
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

// ---------- 교통카드 (K-패스) ----------
// 매일 타는 교통카드는 탈 때마다 기록하지 않고 한 달에 한 번 본다: 매달 출금일에 한 달 요금이 빠지고
// 같은 날 K-패스 환급이 들어온다. 가계부에는 실부담(요금 − 환급) 한 건만 남는다.
// 출금일이 지나면 앱이 최근 확정한 달들로 어림한 '추정' 기록을 만들고, 실제 금액을 넣으면 추정 표시가 사라진다.
// 출금 통장이 카드값 통장과 달라 자금 계획의 잔액·출금 일정에는 넣지 않는다.

const pad2 = (n: number) => String(n).padStart(2, "0");
const thisYM = () => today().slice(0, 7);
function addYM(ym: string, n: number): string {
  const [y, m] = ym.split("-").map(Number);
  const t = y * 12 + (m - 1) + n;
  return `${Math.floor(t / 12)}-${pad2((t % 12) + 1)}`;
}
const monthLabel = (ym: string) => `${Number(ym.slice(5, 7))}월`;
const shortDate = (d: string) => (d ? `${Number(d.slice(5, 7))}/${Number(d.slice(8, 10))}` : "");
function dday(d: string): string {
  const n = Math.round((new Date(d).getTime() - new Date(today()).getTime()) / 86400000);
  return n === 0 ? "오늘" : n > 0 ? `D-${n}` : `${-n}일 지남`;
}
const idOrUndef = (s: string) => (s ? Number(s) : undefined);

const HOLIDAY_RULES: [string, string][] = [
  ["next", "다음 영업일"],
  ["prev", "이전 영업일"],
  ["none", "그날 그대로"],
];

// ---------- 교통카드 등록/수정 ----------
function TransitForm({
  sub,
  refs,
  reloadRefs,
  onClose,
  onSaved,
}: {
  sub: store.Subscription | null;
  refs: Refs;
  reloadRefs: () => Promise<void>;
  onClose: () => void;
  onSaved: () => void;
}) {
  // 새 교통카드는 '교통' 들어간 카테고리를 기본으로(기본 카테고리 "교통/차량")
  const defaultCat = useMemo(() => {
    if (sub) return sub.categoryId ? String(sub.categoryId) : "";
    const c = refs.categories.find((x) => x.kind === "expense" && x.name.includes("교통"));
    return c ? String(c.id) : "";
  }, [sub, refs.categories]);
  const [f, setF] = useState(() => ({
    name: sub?.name ?? "K-패스",
    amount: sub ? formatAmount(String(sub.amount)) : "",
    day: String(sub?.billingDay || 10),
    holidayRule: sub?.holidayRule || "next",
    categoryId: defaultCat,
    memberId: sub?.memberId ? String(sub.memberId) : "",
    paymentMethodId: sub?.paymentMethodId ? String(sub.paymentMethodId) : "",
    memo: sub?.memo ?? "",
    active: sub?.active ?? true,
  }));
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const set = (p: Partial<typeof f>) => setF((prev) => ({ ...prev, ...p }));

  // 아직 카드를 등록하지 않았으면 여기서 바로: 결제일 없이 만들어 카드값 계산에서 빠지게 한다
  const addCard = async () => {
    setErr("");
    try {
      const pm = await AddPaymentMethod("K뱅크 K-패스", "card");
      await reloadRefs();
      set({ paymentMethodId: String(pm.id) });
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    const amount = parseAmount(f.amount) || 0;
    if (!f.name.trim()) return setErr("이름을 입력하세요.");
    if (amount <= 0) return setErr("한 달 예상 요금을 입력하세요.");
    setBusy(true);
    try {
      await SaveSubscription(
        new store.Subscription({
          ...(sub ?? {}),
          id: sub?.id ?? 0,
          kind: "transit",
          name: f.name.trim(),
          merchant: sub?.merchant ?? "",
          amount,
          cycle: "monthly",
          billingDay: Math.min(31, Math.max(1, Number(f.day) || 1)),
          billingMonth: 0,
          dayRule: "day",
          weekNth: sub?.weekNth ?? 1,
          weekday: sub?.weekday ?? 5,
          holidayRule: f.holidayRule,
          startYm: "",
          endYm: "",
          nextAmount: 0,
          nextAmountYm: "",
          categoryId: idOrUndef(f.categoryId),
          memberId: idOrUndef(f.memberId),
          paymentMethodId: idOrUndef(f.paymentMethodId),
          memo: f.memo,
          active: f.active,
          autoPost: true,
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
    <Modal title={sub ? "교통카드 수정" : "교통카드 추가"} icon={TrainFront} tone="sky" onClose={onClose} onSubmit={submit}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="이름">
          <input className="input-field" placeholder="예: K-패스" value={f.name} autoFocus onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <Field label="한 달 예상 요금">
          <input className="input-field num" inputMode="numeric" placeholder="0" value={f.amount} onChange={(e) => set({ amount: formatAmount(e.target.value) })} />
          <span className="label-mono mt-1 block whitespace-normal">실제 금액을 한 번 넣고 나면 최근 몇 달 평균으로 어림합니다</span>
        </Field>
        <Field label="출금일" className="sm:col-span-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm text-zinc-400">매월</span>
            <select className="input-field num !w-auto" value={f.day} onChange={(e) => set({ day: e.target.value })}>
              {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                <option key={d} value={d}>{d}일</option>
              ))}
            </select>
            <span className="text-sm text-zinc-400">· 주말·공휴일이면</span>
            <select className="input-field !w-auto" value={f.holidayRule} onChange={(e) => set({ holidayRule: e.target.value })}>
              {HOLIDAY_RULES.map(([v, l]) => (
                <option key={v} value={v}>{l}</option>
              ))}
            </select>
          </div>
          <span className="label-mono mt-1 block whitespace-normal">한 달 요금이 빠지고 K-패스 환급이 들어오는 날</span>
        </Field>
        <Field label="결제수단 (교통카드)" className="sm:col-span-2">
          <div className="flex gap-2">
            <select className="input-field min-w-0 flex-1" value={f.paymentMethodId} onChange={(e) => set({ paymentMethodId: e.target.value })}>
              <option value="">선택 안 함</option>
              {selectableMethods(refs.paymentMethods, f.paymentMethodId).map((p) => (
                <option key={p.id} value={p.id}>{methodLabel(p)}</option>
              ))}
            </select>
            {!f.paymentMethodId && (
              <button type="button" className="btn-ghost shrink-0" onClick={addCard}>
                <CreditCard className="h-4 w-4" />
                K뱅크 카드 등록
              </button>
            )}
          </div>
          {isClosed(refs.paymentMethods.find((p) => String(p.id) === f.paymentMethodId)) && (
            <p className="mt-1 text-xs text-amber-300">{CLOSED_HINT}</p>
          )}
          <span className="label-mono mt-1 block whitespace-normal">
            출금 통장이 카드값 통장과 달라 자금 계획의 잔액·출금 일정에는 넣지 않습니다
          </span>
        </Field>
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
        <Field label="메모" className="sm:col-span-2">
          <input className="input-field" value={f.memo} onChange={(e) => set({ memo: e.target.value })} />
        </Field>
        {sub && (
          <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" checked={f.active} onChange={(e) => set({ active: e.target.checked })} />
            사용 중 (끄면 출금일이 지나도 기록하지 않습니다)
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

// ---------- 한 달 금액 기록 ----------
function MonthModal({
  st,
  ym: initYM,
  onClose,
  onSaved,
}: {
  st: store.TransitStatus;
  ym: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const rowOf = (ym: string) => (st.months ?? []).find((m) => m.ym === ym);
  const fill = (ym: string) => {
    const r = rowOf(ym);
    return { ym, fare: r ? formatAmount(String(r.fare)) : "", refund: r ? formatAmount(String(r.refund)) : "" };
  };
  const [f, setF] = useState(() => fill(initYM));
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const fare = parseAmount(f.fare) || 0;
  const refund = parseAmount(f.refund) || 0;
  const row = rowOf(f.ym);
  // 이번 달부터 거꾸로 12달
  const months = Array.from({ length: 12 }, (_, i) => addYM(thisYM(), -i));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (refund > fare) return setErr("환급액이 요금보다 클 수 없습니다.");
    setBusy(true);
    try {
      await SaveTransitMonth(st.sub.id, f.ym, fare, refund);
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={`${st.sub.name} ${monthLabel(f.ym)} 금액`} icon={TrainFront} tone="sky" onClose={onClose} onSubmit={submit}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Field label="달">
          <select className="input-field num" value={f.ym} onChange={(e) => setF(fill(e.target.value))}>
            {months.map((m) => (
              <option key={m} value={m}>
                {m.slice(0, 4)}년 {monthLabel(m)}
                {rowOf(m)?.estimated ? " · 추정" : rowOf(m) ? " · 기록함" : ""}
              </option>
            ))}
          </select>
        </Field>
        <Field label="이용 요금">
          <input className="input-field num" inputMode="numeric" placeholder="0" autoFocus value={f.fare} onChange={(e) => setF({ ...f, fare: formatAmount(e.target.value) })} />
        </Field>
        <Field label="K-패스 환급">
          <input className="input-field num" inputMode="numeric" placeholder="0" value={f.refund} onChange={(e) => setF({ ...f, refund: formatAmount(e.target.value) })} />
        </Field>
      </div>
      <div className="surface-inset mt-3 flex flex-wrap items-baseline justify-between gap-2 px-4 py-3">
        <span className="text-sm text-zinc-300">실부담 (가계부에 남는 지출)</span>
        <span className={`num text-xl font-semibold ${refund > fare ? "text-rose-300" : "text-white"}`}>{won(Math.max(0, fare - refund))}</span>
      </div>
      <p className="mt-2 text-xs leading-relaxed text-zinc-500">
        카드 앱에서 그달 출금된 요금과 들어온 환급액을 넣으세요. 안 탄 달은 0원으로 넣으면 지출 없이 기록만 남습니다.
        {row?.estimated && " 지금 값은 앱이 어림한 추정입니다."}
      </p>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>기록</button>
      </div>
    </Modal>
  );
}

type ModalState =
  | { t: "form"; sub: store.Subscription | null }
  | { t: "month"; id: number; ym: string }
  | null;

// 금액을 넣을 기본 달: 아직 추정인 가장 오래된 달 → 이번 달(출금일이 지났으면) → 지난달
function defaultMonth(st: store.TransitStatus): string {
  const pending = (st.months ?? []).filter((m) => m.estimated).map((m) => m.ym).sort();
  if (pending.length) return pending[0];
  return st.nextYm === thisYM() ? addYM(thisYM(), -1) : thisYM();
}

export default function TransitSection({ refs, reloadRefs }: { refs: Refs; reloadRefs: () => Promise<void> }) {
  const [list, setList] = useState<store.TransitStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState<ModalState>(null);
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(async () => {
    try {
      setList((await ListTransit()) ?? []);
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
  const cur = modal?.t === "month" ? list.find((s) => s.sub.id === modal.id) : undefined;
  const pmName = (id?: number) => refs.paymentMethods.find((p) => p.id === id)?.name ?? "";

  const remove = async (st: store.TransitStatus) => {
    if (
      !(await confirm({
        title: `"${st.sub.name}"를 삭제할까요?`,
        danger: true,
        confirmLabel: "삭제",
        message: "기록된 교통비 지출은 거래내역에 그대로 남고, 월 기록(요금·환급)만 지워집니다.",
      }))
    )
      return;
    try {
      await DeleteSubscription(st.sub.id);
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
          <IconTile icon={TrainFront} tone="sky" size="sm" />
          <div className="min-w-0">
            <h3 className="section-title">교통카드 (K-패스)</h3>
            <p className="label-mono">한 달에 한 번, 출금일에 요금 · 환급을 기록</p>
          </div>
        </div>
        <button className="btn-ghost" onClick={() => setModal({ t: "form", sub: null })}>
          <Plus className="h-4 w-4" />
          교통카드 추가
        </button>
      </div>
      {err && <p className="mb-3 text-sm text-rose-300">{err}</p>}
      {!loading && !err && list.length === 0 && (
        <p className="text-sm leading-relaxed text-zinc-500">
          매일 타는 교통카드를 등록하면 탈 때마다 기록하지 않아도 됩니다. 출금일이 지나면 최근 평균으로 그달 교통비를
          어림해 기록하고, 카드 앱에서 본 실제 요금과 K-패스 환급을 넣으면 고쳐집니다. 가계부에는 환급을 뺀 실부담만 남습니다.
        </p>
      )}

      <div className="grid gap-3 lg:grid-cols-2">
        {list.map((st) => {
          const s = st.sub;
          const off = !s.active;
          const months = st.months ?? [];
          const last = months[0];
          return (
            <div key={s.id} className={`surface-inset flex flex-col rounded-2xl p-4 ${off ? "opacity-60" : ""}`}>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <strong className="block truncate text-sm font-semibold text-white">{s.name}</strong>
                  <span className="label-mono num block truncate">
                    {[pmName(s.paymentMethodId), `매월 ${s.billingDay}일 출금`, off ? "꺼 둠" : ""].filter(Boolean).join(" · ")}
                  </span>
                </div>
                <div className="flex shrink-0 gap-1">
                  <button className="btn-icon" title="수정" aria-label="수정" onClick={() => setModal({ t: "form", sub: s })}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() => remove(st)}>
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="mt-4">
                {last ? (
                  <>
                    <div className="label-mono flex items-center gap-2">
                      {monthLabel(last.ym)} 실부담 · {shortDate(last.date)} 출금
                      {last.estimated && <span className="badge bg-amber-500/15 !normal-case text-amber-300">추정</span>}
                    </div>
                    <div className="num text-2xl font-semibold text-white">{won(last.net)}</div>
                    <p className="num mt-1 text-xs text-zinc-400">
                      요금 {won(last.fare)} · 환급 {won(last.refund)}
                    </p>
                  </>
                ) : (
                  <p className="text-sm text-zinc-500">아직 기록이 없습니다. 출금일이 지나면 그달 교통비를 어림해 기록합니다.</p>
                )}
                {!off && st.nextYm && (
                  <p className="num mt-3 text-xs leading-relaxed text-zinc-300">
                    다음 {shortDate(st.nextDate)} 출금 <span className="text-zinc-500">({dday(st.nextDate)})</span> · 예상 요금{" "}
                    {won(st.estFare)}
                    {st.basis > 0 ? ` · 환급 약 ${won(st.estRefund)}` : ""}
                    <br />
                    <span className="text-zinc-400">출금 통장에 {won(st.estFare)} 이상 두세요</span>
                    {st.basis === 0 && <span className="text-zinc-500"> · 환급은 첫 실제 기록 뒤부터 어림합니다</span>}
                  </p>
                )}
              </div>

              {months.length > 1 && (
                <table className="mt-4 w-full text-xs">
                  <thead>
                    <tr className="label-mono text-left">
                      <th className="py-1 font-normal">달</th>
                      <th className="py-1 text-right font-normal">요금</th>
                      <th className="py-1 text-right font-normal">환급</th>
                      <th className="py-1 text-right font-normal">실부담</th>
                    </tr>
                  </thead>
                  <tbody>
                    {months.slice(0, 6).map((m) => (
                      <tr
                        key={m.ym}
                        className="cursor-pointer border-t border-white/[0.05] hover:bg-white/[0.03]"
                        onClick={() => setModal({ t: "month", id: s.id, ym: m.ym })}
                        title="눌러서 고치기"
                      >
                        <td className="num py-1.5 text-zinc-300">
                          {monthLabel(m.ym)}
                          {m.estimated && <span className="ml-1.5 text-amber-300">추정</span>}
                        </td>
                        <td className="num py-1.5 text-right text-zinc-300">{won(m.fare)}</td>
                        <td className="num py-1.5 text-right text-zinc-400">{won(m.refund)}</td>
                        <td className="num py-1.5 text-right text-white">{won(m.net)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}

              <div className="mt-4 flex flex-wrap gap-2">
                <button className="btn-primary" onClick={() => setModal({ t: "month", id: s.id, ym: defaultMonth(st) })}>
                  <Pencil className="h-4 w-4" />
                  {st.pending > 0 ? "실제 금액 넣기" : "월 금액 기록"}
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {modal?.t === "form" && (
        <TransitForm sub={modal.sub} refs={refs} reloadRefs={reloadRefs} onClose={close} onSaved={load} />
      )}
      {modal?.t === "month" && cur && <MonthModal st={cur} ym={modal.ym} onClose={close} onSaved={load} />}
    </div>
  );
}
