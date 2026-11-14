import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  AddBasketEntry,
  AddPaymentMethod,
  DeleteBasket,
  DeleteBasketEntry,
  DismissBasketCandidate,
  GetCashPlan,
  ListBasketCandidates,
  ListBaskets,
  PreviewBasket,
  SaveBasket,
} from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import { ArrowDownLeft, History, Pencil, PiggyBank, Plus, Trash2, X } from "lucide-react";
import IconTile from "../components/ui/IconTile";
import Modal, { Field, Segmented } from "../components/ui/Modal";
import { useConfirm } from "../ConfirmModal";
import { categoryOptions, formatAmount, methodLabel, parseAmount, Refs, today, won } from "../lib";

// ---------- 목돈 바구니 ----------
// 갑자기 나가는 지출(경조사 · 자동차 수리 · 부모님 병원비)과 해마다 정해진 큰 지출(재산세 · 자동차세 · 자동차보험)을
// 미리 조금씩 모아 두는 바구니. 바구니마다 자기 통장이 있고, 잔액은 바구니 장부(채움 · 더 넣기 · 꺼냄 · 잔액 맞춤)로 따라간다.
// 생활비 통장에서 바구니로 보낸 돈과 바구니에서 옮겨 온 돈은 자금 계획 잔액에 반영된다(store/baskets.go).

const KIND_LABEL: Record<string, string> = { scheduled: "정해진 연간 지출", reserve: "예비비" };
const ENTRY_LABEL: Record<string, string> = { open: "처음 잔액", fill: "채움", extra: "더 넣기", spend: "꺼냄", adjust: "잔액 맞춤" };
const MONTHS = Array.from({ length: 12 }, (_, i) => i + 1);
const DAYS = Array.from({ length: 31 }, (_, i) => i + 1);

const shortDate = (d: string) => (d ? `${Number(d.slice(5, 7))}/${Number(d.slice(8, 10))}` : "");
const monthOf = (ym: string) => `${Number(ym.slice(5, 7))}월`;
const signedWon = (n: number) => (n > 0 ? "+" : n < 0 ? "−" : "") + won(Math.abs(n));

type ItemRow = { id: number; name: string; month: number; day: number; amount: string };
// catHint: 새 바구니에 미리 이어 둘 카테고리(이름에 이 말이 들어간 주 카테고리)
type Draft = { name: string; kind: "scheduled" | "reserve"; items?: ItemRow[]; catHint?: string };

const TEMPLATES: { label: string; desc: string; draft: () => Draft }[] = [
  {
    label: "정해진 연간 지출",
    desc: "재산세 · 자동차세 · 자동차보험",
    draft: () => ({
      name: "정해진 연간 지출",
      kind: "scheduled",
      items: [
        { id: 0, name: "재산세", month: 7, day: 0, amount: "" },
        { id: 0, name: "재산세", month: 9, day: 0, amount: "" },
        { id: 0, name: "자동차세(연납)", month: 1, day: 0, amount: "" },
        { id: 0, name: "자동차보험", month: 0, day: 0, amount: "" },
      ],
    }),
  },
  { label: "생활 예비비", desc: "경조사 · 자동차 수리", draft: () => ({ name: "생활 예비비", kind: "reserve", catHint: "경조사" }) },
  { label: "부모님 병원비", desc: "크고 갑작스러운 의료비", draft: () => ({ name: "부모님 병원비", kind: "reserve", catHint: "의료" }) },
];

const bankMethods = (refs: Refs, cur: string) =>
  refs.paymentMethods.filter((p) => p.type === "bank" && (!p.closedAt || String(p.id) === cur));

