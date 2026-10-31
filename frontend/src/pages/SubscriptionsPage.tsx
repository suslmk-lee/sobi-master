import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  DeleteSubscription,
  GetSubscriptionMonth,
  PostDueSubscriptions,
  SaveSubscription,
} from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import { categoryOptions, formatAmount, parseAmount, Refs, won } from "../lib";
import MonthPicker from "../MonthPicker";
import { LineChart } from "../charts";
import { Sk } from "../Skeleton";
import { useConfirm } from "../ConfirmModal";
import { AlertCircle, CalendarClock, CalendarOff, Check, Pencil, Plus, PlusCircle, Power, RefreshCw, Repeat, Trash2, TrendingUp } from "lucide-react";
import IconTile from "../components/ui/IconTile";
import { useThemeColors } from "../appearance/AppearanceContext";

const BLANK = {
  id: 0,
  name: "",
  merchant: "",
  amount: "",
  cycle: "monthly",
  billingDay: "1",
  billingMonth: "1",
  startYm: "",
  endYm: "",
  nextAmount: "",
  nextAmountYm: "",
  categoryId: "",
  memberId: "",
  paymentMethodId: "",
  memo: "",
  active: true,
  autoPost: false,
};
type Form = typeof BLANK;

const monthLabel = (ym: string) => `${Number(ym.slice(5))}월`;

// "2026-10-26" → "10/26(월)"
const WD = ["일", "월", "화", "수", "목", "금", "토"];
const dayLabel = (d: string) => {
  const t = new Date(d + "T00:00:00");
  return `${t.getMonth() + 1}/${t.getDate()}(${WD[t.getDay()]})`;
};

// "YYYY-MM" 입력칸. 비울 수 있어야 해서 MonthPicker 대신 체크박스 + 선택기 조합을 쓴다.
function OptionalMonth({
  label,
  value,
  onChange,
  hint,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  hint: string;
}) {
  const on = value !== "";
  const thisYm = new Date().toISOString().slice(0, 7);
  return (
    <div className="surface-inset flex flex-wrap items-center gap-3 px-3.5 py-2.5">
      <label className="flex items-center gap-2 text-sm text-zinc-300">
        <input
          type="checkbox"
          className="accent-mint-500"
          checked={on}
          onChange={(e) => onChange(e.target.checked ? thisYm : "")}
        />
        {label}
      </label>
      {on ? (
        <MonthPicker value={value} onChange={onChange} />
      ) : (
        <span className="text-xs text-zinc-500">{hint}</span>
      )}
    </div>
  );
}

