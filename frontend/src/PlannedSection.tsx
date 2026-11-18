import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  CancelPlannedExpense,
  DeletePlannedExpense,
  ListBaskets,
  ListPlannedExpenses,
  PlannedCandidates,
  SavePlannedExpense,
  SettlePlannedExpense,
} from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { CalendarClock, Check, Pencil, Plus, Trash2, Undo2, X } from "lucide-react";
import IconTile from "./components/ui/IconTile";
import Modal, { Field } from "./components/ui/Modal";
import { useConfirm } from "./ConfirmModal";
import { categoryOptions, formatAmount, parseAmount, Refs, today, won } from "./lib";

// ---------- 예정 지출 ----------
// 정기는 아니지만 날짜와 금액을 미리 아는 일회성 지출(결혼식 축의금 · 돌잔치 · 수리 예약 등).
// 생활비 통장에서 낼 돈은 자금 계획 출금 일정에, 바구니에서 낼 돈은 바구니 카드에 '예정'으로 들어간다.
// 날짜가 지나도 실제 거래와 연결(냈어요)하거나 '안 냄'으로 끝낼 때까지 계획에 남는다(store/planned.go).

const shortDate = (d: string) => (d ? `${Number(d.slice(5, 7))}/${Number(d.slice(8, 10))}` : "");
function dday(d: string): { text: string; past: boolean } {
  const n = Math.round((new Date(d).getTime() - new Date(today()).getTime()) / 86400000);
  return { text: n === 0 ? "오늘" : n > 0 ? `D-${n}` : `${-n}일 지남`, past: n < 0 };
}
function addDays(d: string, n: number): string {
  const t = new Date(d);
  t.setDate(t.getDate() + n);
  return t.toISOString().slice(0, 10);
}
// 이름에 이런 말이 있으면 경조사 카테고리를 미리 고른다
const GIFT_WORDS = ["축의", "부의", "결혼", "장례", "돌잔치", "조의", "경조"];

type Source = { id: number; name: string; pmId?: number; catIds: number[] };