// ---------- 바구니 만들기 / 고치기 ----------
function BasketForm({
  basket,
  draft,
  refs,
  reloadRefs,
  onClose,
  onSaved,
}: {
  basket: store.Basket | null;
  draft: Draft | null;
  refs: Refs;
  reloadRefs: () => Promise<void>;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [f, setF] = useState(() => ({
    name: basket?.name ?? draft?.name ?? "",
    kind: (basket?.kind ?? draft?.kind ?? "reserve") as "scheduled" | "reserve",
    pmId: basket?.paymentMethodId ? String(basket.paymentMethodId) : "",
    opening: "",
    monthly: basket ? formatAmount(String(basket.monthly)) : "",
    fillDay: basket?.fillDay || 10,
    autoFill: basket?.autoFill ?? true,
    cap: basket?.cap ? formatAmount(String(basket.cap)) : "",
    items: (basket?.items ?? []).map((i) => ({ id: i.id, name: i.name, month: i.month, day: i.day, amount: formatAmount(String(i.amount)) })) as ItemRow[],
    memo: basket?.memo ?? "",
    active: basket?.active ?? true,
    catIds: (basket?.categoryIds ??
      refs.categories
        .filter((c) => draft?.catHint && c.kind === "expense" && !c.parentId && c.name.includes(draft.catHint))
        .map((c) => c.id)) as number[],
  }));
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<store.BasketStatus | null>(null);
  const set = (p: Partial<typeof f>) => setF((prev) => ({ ...prev, ...p }));

  // 새 바구니: 템플릿 항목과 월급날(채우는 날 기본값)
  useEffect(() => {
    if (basket) return;
    if (draft?.items) set({ items: draft.items });
    GetCashPlan()
      .then((p) => p.settings?.salaryDay && set({ fillDay: p.settings.salaryDay }))
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toBasket = useCallback(
    () =>
      new store.Basket({
        ...(basket ?? {}),
        id: basket?.id ?? 0,
        name: f.name.trim(),
        kind: f.kind,
        paymentMethodId: f.pmId ? Number(f.pmId) : undefined,
        cap: f.kind === "reserve" ? parseAmount(f.cap) || 0 : 0,
        monthly: parseAmount(f.monthly) || 0,
        autoFill: f.autoFill,
        fillDay: f.fillDay,
        active: f.active,
        memo: f.memo,
        categoryIds: f.catIds,
        items:
          f.kind === "scheduled"
            ? f.items.map((i) => ({ id: i.id, basketId: basket?.id ?? 0, name: i.name.trim(), month: i.month, day: i.day, amount: parseAmount(i.amount) || 0 }))
            : [],
      }),
    [f, basket]
  );

  // 권장 월 적립액 · 목표까지 걸리는 달을 바로 보여 준다
  useEffect(() => {
    const t = window.setTimeout(() => {
      PreviewBasket(toBasket(), basket ? 0 : parseAmount(f.opening) || 0)
        .then(setPreview)
        .catch(() => setPreview(null));
    }, 300);
    return () => window.clearTimeout(t);
  }, [toBasket, basket, f.opening]);

  const addAccount = async () => {
    setErr("");
    try {
      const pm = await AddPaymentMethod(`${f.name.trim() || "바구니"} 통장`, "bank");
      await reloadRefs();
      set({ pmId: String(pm.id) });
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const setItem = (i: number, p: Partial<ItemRow>) => set({ items: f.items.map((it, k) => (k === i ? { ...it, ...p } : it)) });

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!f.name.trim()) return setErr("바구니 이름을 입력하세요.");
    if (f.kind === "scheduled") {
      for (const it of f.items) {
        if (!it.name.trim()) return setErr("항목 이름을 입력하세요.");
        if (it.month < 1) return setErr(`${it.name}: 몇 월에 내는지 고르세요.`);
        if (!(parseAmount(it.amount) > 0)) return setErr(`${it.name}: 금액을 입력하세요.`);
      }
    }
    setBusy(true);
    try {
      await SaveBasket(toBasket(), basket ? 0 : parseAmount(f.opening) || 0);
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const monthly = parseAmount(f.monthly) || 0;
  const rec = preview?.recommended ?? 0;

  return (
    <Modal title={basket ? "바구니 고치기" : "바구니 만들기"} icon={PiggyBank} onClose={onClose} onSubmit={submit} wide>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="이름">
          <input className="input-field" value={f.name} autoFocus={!basket} onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <div>
          <span className="field-label">종류</span>
          <Segmented
            value={f.kind}
            options={[
              { v: "scheduled", label: "정해진 연간 지출" },
              { v: "reserve", label: "예비비" },
            ]}
            onChange={(v) => set({ kind: v })}
          />
          <span className="label-mono mt-1 block whitespace-normal">
            {f.kind === "scheduled" ? "해마다 정해진 달에 나가는 돈 — 마감마다 다 차도록 모읍니다" : "언제 나갈지 모르는 돈 — 목표까지 채워 두고 쓰면 다시 채웁니다"}
          </span>
        </div>
        <Field label="바구니 통장" className="sm:col-span-2">
          <div className="flex gap-2">
            <select className="input-field min-w-0 flex-1" value={f.pmId} onChange={(e) => set({ pmId: e.target.value })}>
              <option value="">선택 안 함</option>
              {bankMethods(refs, f.pmId).map((p) => (
                <option key={p.id} value={p.id}>{methodLabel(p)}</option>
              ))}
            </select>
            {!f.pmId && (
              <button type="button" className="btn-ghost shrink-0" onClick={addAccount}>
                <Plus className="h-4 w-4" />
                통장 등록
              </button>
            )}
          </div>
          <span className="label-mono mt-1 block whitespace-normal">이 통장으로 바로 낸 지출은 자금 계획의 생활비 잔액에서 빠집니다</span>
        </Field>
        {!basket && (
          <Field label="지금 통장에 들어 있는 돈">
            <input className="input-field num" inputMode="numeric" placeholder="0" value={f.opening} onChange={(e) => set({ opening: formatAmount(e.target.value) })} />
          </Field>
        )}
        {f.kind === "reserve" && (
          <Field label="목표 금액 (다 차면 멈춤)">
            <input className="input-field num" inputMode="numeric" placeholder="0 = 정하지 않음" value={f.cap} onChange={(e) => set({ cap: formatAmount(e.target.value) })} />
          </Field>
        )}
        <Field label="매달 넣는 돈" className={basket && f.kind === "scheduled" ? "sm:col-span-2" : ""}>
          <div className="flex flex-wrap items-center gap-2">
            <input className="input-field num !w-40" inputMode="numeric" placeholder="0" value={f.monthly} onChange={(e) => set({ monthly: formatAmount(e.target.value) })} />
            <span className="text-sm text-zinc-400">매월</span>
            <select className="input-field num !w-auto" value={f.fillDay} onChange={(e) => set({ fillDay: Number(e.target.value) })}>
              {DAYS.map((d) => (
                <option key={d} value={d}>{d}일</option>
              ))}
            </select>
          </div>
        </Field>
        <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
          <input type="checkbox" checked={f.autoFill} onChange={(e) => set({ autoFill: e.target.checked })} />
          통장 자동이체를 걸어 둠 (그날 앱이 채움을 기록하고, 자금 계획 출금 일정에 넣습니다)
        </label>

        {f.kind === "scheduled" && (
          <div className="sm:col-span-2">
            <span className="field-label">항목 (해마다 내는 돈)</span>
            <div className="space-y-2">
              {f.items.map((it, i) => (
                <div key={i} className="flex flex-wrap items-center gap-2">
                  <input className="input-field min-w-0 flex-1" placeholder="예: 재산세" value={it.name} onChange={(e) => setItem(i, { name: e.target.value })} />
                  <select className="input-field num !w-auto" value={it.month} onChange={(e) => setItem(i, { month: Number(e.target.value) })} aria-label="달">
                    <option value={0}>달</option>
                    {MONTHS.map((m) => (
                      <option key={m} value={m}>{m}월</option>
                    ))}
                  </select>
                  <select className="input-field num !w-auto" value={it.day} onChange={(e) => setItem(i, { day: Number(e.target.value) })} aria-label="마감일">
                    <option value={0}>말일까지</option>
                    {DAYS.map((d) => (
                      <option key={d} value={d}>{d}일까지</option>
                    ))}
                  </select>
                  <input className="input-field num !w-36" inputMode="numeric" placeholder="금액" value={it.amount} onChange={(e) => setItem(i, { amount: formatAmount(e.target.value) })} />
                  <button type="button" className="btn-icon" aria-label="항목 빼기" onClick={() => set({ items: f.items.filter((_, k) => k !== i) })}>
                    <X className="h-4 w-4" />
                  </button>
                </div>
              ))}
              <button type="button" className="btn-ghost" onClick={() => set({ items: [...f.items, { id: 0, name: "", month: 0, day: 0, amount: "" }] })}>
                <Plus className="h-4 w-4" />
                항목 추가
              </button>
            </div>
          </div>
        )}

        {preview && (
          <div className="surface-inset space-y-1 px-4 py-3 text-sm sm:col-span-2">
            {f.kind === "scheduled" ? (
              <>
                <p className="text-zinc-300">
                  1년 합계 <span className="num text-white">{won(preview.annualTotal)}</span> · 앞으로의 마감을 모두 맞추려면 매달{" "}
                  <span className="num font-semibold text-white">{won(rec)}</span>
                  {rec > monthly && (
                    <button type="button" className="ml-2 text-xs text-mint-300 underline" onClick={() => set({ monthly: formatAmount(String(rec)) })}>
                      이 금액으로
                    </button>
                  )}
                </p>
                {preview.short > 0 ? (
                  <p className="num text-xs text-amber-300">
                    지금 금액이면 {preview.shortName} {monthOf(preview.shortYm)}까지 {won(preview.short)} 모자랍니다
                  </p>
                ) : (
                  preview.annualTotal > 0 && <p className="text-xs text-zinc-500">지금 금액으로 마감을 모두 맞춥니다</p>
                )}
                {preview.annualTotal > 0 && (
                  <p className="num text-xs text-zinc-500">해마다 평균으로는 매달 {won(Math.ceil(preview.annualTotal / 12 / 1000) * 1000)}이면 됩니다</p>
                )}
              </>
            ) : preview.basket.cap > 0 ? (
              <p className="num text-zinc-300">
                {preview.capReached
                  ? "이미 목표를 채웠습니다"
                  : preview.monthsToCap > 0
                  ? `지금 금액으로 ${preview.monthsToCap}개월 뒤 목표를 채웁니다`
                  : "매달 넣는 돈을 정하면 목표까지 걸리는 달을 보여 드립니다"}
              </p>
            ) : (
              <p className="text-zinc-500">목표 금액을 정하면 다 찼을 때 알려 드립니다</p>
            )}
          </div>
        )}

        <div className="sm:col-span-2">
          <span className="field-label">이 바구니에서 낼 지출의 카테고리</span>
          <div className="flex flex-wrap gap-1.5">
            {categoryOptions(refs.categories, "expense").map((c) => {
              const on = f.catIds.includes(c.id);
              return (
                <button
                  key={c.id}
                  type="button"
                  aria-pressed={on}
                  className={`chip !text-xs ${on ? "!border-mint-400/50 !text-mint-300" : ""}`}
                  onClick={() => set({ catIds: on ? f.catIds.filter((x) => x !== c.id) : [...f.catIds, c.id] })}
                >
                  {c.label}
                </button>
              );
            })}
          </div>
          <span className="label-mono mt-1 block whitespace-normal">
            {f.kind === "scheduled"
              ? "정해진 지출은 항목 이름(재산세 등)으로도 찾습니다. 이 카테고리의 5만 원 이상 지출을 '연결할 지출'로 보여 드립니다"
              : "이 카테고리의 5만 원 이상 지출을 '연결할 지출'로 보여 드리고, 지난 1년 기록으로 매달 넣을 돈을 제안합니다"}
          </span>
        </div>
        {f.kind === "reserve" && preview && preview.suggestMonthly > 0 && (
          <p className="num text-sm text-zinc-300 sm:col-span-2">
            지난 12개월 이 카테고리의 큰 지출 {preview.suggestCount}건으로 보면 매달 <span className="font-semibold text-white">{won(preview.suggestMonthly)}</span>
            {preview.suggestMonthly !== monthly && (
              <button type="button" className="ml-2 text-xs text-mint-300 underline" onClick={() => set({ monthly: formatAmount(String(preview.suggestMonthly)) })}>
                이 금액으로
              </button>
            )}
          </p>
        )}

        <Field label="메모" className="sm:col-span-2">
          <input className="input-field" value={f.memo} onChange={(e) => set({ memo: e.target.value })} />
        </Field>
        {basket && (
          <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" checked={f.active} onChange={(e) => set({ active: e.target.checked })} />
            사용 중 (끄면 채우지 않고 알림도 띄우지 않습니다)
          </label>
        )}
      </div>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>{basket ? "저장" : "만들기"}</button>
      </div>
    </Modal>
  );
}

// ---------- 더 넣기 / 꺼내 쓰기 / 잔액 맞춤 ----------
type EntryMode = "extra" | "spend" | "adjust";

// 정해진 지출 바구니에서 꺼낼 때 고를 마감(안 낸 것 먼저). 거래 이름에 항목 이름이 들어 있으면 그걸 미리 고른다.
function dueOptions(st: store.BasketStatus) {
  return (st.dues ?? []).filter((d) => !d.paid);
}
const dueKey = (d: { itemId: number; ym: string }) => `${d.itemId}/${d.ym}`;

export function BasketSpendModal({
  tx,
  prefer,
  onClose,
  onSaved,
}: {
  tx: store.Transaction;
  prefer?: { basketId: number; itemId?: number };
  onClose: () => void;
  onSaved: () => void;
}) {
  const [list, setList] = useState<store.BasketStatus[]>([]);
  const [basketId, setBasketId] = useState(0);
  const [due, setDue] = useState("");
  const [amount, setAmount] = useState(formatAmount(String(tx.amount)));
  const [moved, setMoved] = useState(true);
  const [memo, setMemo] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    ListBaskets()
      .then((l) => {
        const act = (l ?? []).filter((s) => s.basket.active);
        setList(act);
        // 기본 바구니: 그 바구니 통장으로 바로 낸 거래면 그 바구니(옮겨 올 돈도 없다) →
        // 거래 이름에 정해진 항목 이름(재산세 등)이 있으면 그 바구니 → 아니면 예비비(경조사 · 수리 · 병원비)
        const own = act.find((s) => s.basket.paymentMethodId && s.basket.paymentMethodId === tx.paymentMethodId);
        const text = `${tx.merchant} ${tx.memo}`;
        const named = act.find((s) => dueOptions(s).some((d) => text.includes(d.name.replace(/\(.*\)/, "").trim())));
        const preferred = prefer ? act.find((s) => s.basket.id === prefer.basketId) : undefined;
        const pick = preferred ?? own ?? named ?? act.find((s) => s.basket.kind === "reserve") ?? act[0];
        if (pick) setBasketId(pick.basket.id);
        if (own) setMoved(false);
      })
      .catch((e) => setErr(String(e)));
  }, [tx]);

  const cur = list.find((s) => s.basket.id === basketId);
  useEffect(() => {
    if (!cur || cur.basket.kind !== "scheduled") return setDue("");
    const text = `${tx.merchant} ${tx.memo}`;
    const hit =
      (prefer?.itemId ? dueOptions(cur).find((d) => d.itemId === prefer.itemId) : undefined) ??
      dueOptions(cur).find((d) => text.includes(d.name.replace(/\(.*\)/, "").trim()));
    setDue(hit ? dueKey(hit) : "");
  }, [cur, tx, prefer]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!cur) return setErr("바구니를 고르세요.");
    const d = dueOptions(cur).find((x) => dueKey(x) === due);
    setBusy(true);
    try {
      await AddBasketEntry(
        new store.BasketEntry({
          basketId: cur.basket.id,
          kind: "spend",
          amount: parseAmount(amount) || 0,
          date: tx.date,
          transactionId: tx.id,
          itemId: d?.itemId,
          dueYm: d?.ym ?? "",
          moved,
          memo,
        })
      );
      onSaved();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="바구니에서 냄" icon={PiggyBank} onClose={onClose} onSubmit={submit}>
      <p className="mb-3 text-sm text-zinc-400">
        <span className="num">{tx.date}</span> · {tx.merchant} · <span className="num text-white">{won(tx.amount)}</span>
      </p>
      {list.length === 0 ? (
        <p className="text-sm text-zinc-500">아직 바구니가 없습니다. 바구니 탭에서 먼저 만드세요.</p>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field label="바구니">
            <select className="input-field" value={basketId} onChange={(e) => setBasketId(Number(e.target.value))}>
              {list.map((s) => (
                <option key={s.basket.id} value={s.basket.id}>
                  {s.basket.name} (잔액 {won(s.balance)})
                </option>
              ))}
            </select>
          </Field>
          <Field label="꺼낸 금액">
            <input className="input-field num" inputMode="numeric" value={amount} onChange={(e) => setAmount(formatAmount(e.target.value))} />
          </Field>
          {cur?.basket.kind === "scheduled" && (
            <Field label="어느 마감 몫인가요" className="sm:col-span-2">
              <select className="input-field" value={due} onChange={(e) => setDue(e.target.value)}>
                <option value="">항목 없이</option>
                {dueOptions(cur).map((d) => (
                  <option key={dueKey(d)} value={dueKey(d)}>
                    {d.ym.slice(0, 4)}년 {monthOf(d.ym)} {d.name} {won(d.amount)}
                  </option>
                ))}
              </select>
            </Field>
          )}
          <label className="flex items-start gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" className="mt-0.5" checked={moved} onChange={(e) => setMoved(e.target.checked)} />
            <span>
              생활비 통장(또는 카드)으로 내고, 바구니 돈을 생활비 통장으로 옮겨 옴
              <span className="block text-xs text-zinc-500">바구니 통장에서 바로 냈으면 끄세요. 켜 두면 옮겨 온 돈이 자금 계획 잔액에 더해집니다</span>
            </span>
          </label>
          <Field label="메모" className="sm:col-span-2">
            <input className="input-field" value={memo} onChange={(e) => setMemo(e.target.value)} />
          </Field>
        </div>
      )}
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy || list.length === 0}>연결</button>
      </div>
    </Modal>
  );
}

function EntryModal({ st, mode: initMode, onClose, onSaved }: { st: store.BasketStatus; mode: EntryMode; onClose: () => void; onSaved: () => void }) {
  const [mode, setMode] = useState<EntryMode>(initMode);
  const [amount, setAmount] = useState("");
  const [sign, setSign] = useState<"+" | "-">("+");
  const [date, setDate] = useState(today());
  const [due, setDue] = useState("");
  const [moved, setMoved] = useState(true);
  const [memo, setMemo] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    const n = parseAmount(amount) || 0;
    if (n <= 0) return setErr("금액을 입력하세요.");
    const d = dueOptions(st).find((x) => dueKey(x) === due);
    setBusy(true);
    try {
      await AddBasketEntry(
        new store.BasketEntry({
          basketId: st.basket.id,
          kind: mode,
          amount: mode === "adjust" && sign === "-" ? -n : n,
          date,
          itemId: mode === "spend" ? d?.itemId : undefined,
          dueYm: mode === "spend" ? d?.ym ?? "" : "",
          moved: mode === "spend" && moved,
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
    <Modal title={st.basket.name} icon={PiggyBank} onClose={onClose} onSubmit={submit}>
      <div className="mb-3">
        <Segmented
          value={mode}
          options={[
            { v: "extra", label: "더 넣기" },
            { v: "spend", label: "꺼내 쓰기" },
            { v: "adjust", label: "잔액 맞춤" },
          ]}
          onChange={setMode}
        />
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label={mode === "extra" ? "넣은 금액" : mode === "spend" ? "꺼낸 금액" : "맞출 금액"}>
          <div className="flex gap-2">
            {mode === "adjust" && (
              <select className="input-field !w-auto" value={sign} onChange={(e) => setSign(e.target.value as "+" | "-")} aria-label="늘림/줄임">
                <option value="+">늘림</option>
                <option value="-">줄임</option>
              </select>
            )}
            <input className="input-field num min-w-0 flex-1" inputMode="numeric" placeholder="0" autoFocus value={amount} onChange={(e) => setAmount(formatAmount(e.target.value))} />
          </div>
        </Field>
        <Field label="날짜">
          <input type="date" className="input-field num" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
        {mode === "spend" && st.basket.kind === "scheduled" && (
          <Field label="어느 마감 몫인가요" className="sm:col-span-2">
            <select className="input-field" value={due} onChange={(e) => setDue(e.target.value)}>
              <option value="">항목 없이</option>
              {dueOptions(st).map((d) => (
                <option key={dueKey(d)} value={dueKey(d)}>
                  {d.ym.slice(0, 4)}년 {monthOf(d.ym)} {d.name} {won(d.amount)}
                </option>
              ))}
            </select>
          </Field>
        )}
        {mode === "spend" && (
          <label className="flex items-start gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" className="mt-0.5" checked={moved} onChange={(e) => setMoved(e.target.checked)} />
            <span>
              바구니 돈을 생활비 통장으로 옮겨 옴
              <span className="block text-xs text-zinc-500">지출 거래와 함께 기록하려면 거래내역에서 그 거래의 '바구니에서 냄'을 쓰세요</span>
            </span>
          </label>
        )}
        <Field label="메모" className="sm:col-span-2">
          <input className="input-field" placeholder={mode === "extra" ? "예: 상여금" : mode === "adjust" ? "예: 이자" : ""} value={memo} onChange={(e) => setMemo(e.target.value)} />
        </Field>
      </div>
      <p className="mt-2 text-xs text-zinc-500">
        {mode === "extra"
          ? "생활비 통장에서 보낸 돈이면 자금 계획 잔액에서 빠집니다."
          : mode === "adjust"
          ? "이자처럼 바구니 통장 안에서만 늘고 준 돈입니다. 자금 계획에는 영향이 없습니다."
          : ""}
      </p>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>기록</button>
      </div>
    </Modal>
  );
}

function HistoryModal({ st, onClose, onChanged }: { st: store.BasketStatus; onClose: () => void; onChanged: () => void }) {
  const [err, setErr] = useState("");
  const remove = async (id: number) => {
    try {
      await DeleteBasketEntry(id);
      onChanged();
    } catch (e: any) {
      setErr(String(e));
    }
  };
  return (
    <Modal title={`${st.basket.name} 기록`} icon={History} onClose={onClose} wide>
      <p className="mb-2 text-xs text-zinc-500">최근 30건. 자동 채움을 지우면 그달은 건너뜀(0원)으로 남아 다시 채우지 않습니다.</p>
      <div className="divide-y divide-white/[0.06]">
        {(st.entries ?? []).map((e) => (
          <div key={e.id} className="flex items-center gap-3 py-2 text-sm">
            <span className="num w-12 shrink-0 text-zinc-400">{shortDate(e.date)}</span>
            <span className="w-20 shrink-0 text-zinc-300">
              {ENTRY_LABEL[e.kind] ?? e.kind}
              {e.auto && <span className="ml-1 text-xs text-zinc-500">자동</span>}
            </span>
            <span className="min-w-0 flex-1 truncate text-zinc-400">
              {[e.itemName && `${e.itemName}${e.dueYm ? ` ${monthOf(e.dueYm)}` : ""}`, e.txMerchant, e.moved && "생활비 통장으로 옮김", e.memo]
                .filter(Boolean)
                .join(" · ")}
            </span>
            <span className={`num shrink-0 ${e.amount < 0 ? "text-rose-300" : e.amount > 0 ? "text-emerald-300" : "text-zinc-500"}`}>{signedWon(e.amount)}</span>
            {e.kind !== "open" && !(e.auto && e.amount === 0) ? (
              <button type="button" className="btn-icon shrink-0 hover:!text-rose-300" aria-label="지우기" title="지우기" onClick={() => remove(e.id)}>
                <Trash2 className="h-4 w-4" />
              </button>
            ) : (
              <span className="w-9 shrink-0" />
            )}
          </div>
        ))}
      </div>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
    </Modal>
  );
}

type ModalState =
  | { t: "form"; basket: store.Basket | null; draft: Draft | null }
  | { t: "link"; cand: store.BasketCandidate }
  | { t: "entry"; id: number; mode: EntryMode }
  | { t: "history"; id: number }
  | null;

export default function BasketsPage({ refs, reloadRefs }: { refs: Refs; reloadRefs: () => Promise<void> }) {
  const [list, setList] = useState<store.BasketStatus[]>([]);
  const [cands, setCands] = useState<store.BasketCandidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState<ModalState>(null);
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(async () => {
    try {
      const [l, c] = await Promise.all([ListBaskets(), ListBasketCandidates()]);
      setList(l ?? []);
      setCands(c ?? []);
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
  const cur = modal && (modal.t === "entry" || modal.t === "history") ? list.find((s) => s.basket.id === modal.id) : undefined;

  const dismiss = async (c: store.BasketCandidate) => {
    try {
      await DismissBasketCandidate(c.transactionId);
      setCands((cs) => cs.filter((x) => x.transactionId !== c.transactionId));
    } catch (e: any) {
      setErr(String(e));
    }
  };
  const total = useMemo(() => list.filter((s) => s.basket.active).reduce((a, s) => a + s.balance, 0), [list]);
  const monthlyTotal = useMemo(() => list.filter((s) => s.basket.active).reduce((a, s) => a + s.basket.monthly, 0), [list]);

  const remove = async (st: store.BasketStatus) => {
    if (
      !(await confirm({
        title: `"${st.basket.name}" 바구니를 지울까요?`,
        danger: true,
        confirmLabel: "지우기",
        message: "바구니 기록(채움 · 꺼냄)이 지워집니다. 연결했던 지출 거래는 거래내역에 그대로 남습니다.",
      }))
    )
      return;
    try {
      await DeleteBasket(st.basket.id);
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  return (
    <div className="space-y-6">
      {confirmUI}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={PiggyBank} tone="mint" />
          <div className="min-w-0">
            <h2 className="text-2xl font-light tracking-tight text-white">바구니</h2>
            <p className="label-mono num">
              {list.length ? `모은 돈 ${won(total)} · 매달 ${won(monthlyTotal)}` : "갑자기 나가는 돈과 해마다 정해진 큰돈을 미리 모아 둡니다"}
            </p>
          </div>
        </div>
        <button className="btn-primary" onClick={() => setModal({ t: "form", basket: null, draft: null })}>
          <Plus className="h-4 w-4" />
          바구니 만들기
        </button>
      </div>
      {err && <p className="text-sm text-rose-300">{err}</p>}

      {!loading && !err && list.length === 0 && (
        <div className="surface p-5">
          <p className="mb-4 text-sm leading-relaxed text-zinc-400">
            바구니마다 통장을 하나 두고 월급날 조금씩 채워 두면, 축의금 · 자동차 수리 · 재산세 같은 큰돈이 나가도 생활비가 흔들리지 않습니다.
            아래에서 골라 시작하세요.
          </p>
          <div className="grid gap-3 sm:grid-cols-3">
            {TEMPLATES.map((t) => (
              <button
                key={t.label}
                type="button"
                className="surface-inset rounded-2xl p-4 text-left transition-colors hover:bg-white/[0.04]"
                onClick={() => setModal({ t: "form", basket: null, draft: t.draft() })}
              >
                <strong className="block text-sm font-semibold text-white">{t.label}</strong>
                <span className="text-xs text-zinc-400">{t.desc}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {list.map((st) => {
          const b = st.basket;
          const off = !b.active;
          const pct = b.cap > 0 ? Math.min(100, Math.round((st.balance / b.cap) * 100)) : 0;
          return (
            <div key={b.id} className={`surface flex flex-col p-5 ${off ? "opacity-60" : ""}`}>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <strong className="block truncate text-sm font-semibold text-white">{b.name}</strong>
                  <span className="label-mono num block truncate">
                    {[KIND_LABEL[b.kind], b.paymentMethodName, off ? "꺼 둠" : ""].filter(Boolean).join(" · ")}
                  </span>
                </div>
                <div className="flex shrink-0 gap-1">
                  <button className="btn-icon" title="고치기" aria-label="고치기" onClick={() => setModal({ t: "form", basket: b, draft: null })}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button className="btn-icon hover:!text-rose-300" title="지우기" aria-label="지우기" onClick={() => remove(st)}>
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="mt-4">
                <div className="label-mono">잔액</div>
                <div className="num text-3xl font-light tracking-tight text-white">{won(st.balance)}</div>
                {b.monthly > 0 && (
                  <p className="num mt-1 text-xs text-zinc-400">
                    매달 {b.fillDay}일 {won(b.monthly)} · {b.autoFill ? "자동이체" : "직접 넣기"}
                    {!off && st.nextFillDate && <span className="text-zinc-500"> · 다음 {shortDate(st.nextFillDate)}</span>}
                  </p>
                )}
              </div>

              {b.kind === "reserve" && b.cap > 0 && (
                <div className="mt-4">
                  <div className="h-2 overflow-hidden rounded-full bg-white/[0.06]">
                    <div className={`h-full rounded-full ${st.capReached ? "bg-emerald-400" : "bg-mint-400"}`} style={{ width: `${pct}%` }} />
                  </div>
                  <p className="num mt-1.5 text-xs text-zinc-400">
                    목표 {won(b.cap)} · {pct}%
                    {st.capReached ? " · 다 찼습니다" : st.monthsToCap > 0 ? ` · ${st.monthsToCap}개월 뒤 다 참` : ""}
                  </p>
                </div>
              )}

              {b.kind === "scheduled" && (
                <div className="mt-4">
                  {(st.dues ?? []).length === 0 ? (
                    <p className="text-xs text-zinc-500">항목을 넣으면 마감마다 얼마가 필요한지 보여 드립니다.</p>
                  ) : (
                    <table className="w-full text-xs">
                      <tbody>
                        {(st.dues ?? []).map((d) => (
                          <tr key={dueKey(d)} className="border-t border-white/[0.05]">
                            <td className="num py-1.5 text-zinc-400">{monthOf(d.ym)}</td>
                            <td className="py-1.5 text-zinc-200">{d.name}</td>
                            <td className="num py-1.5 text-right text-zinc-300">{won(d.amount)}</td>
                            <td className="num py-1.5 pl-2 text-right">
                              {d.paid ? (
                                <span className="text-emerald-300">냄</span>
                              ) : d.short > 0 ? (
                                <span className="text-amber-300">{won(d.short)} 모자람</span>
                              ) : (
                                <span className="text-zinc-500">준비됨</span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                  {st.recommended > b.monthly && (
                    <p className="num mt-2 text-xs text-amber-300">마감을 모두 맞추려면 매달 {won(st.recommended)} (지금 {won(b.monthly)})</p>
                  )}
                </div>
              )}

              <div className="mt-auto flex flex-wrap gap-2 pt-4">
                <button className="btn-primary" onClick={() => setModal({ t: "entry", id: b.id, mode: "extra" })} disabled={off}>
                  <Plus className="h-4 w-4" />
                  더 넣기
                </button>
                <button className="btn-ghost" onClick={() => setModal({ t: "entry", id: b.id, mode: "spend" })} disabled={off}>
                  <ArrowDownLeft className="h-4 w-4" />
                  꺼내 쓰기
                </button>
                <button className="btn-ghost" onClick={() => setModal({ t: "history", id: b.id })}>
                  <History className="h-4 w-4" />
                  기록
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {cands.length > 0 && (
        <div className="surface p-5">
          <div className="mb-3 flex items-center gap-3">
            <IconTile icon={ArrowDownLeft} tone="amber" size="sm" />
            <div className="min-w-0">
              <h3 className="section-title">연결할 지출</h3>
              <p className="label-mono">바구니에 이은 카테고리 · 항목 이름으로 찾은 최근 90일의 큰 지출</p>
            </div>
          </div>
          <div className="divide-y divide-white/[0.06]">
            {cands.map((c) => (
              <div key={c.transactionId} className="flex flex-wrap items-center gap-x-3 gap-y-1.5 py-2.5 text-sm">
                <span className="num w-12 shrink-0 text-zinc-400">{shortDate(c.date)}</span>
                <span className="min-w-0 flex-1">
                  <span className="text-white">{c.merchant}</span>
                  <span className="ml-2 text-xs text-zinc-500">
                    {c.reason} → {c.basketName}
                    {c.itemName ? ` · ${c.itemName}` : ""}
                  </span>
                </span>
                <span className="num shrink-0 text-white">{won(c.amount)}</span>
                <span className="flex shrink-0 gap-1.5">
                  <button type="button" className="btn-primary !py-1 text-xs" onClick={() => setModal({ t: "link", cand: c })}>
                    바구니에서 냄
                  </button>
                  <button type="button" className="btn-ghost !py-1 text-xs" onClick={() => dismiss(c)} title="바구니에서 낸 돈이 아님 — 다시 보이지 않습니다">
                    아님
                  </button>
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {list.length > 0 && (
        <p className="text-xs leading-relaxed text-zinc-500">
          축의금 · 재산세처럼 바구니 돈으로 낸 지출은 거래내역에서 그 거래의 <PiggyBank className="inline h-3.5 w-3.5" /> 바구니에서 냄으로 연결하세요.
          바구니 잔액이 줄고, 생활비 통장으로 옮겨 온 돈은 자금 계획 잔액에 더해집니다.
        </p>
      )}

      {modal?.t === "form" && (
        <BasketForm basket={modal.basket} draft={modal.draft} refs={refs} reloadRefs={reloadRefs} onClose={close} onSaved={load} />
      )}
      {modal?.t === "entry" && cur && <EntryModal st={cur} mode={modal.mode} onClose={close} onSaved={load} />}
      {modal?.t === "history" && cur && <HistoryModal st={cur} onClose={close} onChanged={load} />}
      {modal?.t === "link" && (
        <BasketSpendModal
          tx={
            new store.Transaction({
              id: modal.cand.transactionId,
              date: modal.cand.date,
              amount: modal.cand.amount,
              merchant: modal.cand.merchant,
              memo: modal.cand.memo,
              paymentMethodId: modal.cand.paymentMethodId,
              direction: "expense",
            })
          }
          prefer={{ basketId: modal.cand.basketId, itemId: modal.cand.itemId || undefined }}
          onClose={close}
          onSaved={() => {
            close();
            load();
          }}
        />
      )}
    </div>
  );
}