export default function SubscriptionsPage({
  refs,
  month,
  setMonth,
}: {
  refs: Refs;
  month: string;
  setMonth: (m: string) => void;
}) {
  const [data, setData] = useState<main.SubMonth | null>(null);
  const [f, setF] = useState<Form>({ ...BLANK });
  const [editing, setEditing] = useState(false);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(true);
  const [postMsg, setPostMsg] = useState("");
  const [confirm, confirmUI] = useConfirm();
  const theme = useThemeColors();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [y, m] = month.split("-").map(Number);
      setData(await GetSubscriptionMonth(y, m));
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setLoading(false);
    }
  }, [month]);

  useEffect(() => {
    load();
  }, [load]);

  const reset = () => {
    setF({ ...BLANK });
    setEditing(false);
  };

  const edit = (s: store.Subscription) => {
    setF({
      id: s.id,
      name: s.name,
      merchant: s.merchant,
      amount: formatAmount(String(s.amount)),
      cycle: s.cycle,
      billingDay: String(s.billingDay),
      billingMonth: String(s.billingMonth || 1),
      startYm: s.startYm,
      endYm: s.endYm,
      nextAmount: s.nextAmount ? formatAmount(String(s.nextAmount)) : "",
      nextAmountYm: s.nextAmountYm,
      categoryId: s.categoryId ? String(s.categoryId) : "",
      memberId: s.memberId ? String(s.memberId) : "",
      paymentMethodId: s.paymentMethodId ? String(s.paymentMethodId) : "",
      memo: s.memo,
      active: s.active,
      autoPost: s.autoPost,
    });
    setEditing(true);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!f.name.trim()) return setErr("이름을 입력하세요.");
    try {
      await SaveSubscription(
        new store.Subscription({
          id: f.id,
          name: f.name.trim(),
          merchant: f.merchant.trim(),
          amount: parseAmount(f.amount),
          cycle: f.cycle,
          billingDay: Number(f.billingDay) || 1,
          billingMonth: f.cycle === "yearly" ? Number(f.billingMonth) || 1 : 0,
          startYm: f.startYm,
          endYm: f.endYm,
          nextAmount: parseAmount(f.nextAmount),
          nextAmountYm: f.nextAmountYm,
          categoryId: f.categoryId ? Number(f.categoryId) : undefined,
          memberId: f.memberId ? Number(f.memberId) : undefined,
          paymentMethodId: f.paymentMethodId ? Number(f.paymentMethodId) : undefined,
          memo: f.memo,
          active: f.active,
          autoPost: f.autoPost,
        })
      );
      reset();
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const remove = async (s: store.Subscription) => {
    const ok = await confirm({
      title: `"${s.name}" 정기결제를 삭제할까요?`,
      danger: true,
      confirmLabel: "삭제",
      message: s.autoPost
        ? "이미 기록된 거래는 그대로 남고, 다음 달부터 자동 등록이 멈춥니다."
        : "이미 기록된 거래는 그대로 남습니다.",
    });
    if (!ok) return;
    try {
      await DeleteSubscription(s.id);
      if (f.id === s.id) reset();
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  // 결제일이 지났는데 아직 안 들어간 자동 등록분을 지금 만든다.
  // (앱이 켜질 때와 한 시간마다 저절로 돌지만, 바로 확인하고 싶을 때 누른다)
  const postNow = async () => {
    setErr("");
    setPostMsg("");
    try {
      const r = await PostDueSubscriptions();
      const parts: string[] = [];
      if (r.created) parts.push(`새로 ${r.created}건 등록`);
      if (r.matched) parts.push(`이미 입력된 ${r.matched}건에 연결`);
      setPostMsg(
        (parts.length ? parts.join(", ") : "새로 등록할 정기결제가 없습니다") +
          (r.warnings?.length ? ` · ${r.warnings.join(" ")}` : "")
      );
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const toggleActive = async (s: store.Subscription) => {
    try {
      await SaveSubscription(new store.Subscription({ ...s, active: !s.active }));
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const cats = categoryOptions(refs.categories, "expense");
  const items = data?.items ?? [];
  const due = items.filter((i) => i.due);
  const off = items.filter((i) => !i.due);
  const ending = items.filter((i) => i.ending);
  const changing = items.filter((i) => i.changingYm);

  const statCls = "surface p-4 flex flex-col gap-1";

  return (
    <div className="space-y-5">
      {confirmUI}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={Repeat} tone="mint" />
          <div className="min-w-0">
            <h2 className="text-2xl font-light tracking-tight text-white">정기결제</h2>
            <p className="label-mono num">{items.length} items</p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <MonthPicker value={month} onChange={setMonth} />
          <button className="btn-ghost" onClick={postNow} title="결제일이 지난 자동 등록분을 지금 반영">
            <RefreshCw className="h-4 w-4" />
            자동 등록 지금 반영
          </button>
        </div>
      </div>

      <p className="text-xs leading-relaxed text-zinc-500">
        등록한 정기결제를 실제 거래와 대조합니다. "자동 등록"을 켠 항목은 결제일(주말·공휴일이면
        다음 업무일)에 거래가 저절로 생깁니다.
      </p>

      {err && (
        <div role="alert" className="flex items-start gap-3 rounded-xl border border-rose-400/25 bg-rose-500/10 p-3.5 text-sm text-rose-200">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <p className="min-w-0 flex-1">{err}</p>
        </div>
      )}
      {postMsg && (
        <div role="status" className="surface flex items-start gap-3 p-3.5">
          <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-mint-400" />
          <p className="min-w-0 flex-1 text-sm leading-relaxed text-zinc-300">{postMsg}</p>
        </div>
      )}

      {loading ? (
        <Sk h={92} r={16} />
      ) : (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <div className={statCls}>
            <span className="label-mono">이번 달 예정</span>
            <strong className="num text-xl font-semibold text-rose-300">{won(data?.planned ?? 0)}</strong>
            <span className="label-mono num">{data?.dueCount ?? 0}건</span>
          </div>
          <div className={statCls}>
            <span className="label-mono">실제 빠진 금액</span>
            <strong className="num text-xl font-semibold text-white">{won(data?.seenTotal ?? 0)}</strong>
            <span className="label-mono num">
              {data?.seenCount ?? 0} / {data?.dueCount ?? 0}건 확인
            </span>
          </div>
          <div className={statCls}>
            <span className="label-mono">월 환산 (연결제 ÷12 포함)</span>
            <strong className="num text-xl font-semibold text-white">{won(data?.monthlyEquivalent ?? 0)}</strong>
          </div>
          <div className={statCls}>
            <span className="label-mono">연간 예상</span>
            <strong className="num text-xl font-semibold text-white">{won(data?.yearlyProjection ?? 0)}</strong>
          </div>
        </div>
      )}

      {(ending.length > 0 || changing.length > 0) && (
        <div className="space-y-2">
          {ending.map((i) => (
            <div
              className="flex flex-col gap-0.5 rounded-xl border border-amber-400/25 bg-amber-500/10 p-3.5"
              key={`e${i.sub.id}`}
            >
              <strong className="text-sm text-amber-200">{i.sub.name} — 이번 달이 마지막</strong>
              <span className="text-xs text-zinc-400">
                {i.sub.endYm} 이후로는 결제되지 않습니다.
              </span>
            </div>
          ))}
          {changing.map((i) => (
            <div
              className="flex flex-col gap-0.5 rounded-xl border border-sky-400/25 bg-sky-500/10 p-3.5"
              key={`c${i.sub.id}`}
            >
              <strong className="num text-sm text-sky-200">
                {i.sub.name} — {i.changingYm}부터 {won(i.changingAmount)}
              </strong>
              <span className="num text-xs text-zinc-400">
                현재 {won(i.amount)} → {won(i.changingAmount)} (
                {i.changingAmount > i.amount ? "인상" : "인하"})
              </span>
            </div>
          ))}
        </div>
      )}

      <div className="surface p-5">
        <div className="mb-4 flex items-center gap-3">
          <IconTile icon={editing ? Pencil : PlusCircle} tone="mint" size="sm" />
          <h3 className="section-title">{editing ? "정기결제 수정" : "정기결제 등록"}</h3>
        </div>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <input
              type="text"
              className="input-field"
              placeholder="이름 (예: 구글 제미나이)"
              value={f.name}
              onChange={(e) => setF({ ...f, name: e.target.value })}
            />
            <input
              type="text"
              className="input-field"
              placeholder="대조할 가맹점명 (비우면 이름으로 대조)"
              value={f.merchant}
              onChange={(e) => setF({ ...f, merchant: e.target.value })}
            />
            <input
              type="text"
              inputMode="numeric"
              className="input-field num"
              placeholder="요금"
              value={f.amount}
              onChange={(e) => setF({ ...f, amount: formatAmount(e.target.value) })}
            />
            <div className="flex gap-3">
              <select
                className="input-field"
                value={f.cycle}
                onChange={(e) => setF({ ...f, cycle: e.target.value })}
              >
                <option value="monthly">매월</option>
                <option value="yearly">매년</option>
              </select>
              {f.cycle === "yearly" && (
                <select
                  className="input-field"
                  value={f.billingMonth}
                  onChange={(e) => setF({ ...f, billingMonth: e.target.value })}
                  title="결제 월"
                >
                  {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
                    <option key={m} value={m}>{m}월</option>
                  ))}
                </select>
              )}
              <select
                className="input-field"
                value={f.billingDay}
                onChange={(e) => setF({ ...f, billingDay: e.target.value })}
                title="결제일"
              >
                {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                  <option key={d} value={d}>{d}일</option>
                ))}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <select
              className="input-field"
              value={f.categoryId}
              onChange={(e) => setF({ ...f, categoryId: e.target.value })}
            >
              <option value="">카테고리 선택</option>
              {cats.map((c) => (
                <option key={c.id} value={c.id}>{c.label}</option>
              ))}
            </select>
            <select
              className="input-field"
              value={f.memberId}
              onChange={(e) => setF({ ...f, memberId: e.target.value })}
            >
              <option value="">귀속자 선택</option>
              {refs.members.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </select>
            <select
              className="input-field"
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
              className="input-field"
              placeholder="메모"
              value={f.memo}
              onChange={(e) => setF({ ...f, memo: e.target.value })}
            />
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <OptionalMonth
              label="시작 월"
              value={f.startYm}
              onChange={(v) => setF({ ...f, startYm: v })}
              hint="제한 없음"
            />
            <OptionalMonth
              label="종료 월 (이 달까지 결제)"
              value={f.endYm}
              onChange={(v) => setF({ ...f, endYm: v })}
              hint="무기한"
            />
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <OptionalMonth
              label="요금 변경 예정"
              value={f.nextAmountYm}
              onChange={(v) => setF({ ...f, nextAmountYm: v })}
              hint="변경 예정 없음"
            />
            {f.nextAmountYm && (
              <input
                type="text"
                inputMode="numeric"
                className="input-field num"
                placeholder="변경 후 요금"
                value={f.nextAmount}
                onChange={(e) => setF({ ...f, nextAmount: formatAmount(e.target.value) })}
              />
            )}
          </div>

          <label className="surface-inset flex items-start gap-3 p-3.5">
            <input
              type="checkbox"
              className="mt-1 accent-mint-500"
              checked={f.autoPost}
              onChange={(e) => setF({ ...f, autoPost: e.target.checked })}
            />
            <span className="text-sm text-zinc-200">
              <strong className="font-medium text-white">매달 거래 자동 등록</strong>
              <span className="text-xs leading-relaxed text-zinc-500">
                {" "}— 결제일에 거래를 만들어 줍니다. 주말·공휴일이면 다음 업무일로 밀고, 그 달 결제를
                이미 입력해 뒀으면 새로 만들지 않고 연결만 합니다. 켠 달부터 적용됩니다(지난 달은 소급하지 않음).
              </span>
            </span>
          </label>

          <div className="flex flex-wrap items-center gap-3">
            <button type="submit" className="btn-primary">
              {editing ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
              {editing ? "수정 저장" : "등록"}
            </button>
            {editing && (
              <button type="button" className="btn-ghost" onClick={reset}>취소</button>
            )}
          </div>
        </form>
      </div>

      <div className="surface p-5">
        <div className="mb-4 flex items-center gap-3">
          <IconTile icon={CalendarClock} tone="mint" size="sm" />
          <h3 className="section-title">이번 달 결제 예정 ({monthLabel(month)})</h3>
          <span className="chip num">{due.length}</span>
        </div>
        {loading ? (
          <Sk h={120} r={12} />
        ) : due.length === 0 ? (
          <p className="text-sm text-zinc-500">이번 달에 빠질 정기결제가 없습니다.</p>
        ) : (
          <ul className="space-y-2">
            {due.map((i) => (
              <li
                key={i.sub.id}
                className={`surface-inset flex flex-wrap items-center gap-x-4 gap-y-2 p-3.5 ${
                  i.seen ? "" : "border-l-2 border-l-sky-400/60"
                }`}
              >
                <div className="min-w-[10rem] flex-1">
                  <div className="flex flex-wrap items-center gap-1.5 text-sm font-medium text-white">
                    {i.sub.name}
                    {i.sub.cycle === "yearly" && (
                      <span className="badge bg-white/[0.06] text-zinc-300">연 결제</span>
                    )}
                    {i.sub.autoPost && (
                      <span className="badge bg-mint-500/10 text-mint-300">
                        <Repeat className="h-3 w-3" />
                        자동 등록
                      </span>
                    )}
                  </div>
                  <div className="num mt-1 flex flex-wrap items-center gap-1.5 text-xs text-zinc-500">
                    {/* 주말·공휴일로 밀리면 원래 날짜와 실제 결제일을 같이 보여 준다 */}
                    {i.chargeDate && i.chargeDate !== i.plannedDate ? (
                      <>
                        <span>{dayLabel(i.plannedDate)} →</span>
                        <strong className="font-medium text-zinc-200">{dayLabel(i.chargeDate)}</strong>
                        <span className="chip">{i.shiftReason}</span>
                      </>
                    ) : i.chargeDate ? (
                      <span>{dayLabel(i.chargeDate)}</span>
                    ) : (
                      <span>{`매월 ${i.sub.billingDay}일`}</span>
                    )}
                    {i.sub.categoryName && <span>{`· ${i.sub.categoryName}`}</span>}
                    {i.sub.paymentMethodName && <span>{`· ${i.sub.paymentMethodName}`}</span>}
                    {i.sub.endYm && <span>{`· ${i.sub.endYm}까지`}</span>}
                  </div>
                </div>
                <span className="num text-sm font-medium text-rose-300">{won(i.amount)}</span>
                <div className="flex flex-wrap items-center gap-1.5">
                  {i.seen ? (
                    <span className="badge bg-emerald-500/10 text-emerald-300">
                      <span className="num">
                        빠짐 · {won(i.seenAmount)}
                        {i.seenDate ? ` (${i.seenDate.slice(5)})` : ""}
                      </span>
                      {i.diff !== 0 && (
                        <em className={`num not-italic ${i.diff > 0 ? "text-rose-300" : "text-emerald-300"}`}>
                          {i.diff > 0 ? " +" : " "}
                          {won(i.diff)}
                        </em>
                      )}
                    </span>
                  ) : i.postState === "deleted" ? (
                    // 자동으로 만들었는데 사용자가 지운 달 — 다시 만들지 않는다는 걸 알려 준다
                    <span
                      className="badge bg-amber-500/10 text-amber-300"
                      title="자동으로 만든 거래를 지웠습니다. 이 달은 다시 만들지 않습니다."
                    >
                      지움 · 다시 안 만듦
                    </span>
                  ) : (
                    <span className="badge bg-sky-500/10 text-sky-300">
                      {i.sub.autoPost ? "자동 등록 예정" : "예정"}
                    </span>
                  )}
                  {i.postState === "matched" && (
                    <span
                      className="badge bg-sky-500/10 text-sky-300"
                      title="이미 입력된 거래를 이 달 결제로 연결했습니다"
                    >
                      기존 거래 연결
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1">
                  <button className="btn-icon" title="수정" aria-label="수정" onClick={() => edit(i.sub)}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button className="btn-icon" title="끄기" aria-label="끄기" onClick={() => toggleActive(i.sub)}>
                    <Power className="h-4 w-4" />
                  </button>
                  <button
                    className="btn-icon hover:!border-rose-400/30 hover:!text-rose-300"
                    title="삭제"
                    aria-label="삭제"
                    onClick={() => remove(i.sub)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      {off.length > 0 && (
        <div className="surface p-5">
          <div className="mb-4 flex flex-wrap items-center gap-3">
            <IconTile icon={CalendarOff} tone="amber" size="sm" />
            <h3 className="section-title">이번 달 해당 없음</h3>
            <span className="label-mono">종료·시작 전·연결제 비결제월·꺼둠</span>
          </div>
          <ul className="space-y-2">
            {off.map((i) => (
              <li
                key={i.sub.id}
                className="surface-inset flex flex-wrap items-center gap-x-4 gap-y-2 p-3.5 opacity-70"
              >
                <span className="min-w-[8rem] text-sm text-zinc-300">{i.sub.name}</span>
                <span className="min-w-[8rem] flex-1 text-xs text-zinc-500">
                  {!i.sub.active
                    ? "꺼둠"
                    : i.sub.endYm && month > i.sub.endYm
                    ? `${i.sub.endYm} 종료`
                    : i.sub.startYm && month < i.sub.startYm
                    ? `${i.sub.startYm} 시작 예정`
                    : i.sub.cycle === "yearly"
                    ? `매년 ${i.sub.billingMonth}월 결제`
                    : ""}
                </span>
                <span className="num text-sm text-zinc-500">{won(i.amount)}</span>
                <div className="flex items-center gap-1">
                  <button className="btn-icon" title="수정" aria-label="수정" onClick={() => edit(i.sub)}>
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button
                    className="btn-icon"
                    title={i.sub.active ? "끄기" : "켜기"}
                    aria-label={i.sub.active ? "끄기" : "켜기"}
                    onClick={() => toggleActive(i.sub)}
                  >
                    <Power className="h-4 w-4" />
                  </button>
                  <button
                    className="btn-icon hover:!border-rose-400/30 hover:!text-rose-300"
                    title="삭제"
                    aria-label="삭제"
                    onClick={() => remove(i.sub)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="surface p-5">
        <div className="mb-4 flex items-center gap-3">
          <IconTile icon={TrendingUp} tone="sky" size="sm" />
          <h3 className="section-title">정기결제 지출 추이 (최근 {data?.trend?.length ?? 6}개월)</h3>
        </div>
        {loading ? (
          <Sk h={180} r={12} />
        ) : !data?.trend?.length ? (
          <p className="text-sm text-zinc-500">추이를 만들 데이터가 없습니다.</p>
        ) : (
          <LineChart
            xLabels={(data.trendMonths ?? []).map(monthLabel)}
            series={[{ name: "정기결제", color: theme.accent, values: data.trend }]}
          />
        )}
      </div>
    </div>
  );
}