function PlannedForm({
  item,
  refs,
  sources,
  onClose,
  onSaved,
}: {
  item: store.PlannedExpense | null;
  refs: Refs;
  sources: Source[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [f, setF] = useState(() => ({
    date: item?.date ?? addDays(today(), 7),
    name: item?.name ?? "",
    amount: item ? formatAmount(String(item.amount)) : "",
    basketId: item?.basketId ? String(item.basketId) : "",
    categoryId: item?.categoryId ? String(item.categoryId) : "",
    memberId: item?.memberId ? String(item.memberId) : "",
    memo: item?.memo ?? "",
  }));
  const [touchedCat, setTouchedCat] = useState(!!item?.categoryId);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const set = (p: Partial<typeof f>) => setF((prev) => ({ ...prev, ...p }));

  // 카테고리를 직접 고르기 전에는 이름 · 바구니로 짐작해 미리 고른다
  const guessCat = useMemo(() => {
    const src = sources.find((s) => String(s.id) === f.basketId);
    if (src && src.catIds.length) return String(src.catIds[0]);
    if (GIFT_WORDS.some((w) => f.name.includes(w))) {
      const c = refs.categories.find((x) => x.kind === "expense" && !x.parentId && x.name.includes("경조사"));
      if (c) return String(c.id);
    }
    return "";
  }, [f.basketId, f.name, sources, refs.categories]);
  const categoryId = touchedCat ? f.categoryId : guessCat || f.categoryId;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!f.name.trim()) return setErr("무슨 지출인지 이름을 입력하세요.");
    if (!(parseAmount(f.amount) > 0)) return setErr("금액을 입력하세요.");
    setBusy(true);
    try {
      await SavePlannedExpense(
        new store.PlannedExpense({
          ...(item ?? {}),
          id: item?.id ?? 0,
          date: f.date,
          name: f.name.trim(),
          amount: parseAmount(f.amount),
          basketId: f.basketId ? Number(f.basketId) : undefined,
          categoryId: categoryId ? Number(categoryId) : undefined,
          memberId: f.memberId ? Number(f.memberId) : undefined,
          memo: f.memo,
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
    <Modal title={item ? "예정 지출 고치기" : "예정 지출 추가"} icon={CalendarClock} tone="amber" onClose={onClose} onSubmit={submit}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="무슨 지출" className="sm:col-span-2">
          <input className="input-field" placeholder="예: 김○○ 결혼식 축의금" autoFocus value={f.name} onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <Field label="날짜">
          <input type="date" className="input-field num" value={f.date} onChange={(e) => set({ date: e.target.value })} />
        </Field>
        <Field label="금액">
          <input className="input-field num" inputMode="numeric" placeholder="0" value={f.amount} onChange={(e) => set({ amount: formatAmount(e.target.value) })} />
        </Field>
        <Field label="어디서 낼 돈" className="sm:col-span-2">
          <select className="input-field" value={f.basketId} onChange={(e) => set({ basketId: e.target.value })}>
            <option value="">생활비 통장 (자금 계획 출금 일정에 넣음)</option>
            {sources.map((s) => (
              <option key={s.id} value={s.id}>
                바구니: {s.name} (바구니 잔액에서 미리 빼 둠)
              </option>
            ))}
          </select>
        </Field>
        <Field label="카테고리">
          <select
            className="input-field"
            value={categoryId}
            onChange={(e) => {
              setTouchedCat(true);
              set({ categoryId: e.target.value });
            }}
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
      </div>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>{item ? "저장" : "추가"}</button>
      </div>
    </Modal>
  );
}

function SettleModal({
  item,
  source,
  refs,
  onClose,
  onSaved,
}: {
  item: store.PlannedExpense;
  source?: Source;
  refs: Refs;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [cands, setCands] = useState<store.Transaction[] | null>(null);
  const [pick, setPick] = useState(0);
  const [moved, setMoved] = useState(true);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    PlannedCandidates(item.id)
      .then((c) => {
        setCands(c ?? []);
        if (c && c.length) setPick(c[0].id);
      })
      .catch((e) => setErr(String(e)));
  }, [item.id]);

  // 바구니 통장으로 바로 냈으면 옮겨 올 돈이 없다
  const picked = cands?.find((t) => t.id === pick);
  useEffect(() => {
    if (source && picked) setMoved(!(source.pmId && picked.paymentMethodId === source.pmId));
  }, [source, picked]);

  const pmName = (id?: number) => refs.paymentMethods.find((p) => p.id === id)?.name ?? "";

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!pick) return setErr("연결할 거래를 고르세요.");
    setBusy(true);
    try {
      await SettlePlannedExpense(item.id, pick, moved);
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={`${item.name} — 냈어요`} icon={Check} tone="amber" onClose={onClose} onSubmit={submit}>
      <p className="mb-3 text-sm text-zinc-400">
        <span className="num">{item.date}</span> 예정 · <span className="num text-white">{won(item.amount)}</span> — 실제로 낸 거래를 고르세요(예정일 앞뒤 2주)
      </p>
      {cands === null ? (
        <p className="text-sm text-zinc-500">불러오는 중…</p>
      ) : cands.length === 0 ? (
        <p className="text-sm text-zinc-500">예정일 앞뒤 2주에 지출이 없습니다. 거래내역에 먼저 넣고 다시 오세요.</p>
      ) : (
        <div className="max-h-72 divide-y divide-white/[0.06] overflow-y-auto">
          {cands.map((t) => (
            <label key={t.id} className="flex cursor-pointer items-center gap-3 py-2 text-sm">
              <input type="radio" name="tx" checked={pick === t.id} onChange={() => setPick(t.id)} />
              <span className="num w-12 shrink-0 text-zinc-400">{shortDate(t.date)}</span>
              <span className="min-w-0 flex-1 truncate text-zinc-200">
                {t.merchant}
                {t.paymentMethodId ? <span className="ml-2 text-xs text-zinc-500">{pmName(t.paymentMethodId)}</span> : null}
              </span>
              <span className={`num shrink-0 ${t.amount === item.amount ? "text-white" : "text-zinc-400"}`}>{won(t.amount)}</span>
            </label>
          ))}
        </div>
      )}
      {source && (
        <label className="mt-3 flex items-start gap-2 text-sm text-zinc-300">
          <input type="checkbox" className="mt-0.5" checked={moved} onChange={(e) => setMoved(e.target.checked)} />
          <span>
            바구니 '{source.name}'의 돈을 생활비 통장으로 옮겨 옴
            <span className="block text-xs text-zinc-500">바구니 통장에서 바로 냈으면 끄세요. 바구니 꺼냄도 함께 기록합니다</span>
          </span>
        </label>
      )}
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy || !cands?.length}>연결</button>
      </div>
    </Modal>
  );
}

type ModalState = { t: "form"; item: store.PlannedExpense | null } | { t: "settle"; item: store.PlannedExpense } | null;

export default function PlannedSection({ refs, onChanged }: { refs: Refs; onChanged: () => void }) {
  const [list, setList] = useState<store.PlannedExpense[]>([]);
  const [sources, setSources] = useState<Source[]>([]);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState<ModalState>(null);
  const [showDone, setShowDone] = useState(false);
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(async () => {
    try {
      const [l, b] = await Promise.all([ListPlannedExpenses(), ListBaskets()]);
      setList(l ?? []);
      setSources(
        (b ?? [])
          .filter((s) => s.basket.active)
          .map((s) => ({ id: s.basket.id, name: s.basket.name, pmId: s.basket.paymentMethodId, catIds: s.basket.categoryIds ?? [] }))
      );
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    }
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  const changed = useCallback(() => {
    load();
    onChanged(); // 출금 일정 · 잔액 예상을 다시 계산
  }, [load, onChanged]);

  const open = list.filter((p) => p.status === "planned");
  const closed = list.filter((p) => p.status !== "planned").reverse();
  const mainTotal = open.filter((p) => !p.basketId).reduce((a, p) => a + p.amount, 0);
  const basketTotal = open.filter((p) => p.basketId).reduce((a, p) => a + p.amount, 0);

  const act = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
      changed();
    } catch (e: any) {
      setErr(String(e));
    }
  };
  const remove = async (p: store.PlannedExpense) => {
    if (!(await confirm({ title: `"${p.name}"을(를) 지울까요?`, danger: true, confirmLabel: "지우기", message: "연결한 거래와 바구니 기록은 그대로 남습니다." })))
      return;
    act(() => DeletePlannedExpense(p.id));
  };

  return (
    <div className="surface p-5 sm:p-6">
      {confirmUI}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={CalendarClock} tone="amber" size="sm" />
          <div className="min-w-0">
            <h3 className="section-title">예정 지출</h3>
            <p className="label-mono num !normal-case">
              {open.length
                ? `${open.length}건 · 생활비에서 ${won(mainTotal)}${basketTotal ? ` · 바구니에서 ${won(basketTotal)}` : ""}`
                : "날짜를 아는 일회성 지출 — 축의금 · 돌잔치 · 수리 예약"}
            </p>
          </div>
        </div>
        <button className="btn-ghost" onClick={() => setModal({ t: "form", item: null })}>
          <Plus className="h-4 w-4" />
          예정 지출 추가
        </button>
      </div>
      {err && <p className="mb-3 text-sm text-rose-300">{err}</p>}
      {open.length === 0 ? (
        <p className="text-sm text-zinc-500">
          결혼식 축의금처럼 날짜를 아는 지출을 미리 넣어 두면, 생활비에서 낼 돈은 출금 일정에, 바구니에서 낼 돈은 그 바구니에 미리 잡힙니다.
        </p>
      ) : (
        <div className="divide-y divide-white/[0.06]">
          {open.map((p) => {
            const d = dday(p.date);
            return (
              <div key={p.id} className="flex flex-wrap items-center gap-x-3 gap-y-1.5 py-2.5 text-sm">
                <span className="num w-12 shrink-0 text-zinc-300">{shortDate(p.date)}</span>
                <span className={`num w-16 shrink-0 text-xs ${d.past ? "text-amber-300" : "text-zinc-500"}`}>{d.text}</span>
                <span className="min-w-0 flex-1">
                  <span className="text-white">{p.name}</span>
                  <span className="ml-2 text-xs text-zinc-500">{p.basketName ? `바구니: ${p.basketName}` : "생활비 통장"}</span>
                </span>
                <span className="num shrink-0 text-white">{won(p.amount)}</span>
                <span className="flex shrink-0 gap-1">
                  <button type="button" className="btn-primary !py-1 text-xs" onClick={() => setModal({ t: "settle", item: p })} title="실제로 낸 거래와 연결">
                    냈어요
                  </button>
                  <button type="button" className="btn-ghost !py-1 text-xs" onClick={() => act(() => CancelPlannedExpense(p.id, false))} title="안 내게 됨 — 계획에서 뺍니다">
                    안 냄
                  </button>
                  <button type="button" className="btn-icon" aria-label="고치기" title="고치기" onClick={() => setModal({ t: "form", item: p })}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button type="button" className="btn-icon hover:!text-rose-300" aria-label="지우기" title="지우기" onClick={() => remove(p)}>
                    <Trash2 className="h-4 w-4" />
                  </button>
                </span>
              </div>
            );
          })}
        </div>
      )}

      {closed.length > 0 && (
        <div className="mt-3">
          <button type="button" className="text-xs text-zinc-500 underline" onClick={() => setShowDone((v) => !v)}>
            {showDone ? "끝낸 것 숨기기" : `끝낸 것 ${closed.length}건 보기`}
          </button>
          {showDone && (
            <div className="mt-2 divide-y divide-white/[0.06]">
              {closed.map((p) => (
                <div key={p.id} className="flex items-center gap-3 py-2 text-xs text-zinc-400">
                  <span className="num w-12 shrink-0">{shortDate(p.date)}</span>
                  <span className="min-w-0 flex-1 truncate">
                    {p.name}
                    {p.status === "done" ? (
                      <span className="ml-2 text-emerald-300">
                        <Check className="inline h-3 w-3" /> {shortDate(p.txDate)} {p.txMerchant} {won(p.txAmount)}
                      </span>
                    ) : (
                      <span className="ml-2 text-zinc-500">
                        <X className="inline h-3 w-3" /> 안 냄
                      </span>
                    )}
                  </span>
                  {p.status === "cancelled" && (
                    <button type="button" className="btn-icon" aria-label="되돌리기" title="다시 예정으로" onClick={() => act(() => CancelPlannedExpense(p.id, true))}>
                      <Undo2 className="h-3.5 w-3.5" />
                    </button>
                  )}
                  <span className="num shrink-0">{won(p.amount)}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {modal?.t === "form" && <PlannedForm item={modal.item} refs={refs} sources={sources} onClose={() => setModal(null)} onSaved={changed} />}
      {modal?.t === "settle" && (
        <SettleModal
          item={modal.item}
          source={sources.find((s) => s.id === modal.item.basketId)}
          refs={refs}
          onClose={() => setModal(null)}
          onSaved={changed}
        />
      )}
    </div>
  );
}
