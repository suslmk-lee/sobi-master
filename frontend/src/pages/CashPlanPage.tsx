import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  AddCashBalance,
  DeleteCashBalance,
  GetCashPlan,
  ListCashBalances,
  ListTransactions,
  SaveCashSettings,
} from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import {
  AlertTriangle,
  Banknote,
  CalendarClock,
  ChevronRight,
  CreditCard,
  Flag,
  Landmark,
  PiggyBank,
  Repeat,
  Settings2,
  Trash2,
  TrendingDown,
  Wallet,
  type LucideIcon,
} from "lucide-react";
import { Refs, formatAmount, parseAmount, today as todayStr, won } from "../lib";
import IconTile from "../components/ui/IconTile";
import CountUp from "../components/CountUp";
import { ChartEmpty, ChartTip, LegendDot, compact, cv, useAxisTick } from "../charts";
import { useThemeColors } from "../appearance/AppearanceContext";
import { autoColor } from "../PmChip";
import { Sk } from "../Skeleton";
import { useConfirm } from "../ConfirmModal";
import TxDrillModal from "../TxDrillModal";
import EditModal from "../EditModal";

const DOW = ["일", "월", "화", "수", "목", "금", "토"];
const parse = (d: string) => new Date(d + "T00:00:00");
// "2026-11-01" → "11월 1일 (일)"
const dateLabel = (d: string) => {
  const x = parse(d);
  return `${x.getMonth() + 1}월 ${x.getDate()}일 (${DOW[x.getDay()]})`;
};
const shortDate = (d: string) => `${Number(d.slice(5, 7))}/${Number(d.slice(8, 10))}`;
const dday = (d: string, base: string) =>
  Math.round((parse(d).getTime() - parse(base).getTime()) / 86400000);
const ddayText = (n: number) => (n === 0 ? "D-day" : n > 0 ? `D-${n}` : `D+${-n}`);
const signed = (n: number) => (n >= 0 ? "+" : "−") + won(Math.abs(n));

const KIND: Record<string, { icon: LucideIcon; color: string; label: string }> = {
  card: { icon: CreditCard, color: cv("rose-400"), label: "카드 결제" },
  salary: { icon: Banknote, color: cv("emerald-400"), label: "월급" },
  subscription: { icon: Repeat, color: cv("sky-400"), label: "정기결제" },
};
const kindOf = (k: string) => KIND[k] ?? KIND.subscription;

function EstBadge() {
  return <span className="badge bg-amber-500/10 text-amber-300">예상</span>;
}

// 같은 날짜의 카드 출금을 카드사별로 묶는다. 카드사 대금은 카드사 단위로 한 번에 빠져나가기 때문이다.
// 카드가 아닌 항목(월급·정기결제)은 그대로 두고, 묶음은 그 카드사가 처음 나온 자리에 놓는다.
type TimelineItem =
  | { kind: "event"; ev: store.CashEvent }
  | { kind: "issuer"; issuer: string; events: store.CashEvent[] };

function groupByIssuer(events: store.CashEvent[], issuerOf: (ev: store.CashEvent) => string): TimelineItem[] {
  const out: TimelineItem[] = [];
  const byIssuer = new Map<string, Extract<TimelineItem, { kind: "issuer" }>>();
  for (const ev of events) {
    if (ev.kind !== "card") {
      out.push({ kind: "event", ev });
      continue;
    }
    const issuer = issuerOf(ev);
    let g = byIssuer.get(issuer);
    if (!g) {
      g = { kind: "issuer", issuer, events: [] };
      byIssuer.set(issuer, g);
      out.push(g);
    }
    g.events.push(ev);
  }
  return out;
}

// 일정 한 줄. 카드 출금(청구서가 있으면)은 눌러서 그 청구분의 이용 내역을 본다.
// nested 면 카드사 묶음 아래의 카드 한 장 — 들여 쓰고, 잔액은 묶음 줄에만 보여 준다.
function EventRow({
  ev,
  bill,
  onOpen,
  nested,
}: {
  ev: store.CashEvent;
  bill?: store.CardBill;
  onOpen: (b: store.CardBill) => void;
  nested?: boolean;
}) {
  const k = kindOf(ev.kind);
  const Icon = k.icon;
  const Row = bill ? "button" : "div";
  return (
    <Row
      {...(bill
        ? {
            type: "button" as const,
            onClick: () => onOpen(bill),
            title: `${bill.cardName} 이용 내역 보기 (${bill.periodStart} ~ ${bill.periodEnd})`,
          }
        : {})}
      className={`flex w-full items-center gap-3 px-4 text-left ${nested ? "py-1.5 pl-11" : "py-2.5"} ${
        bill ? "group cursor-pointer transition-colors hover:bg-white/[0.04]" : ""
      }`}
    >
      {nested ? (
        <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-white/20" aria-hidden />
      ) : (
        <Icon className="h-4 w-4 shrink-0" style={{ color: k.color }} strokeWidth={1.75} />
      )}
      <span className={`min-w-0 flex-1 truncate ${nested ? "text-xs text-zinc-400" : "text-sm text-zinc-200"}`}>
        {ev.label}
      </span>
      {ev.estimated && !nested && <EstBadge />}
      <span
        className={`num whitespace-nowrap ${nested ? "text-xs text-zinc-300" : "text-sm font-medium"} ${
          nested ? "" : ev.amount >= 0 ? "text-emerald-300" : "text-rose-300"
        }`}
      >
        {signed(ev.amount)}
      </span>
      <span
        className={`num w-28 whitespace-nowrap text-right text-xs sm:w-36 ${
          ev.balance < 0 ? "text-rose-300" : "text-zinc-400"
        }`}
      >
        {nested ? "" : won(ev.balance)}
      </span>
      {bill ? (
        <ChevronRight className="h-4 w-4 shrink-0 text-zinc-600 transition-colors group-hover:text-zinc-300" />
      ) : (
        <span className="w-4 shrink-0" aria-hidden />
      )}
    </Row>
  );
}

function Callout({ tone, children }: { tone: "amber" | "rose"; children: React.ReactNode }) {
  const t =
    tone === "amber"
      ? { bar: "!border-l-amber-300/60", icon: "text-amber-300" }
      : { bar: "!border-l-rose-300/60", icon: "text-rose-300" };
  return (
    <div className={`surface flex items-start gap-3 border-l-2 px-5 py-3.5 ${t.bar}`}>
      <AlertTriangle className={`mt-0.5 h-4 w-4 shrink-0 ${t.icon}`} />
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

function SectionHead({ icon, title, sub }: { icon: LucideIcon; title: string; sub?: string }) {
  return (
    <div className="mb-4 flex items-center gap-3">
      <IconTile icon={icon} tone="mint" size="sm" />
      <div className="min-w-0">
        <h3 className="section-title">{title}</h3>
        {sub && <p className="label-mono !normal-case mt-0.5">{sub}</p>}
      </div>
    </div>
  );
}

// 잔액 기록 입력 폼 (잔액이 없을 때 안내 안에서도, 설정 카드에서도 쓴다)
function BalanceForm({ onSaved, compactLayout }: { onSaved: () => void; compactLayout?: boolean }) {
  const [date, setDate] = useState(todayStr());
  const [amount, setAmount] = useState("");
  const [memo, setMemo] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!date || amount.replace(/[^\d]/g, "") === "") {
      setErr("날짜와 잔액을 입력하세요.");
      return;
    }
    setBusy(true);
    try {
      await AddCashBalance(date, parseAmount(amount), memo.trim());
      setAmount("");
      setMemo("");
      onSaved();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-2">
      <div className={`grid items-end gap-3 ${compactLayout ? "sm:grid-cols-4" : "sm:grid-cols-2"}`}>
        <label className="block">
          <span className="field-label">기준일</span>
          <input type="date" className="input-field num" value={date} onChange={(e) => setDate(e.target.value)} />
        </label>
        <label className="block">
          <span className="field-label">통장 잔액 (원)</span>
          <input
            type="text"
            inputMode="numeric"
            className="input-field num"
            placeholder="예: 1,500,000"
            value={amount}
            onChange={(e) => setAmount(formatAmount(e.target.value))}
          />
        </label>
        <label className="block">
          <span className="field-label">메모</span>
          <input
            type="text"
            className="input-field"
            placeholder="선택"
            value={memo}
            onChange={(e) => setMemo(e.target.value)}
          />
        </label>
        <button type="submit" className="btn-primary" disabled={busy}>
          {busy ? "저장 중…" : "잔액 기록"}
        </button>
      </div>
      {err && <p className="text-xs text-rose-300">{err}</p>}
    </form>
  );
}

function Skeleton() {
  return (
    <div className="space-y-4">
      <Sk w={170} h={36} r={10} />
      <div className="surface p-6">
        <Sk w={200} h={14} style={{ marginBottom: 14 }} />
        <Sk w={260} h={40} r={10} />
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {[0, 1, 2, 3].map((i) => (
          <div className="surface-inset flex flex-col gap-3 p-4" key={i}>
            <Sk w={90} h={12} />
            <Sk w={130} h={22} />
            <Sk w={100} h={12} />
          </div>
        ))}
      </div>
      <div className="surface p-5 sm:p-6">
        <Sk h={240} r={16} />
      </div>
    </div>
  );
}

// 잔액 흐름: 오늘 → horizon, 계단선. 이벤트가 있는 날에 점을 찍는다.
function FlowChart({ plan }: { plan: store.CashPlan }) {
  const tc = useThemeColors();
  const tick = useAxisTick();
  const reserve = plan.settings.reserve;

  const { data, byDate } = useMemo(() => {
    const t = (d: string) => parse(d).getTime();
    const pts: { t: number; date: string; balance: number; kind?: string }[] = [
      { t: t(plan.today), date: plan.today, balance: plan.balanceNow },
    ];
    const map: Record<string, store.CashEvent[]> = {};
    for (const e of plan.events) {
      pts.push({ t: t(e.date), date: e.date, balance: e.balance, kind: e.kind });
      (map[e.date] ||= []).push(e);
    }
    const last = pts[pts.length - 1];
    if (plan.horizon && t(plan.horizon) > last.t) {
      pts.push({ t: t(plan.horizon), date: plan.horizon, balance: last.balance });
    }
    return { data: pts, byDate: map };
  }, [plan]);

  if (!plan.events.length) return <ChartEmpty>예정된 입출금이 없습니다</ChartEmpty>;

  const lineColor = tc.accent;
  const domain: [number, number] = [data[0].t, data[data.length - 1].t];

  return (
    <div>
      <div className="mb-2 flex flex-wrap gap-4 text-[11px]" style={{ color: tc.muted }}>
        <span className="inline-flex items-center gap-1.5"><LegendDot color={lineColor} /> 예상 잔액</span>
        {Object.values(KIND).map((k) => (
          <span key={k.label} className="inline-flex items-center gap-1.5"><LegendDot color={k.color} /> {k.label}</span>
        ))}
        <span className="inline-flex items-center gap-1.5"><LegendDot color={cv("rose-400")} dashed /> 0원</span>
        {reserve > 0 && (
          <span className="inline-flex items-center gap-1.5">
            <LegendDot color={cv("amber-400")} dashed /> 최소 잔액 <span className="num">{won(reserve)}</span>
          </span>
        )}
      </div>
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data} margin={{ top: 8, right: 14, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id="cashplan-area" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={lineColor} stopOpacity={0.28} />
                <stop offset="100%" stopColor={lineColor} stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid stroke={tc.grid} strokeOpacity={0.4} vertical={false} />
            <XAxis
              dataKey="t"
              type="number"
              scale="time"
              domain={domain}
              tick={tick}
              minTickGap={32}
              axisLine={{ stroke: tc.grid }}
              tickLine={false}
              tickFormatter={(v) => {
                const d = new Date(v);
                return `${d.getMonth() + 1}/${d.getDate()}`;
              }}
            />
            <YAxis
              tick={tick}
              width={58}
              axisLine={false}
              tickLine={false}
              tickFormatter={compact}
              domain={[(min: number) => Math.min(min, 0), "auto"]}
            />
            <Tooltip
              cursor={{ stroke: tc.axis, strokeDasharray: "3 3" }}
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                const row = payload[0].payload as { date: string; balance: number };
                const evs = byDate[row.date] ?? [];
                return (
                  <ChartTip
                    head={dateLabel(row.date)}
                    rows={[
                      ...evs.map((e) => ({
                        color: kindOf(e.kind).color,
                        value: signed(e.amount),
                        name: e.label + (e.estimated ? " (예상)" : ""),
                      })),
                      { color: lineColor, value: won(row.balance), name: "잔액" },
                    ]}
                  />
                );
              }}
            />
            <ReferenceLine y={0} stroke={cv("rose-400")} strokeDasharray="4 4" strokeOpacity={0.7} />
            {reserve > 0 && (
              <ReferenceLine y={reserve} stroke={cv("amber-400")} strokeDasharray="4 4" strokeOpacity={0.7} />
            )}
            <Area
              type="stepAfter"
              dataKey="balance"
              stroke="none"
              fill="url(#cashplan-area)"
              isAnimationActive={false}
              activeDot={false}
            />
            <Line
              type="stepAfter"
              dataKey="balance"
              stroke={lineColor}
              strokeWidth={1.75}
              isAnimationActive={false}
              dot={(p: any) => {
                const kind = p.payload?.kind as string | undefined;
                if (!kind) return <g key={p.key ?? p.index} />;
                return (
                  <circle
                    key={p.key ?? p.index}
                    cx={p.cx}
                    cy={p.cy}
                    r={4}
                    fill={kindOf(kind).color}
                    stroke={tc.surface}
                    strokeWidth={1.5}
                  />
                );
              }}
              activeDot={{ r: 5, fill: lineColor, stroke: tc.surface }}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

// 카드별 청구 표
function BillTable({
  bills,
  onPick,
}: {
  bills: store.CardBill[];
  onPick: (b: store.CardBill) => void;
}) {
  if (!bills.length) return <p className="py-4 text-center text-sm text-zinc-500">해당하는 청구가 없습니다.</p>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="table-head border-b border-white/[0.08]">
            <th className="py-2.5 pr-3 text-left font-normal">카드</th>
            <th className="whitespace-nowrap px-3 py-2.5 text-left font-normal">이용기간</th>
            <th className="px-3 py-2.5 text-left font-normal">상태</th>
            <th className="px-3 py-2.5 text-right font-normal">이용액</th>
            <th className="px-3 py-2.5 text-right font-normal">선결제</th>
            <th className="px-3 py-2.5 text-right font-normal">결제액</th>
            <th className="px-3 py-2.5 text-right font-normal">예상</th>
            <th className="py-2.5 pl-3 text-right font-normal">건수</th>
          </tr>
        </thead>
        <tbody>
          {bills.map((b) => (
            <tr
              key={`${b.cardId}-${b.nominalDue}`}
              className="table-row cursor-pointer text-zinc-200"
              onClick={() => onPick(b)}
              title="이용 내역 보기"
            >
              <td className="whitespace-nowrap py-2.5 pr-3">
                <span className="inline-flex items-center gap-2">
                  <span
                    className="h-2 w-2 shrink-0 rounded-full"
                    style={{ background: b.color || autoColor(b.cardName) }}
                  />
                  {b.cardName}
                </span>
              </td>
              <td className="num whitespace-nowrap px-3 py-2.5 text-zinc-400">
                {b.periodStart} ~ {b.periodEnd}
              </td>
              <td className="px-3 py-2.5">
                {b.closed ? (
                  <span className="badge bg-emerald-500/10 text-emerald-300">확정</span>
                ) : (
                  <span className="badge bg-sky-500/10 text-sky-300">누적 중</span>
                )}
              </td>
              <td className="num whitespace-nowrap px-3 py-2.5 text-right">{won(b.total)}</td>
              <td className="num whitespace-nowrap px-3 py-2.5 text-right text-zinc-400">
                {b.prepaid ? `−${won(b.prepaid)}` : "-"}
              </td>
              <td className="num whitespace-nowrap px-3 py-2.5 text-right font-medium text-rose-300">
                {won(b.due)}
              </td>
              <td className="num whitespace-nowrap px-3 py-2.5 text-right text-amber-300">
                {b.closed ? "" : won(b.projected)}
              </td>
              <td className="num py-2.5 pl-3 text-right text-zinc-400">{b.txCount}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function CashPlanPage({ refs }: { refs: Refs }) {
  const [plan, setPlan] = useState<store.CashPlan | null>(null);
  const [balances, setBalances] = useState<store.CashBalance[]>([]);
  const [err, setErr] = useState("");
  const [confirm, confirmUI] = useConfirm();

  // 설정 폼
  const [salaryDay, setSalaryDay] = useState("10");
  const [salaryAmount, setSalaryAmount] = useState("");
  const [reserve, setReserve] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState("");
  const [saveErr, setSaveErr] = useState("");

  // 카드 청구 드릴다운
  const [drill, setDrill] = useState<store.CardBill | null>(null);
  const [drillTx, setDrillTx] = useState<store.Transaction[]>([]);
  const [editTx, setEditTx] = useState<store.Transaction | null>(null);

  const load = useCallback(async () => {
    try {
      const p = await GetCashPlan();
      setPlan(p);
      setSalaryDay(String(p.settings.salaryDay || 10));
      setSalaryAmount(p.settings.salaryAmount ? formatAmount(String(p.settings.salaryAmount)) : "");
      setReserve(p.settings.reserve ? formatAmount(String(p.settings.reserve)) : "");
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    }
    try {
      setBalances((await ListCashBalances()) ?? []);
    } catch {
      setBalances([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const loadDrill = useCallback(async (b: store.CardBill) => {
    try {
      setDrillTx(
        (await ListTransactions(
          new store.TxFilter({
            from: b.periodStart,
            to: b.periodEnd,
            direction: "expense",
            paymentMethodId: b.cardId,
            sort: "amount_desc",
          })
        )) ?? []
      );
    } catch {
      setDrillTx([]);
    }
  }, []);

  // 카드 → 카드사 (카드 탭의 "카드사". 비어 있으면 카드 이름을 그대로 묶음 이름으로)
  const issuerOf = (ev: store.CashEvent) => {
    const pm = refs.paymentMethods.find((p) => p.id === ev.refId);
    return pm?.issuer?.trim() || ev.label;
  };

  // 일정의 카드 출금 항목 → 같은 카드·같은 출금일의 청구서 (세부 내역 팝업용)
  const billFor = (ev: store.CashEvent) =>
    ev.kind === "card"
      ? [...(plan?.nextDueBills ?? []), ...(plan?.followingBills ?? [])].find(
          (b) => b.cardId === ev.refId && b.dueDate === ev.date
        )
      : undefined;

  const openDrill = (b: store.CardBill) => {
    setDrill(b);
    setDrillTx([]);
    loadDrill(b);
  };

  const saveSettings = async (e: FormEvent) => {
    e.preventDefault();
    setSaveErr("");
    setSaveMsg("");
    setSaving(true);
    try {
      await SaveCashSettings(
        new store.CashSettings({
          salaryDay: Number(salaryDay) || 10,
          salaryAmount: parseAmount(salaryAmount) || 0,
          reserve: parseAmount(reserve) || 0,
        })
      );
      setSaveMsg("저장했습니다.");
      await load();
    } catch (e: any) {
      setSaveErr(String(e));
    } finally {
      setSaving(false);
    }
  };

  const removeBalance = async (b: store.CashBalance) => {
    const ok = await confirm({
      title: "잔액 기록을 삭제할까요?",
      danger: true,
      confirmLabel: "삭제",
      detail: (
        <span className="num">
          {b.asOf} · {won(b.amount)}
          {b.memo ? ` · ${b.memo}` : ""}
        </span>
      ),
    });
    if (!ok) return;
    try {
      await DeleteCashBalance(b.id);
      await load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  // 날짜별로 묶은 타임라인
  const groups = useMemo(() => {
    const out: { date: string; events: store.CashEvent[] }[] = [];
    for (const ev of plan?.events ?? []) {
      const g = out[out.length - 1];
      if (g && g.date === ev.date) g.events.push(ev);
      else out.push({ date: ev.date, events: [ev] });
    }
    return out;
  }, [plan]);

  if (err && !plan)
    return (
      <div className="surface p-5 sm:p-6" role="alert">
        <p className="mb-3 text-sm text-rose-300">자금 계획을 불러오지 못했습니다: {err}</p>
        <button className="btn-ghost" onClick={load}>다시 시도</button>
      </div>
    );
  if (!plan) return <Skeleton />;

  const reserveAmt = plan.settings.reserve;
  const hasBalance = !!plan.balance;
  const after = plan.afterNextDue;
  const afterTone =
    after < 0 ? "text-rose-300" : after < reserveAmt ? "text-amber-300" : "text-emerald-300";
  const nominalDiffers = plan.nextDueBills.some((b) => b.nominalDue !== b.dueDate);
  const nominal = plan.nextDueBills.find((b) => b.nominalDue !== b.dueDate)?.nominalDue;
  const nextEstimated = plan.nextDueBills.some((b) => !b.closed);
  const followingEstimated = plan.followingBills.some((b) => !b.closed);
  const daysToNextDue = plan.nextDue ? dday(plan.nextDue, plan.today) : 0;
  const lowTone =
    plan.lowestBalance < 0 ? "text-rose-300" : plan.lowestBalance < reserveAmt ? "text-amber-300" : "text-white";

  return (
    <div className="space-y-4 lg:space-y-5">
      <div className="flex min-w-0 items-center gap-3">
        <IconTile icon={Wallet} tone="mint" />
        <div className="min-w-0">
          <h2 className="text-2xl font-light tracking-tight text-white">자금 계획</h2>
          <p className="label-mono num">{plan.today}</p>
        </div>
      </div>

      {err && (
        <Callout tone="rose">
          <p className="text-sm text-rose-200">{err}</p>
        </Callout>
      )}

      {!hasBalance && (
        <Callout tone="amber">
          <p className="mb-3 text-sm text-amber-200">
            통장 잔액을 입력하면 카드값을 낼 수 있는지 계산합니다
          </p>
          <BalanceForm onSaved={load} compactLayout />
        </Callout>
      )}

      {/* 히어로: 다음 카드 결제 */}
      <div className="hero-grain grid gap-6 p-5 sm:p-6 lg:grid-cols-3">
        <div className="flex flex-col justify-between gap-5 lg:col-span-1">
          <div className="flex items-center gap-3">
            <IconTile icon={CreditCard} tone="mint" size="sm" />
            <div className="label-mono">다음 카드 결제</div>
          </div>
          {plan.nextDue ? (
            <div>
              <p className="num text-sm text-zinc-300">
                {nominalDiffers && nominal ? (
                  <>
                    {dateLabel(nominal)} → <strong className="font-medium text-white">{dateLabel(plan.nextDue)}</strong> 출금
                  </>
                ) : (
                  <>
                    <strong className="font-medium text-white">{dateLabel(plan.nextDue)}</strong> 출금
                  </>
                )}
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <span className="badge num bg-white/[0.08] text-zinc-200">{ddayText(daysToNextDue)}</span>
                {nextEstimated && <EstBadge />}
              </div>
              <div className="num mt-3 text-4xl font-light tracking-tight text-rose-300">
                <CountUp value={plan.nextDueTotal} format={won} />
              </div>
            </div>
          ) : (
            <p className="text-sm text-zinc-400">예정된 카드 결제가 없습니다.</p>
          )}
        </div>

        <div className="surface-inset flex flex-col gap-2 p-4">
          <p className="label-mono">현재 잔액</p>
          {hasBalance ? (
            <>
              <p className="num text-2xl font-light tracking-tight text-white">
                <CountUp value={plan.balanceNow} format={won} />
              </p>
              <p className="num text-xs text-zinc-500">
                {shortDate(plan.balance!.asOf)} 입력 잔액 {won(plan.balance!.amount)} · 이후{" "}
                <span className={plan.balanceAdjust >= 0 ? "text-emerald-300" : "text-rose-300"}>
                  {signed(plan.balanceAdjust)}
                </span>{" "}
                반영
              </p>
            </>
          ) : (
            <p className="text-sm text-amber-300">잔액을 입력하세요</p>
          )}
        </div>

        <div className="surface-inset flex flex-col gap-2 p-4">
          <p className="label-mono">결제 후 예상 잔액</p>
          {hasBalance && plan.nextDue ? (
            <>
              <p className={`num text-2xl font-light tracking-tight ${afterTone}`}>
                <CountUp value={after} format={won} />
              </p>
              <p className={`num text-xs ${afterTone}`}>
                {after < 0
                  ? `${won(-after)} 부족`
                  : after < reserveAmt
                  ? `최소 잔액 ${won(reserveAmt)} 미만`
                  : "여유 있음"}
              </p>
            </>
          ) : (
            <p className="text-sm text-zinc-500">-</p>
          )}
        </div>
      </div>

      {/* 요약 타일 */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div className="surface-inset flex flex-col gap-2 p-4">
          <p className="label-mono">다음 월급</p>
          <p className="num text-xl font-light tracking-tight text-emerald-300">
            {plan.nextSalary ? shortDate(plan.nextSalary) : "-"}
            {plan.nextSalary && (
              <span className="ml-2 text-sm text-zinc-400">{ddayText(dday(plan.nextSalary, plan.today))}</span>
            )}
          </p>
          <p className="num text-xs text-zinc-500">
            {won(plan.salaryUsed)} · {plan.salaryBasis}
          </p>
        </div>
        <div className="surface-inset flex flex-col gap-2 p-4">
          <p className="label-mono">월급 전까지 써도 되는 돈</p>
          <p
            className={`num text-xl font-light tracking-tight ${
              plan.safeToSpend < 0 ? "text-rose-300" : "text-white"
            }`}
          >
            {hasBalance ? won(plan.safeToSpend) : "-"}
          </p>
          <p className="num text-xs text-zinc-500">
            하루 {won(plan.dailyAllowance)} · {plan.daysToSalary}일
          </p>
        </div>
        <div className="surface-inset flex flex-col gap-2 p-4">
          <p className="label-mono">그다음 결제</p>
          <p className="num text-xl font-light tracking-tight text-white">
            {plan.followingDue ? shortDate(plan.followingDue) : "-"}
            {plan.followingDue && (
              <span className="ml-2 text-sm text-zinc-400">
                {ddayText(dday(plan.followingDue, plan.today))}
              </span>
            )}
          </p>
          <p className="num text-xs text-zinc-500">
            지금까지 {won(plan.followingTotal)} → 예상{" "}
            <span className="text-amber-300">{won(plan.followingProjected)}</span>
          </p>
        </div>
        <div className="surface-inset flex flex-col gap-2 p-4">
          <p className="label-mono">기간 중 최저 잔액</p>
          <p className={`num text-xl font-light tracking-tight ${hasBalance ? lowTone : "text-white"}`}>
            {hasBalance ? won(plan.lowestBalance) : "-"}
          </p>
          <p className="num text-xs text-zinc-500">{plan.lowestDate ? `${shortDate(plan.lowestDate)} 기준` : ""}</p>
        </div>
      </div>

      {/* 잔액 흐름 */}
      <div className="surface p-5 sm:p-6">
        <SectionHead
          icon={TrendingDown}
          title="잔액 흐름"
          sub={plan.horizon ? `${shortDate(plan.today)} ~ ${shortDate(plan.horizon)}` : undefined}
        />
        <FlowChart plan={plan} />
      </div>

      {/* 타임라인 */}
      <div className="surface p-5 sm:p-6">
        <SectionHead icon={CalendarClock} title="입출금 일정" sub={`${plan.events.length} events`} />
        {groups.length === 0 ? (
          <ChartEmpty>예정된 입출금이 없습니다</ChartEmpty>
        ) : (
          <div className="space-y-4">
            {groups.map((g) => (
              <div key={g.date}>
                <p className="label-mono !normal-case mb-1.5">
                  {dateLabel(g.date)} · {ddayText(dday(g.date, plan.today))}
                </p>
                <div className="surface-inset divide-y divide-white/[0.06]">
                  {groupByIssuer(g.events, issuerOf).map((item, i) => {
                    if (item.kind === "issuer") {
                      // 카드사 묶음: 실제로는 카드사별로 한 번에 빠져나가므로 합계를 먼저 보여 준다
                      const last = item.events[item.events.length - 1];
                      const sumOut = item.events.reduce((s, e) => s + e.amount, 0);
                      return (
                        <div key={`iss-${i}`}>
                          <div className="flex items-center gap-3 px-4 py-2.5">
                            <Landmark className="h-4 w-4 shrink-0 text-rose-300" strokeWidth={1.75} />
                            <span className="min-w-0 flex-1 truncate text-sm font-medium text-white">
                              {item.issuer}
                              <span className="label-mono !normal-case ml-2">{item.events.length}장</span>
                            </span>
                            {item.events.some((e) => e.estimated) && <EstBadge />}
                            <span className="num whitespace-nowrap text-sm font-semibold text-rose-300">
                              {signed(sumOut)}
                            </span>
                            <span
                              className={`num w-28 whitespace-nowrap text-right text-xs sm:w-36 ${
                                last.balance < 0 ? "text-rose-300" : "text-zinc-400"
                              }`}
                            >
                              {won(last.balance)}
                            </span>
                            <span className="w-4 shrink-0" aria-hidden />
                          </div>
                          {item.events.map((ev, j) => (
                            <EventRow key={j} ev={ev} bill={billFor(ev)} onOpen={openDrill} nested />
                          ))}
                        </div>
                      );
                    }
                    return <EventRow key={i} ev={item.ev} bill={billFor(item.ev)} onOpen={openDrill} />;
                  })}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 카드 청구 */}
      <div className="surface p-5 sm:p-6">
        <SectionHead
          icon={CreditCard}
          title={plan.nextDue ? `다음 결제 (${dateLabel(plan.nextDue)})` : "다음 결제"}
          sub={nextEstimated ? "이용기간이 끝나지 않은 카드는 예상치" : undefined}
        />
        <BillTable bills={plan.nextDueBills} onPick={openDrill} />
      </div>
      {plan.followingDue && (
        <div className="surface p-5 sm:p-6">
          <SectionHead
            icon={Flag}
            title={`그다음 결제 (${dateLabel(plan.followingDue)})`}
            sub="지금 쓰는 카드값이 쌓이는 중"
          />
          <BillTable bills={plan.followingBills} onPick={openDrill} />
          {followingEstimated && (
            <p className="mt-2 text-xs text-zinc-500">예상 열은 이용기간 끝까지의 추정 청구액입니다.</p>
          )}
        </div>
      )}

      {/* 설정 + 잔액 기록 */}
      <div className="grid items-start gap-4 lg:grid-cols-2 lg:gap-5">
        <div className="surface p-5 sm:p-6">
          <SectionHead icon={Settings2} title="계획 설정" />
          <form onSubmit={saveSettings} className="space-y-3">
            <label className="block">
              <span className="field-label">월급일 (매월)</span>
              <select
                className="input-field num"
                value={salaryDay}
                onChange={(e) => setSalaryDay(e.target.value)}
              >
                {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                  <option key={d} value={d}>
                    {d}일
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="field-label">예상 월급 (원) — 비우면 자동</span>
              <input
                type="text"
                inputMode="numeric"
                className="input-field num"
                placeholder={
                  plan.salaryAuto ? `자동 ${formatAmount(String(plan.salaryAuto))} (${plan.salaryBasis})` : "자동"
                }
                value={salaryAmount}
                onChange={(e) => setSalaryAmount(formatAmount(e.target.value))}
              />
            </label>
            <label className="block">
              <span className="field-label">최소 잔액 (원) — 항상 남겨둘 돈</span>
              <input
                type="text"
                inputMode="numeric"
                className="input-field num"
                placeholder="0"
                value={reserve}
                onChange={(e) => setReserve(formatAmount(e.target.value))}
              />
            </label>
            <div className="flex items-center gap-3">
              <button type="submit" className="btn-primary" disabled={saving}>
                {saving ? "저장 중…" : "설정 저장"}
              </button>
              {saveMsg && <span className="text-xs text-emerald-300">{saveMsg}</span>}
              {saveErr && <span className="text-xs text-rose-300">{saveErr}</span>}
            </div>
          </form>
        </div>

        <div className="surface p-5 sm:p-6">
          <SectionHead icon={PiggyBank} title="통장 잔액 기록" sub="가장 최근 기록을 기준으로 계산합니다" />
          <BalanceForm onSaved={load} />
          <div className="mt-4">
            {balances.length === 0 ? (
              <p className="py-3 text-center text-sm text-zinc-500">기록이 없습니다.</p>
            ) : (
              <div className="surface-inset divide-y divide-white/[0.06]">
                {balances.slice(0, 8).map((b) => (
                  <div key={b.id} className="flex items-center gap-3 px-4 py-2.5">
                    <span className="num whitespace-nowrap text-xs text-zinc-400">{b.asOf}</span>
                    <span className="num text-sm text-zinc-100">{won(b.amount)}</span>
                    <span className="min-w-0 flex-1 truncate text-xs text-zinc-500">{b.memo}</span>
                    <button
                      type="button"
                      className="btn-icon shrink-0"
                      aria-label="삭제"
                      title="삭제"
                      onClick={() => removeBalance(b)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {drill && (
        <TxDrillModal
          title={drill.cardName}
          ym={drill.periodEnd.slice(0, 7)}
          heading={`${drill.cardName} 이용 내역 · ${drill.periodStart} ~ ${drill.periodEnd}`}
          months={[]}
          values={[]}
          txs={drillTx}
          onPickMonth={() => {}}
          onEdit={setEditTx}
          onClose={() => setDrill(null)}
        />
      )}
      {editTx && (
        <EditModal
          tx={editTx}
          refs={refs}
          onClose={() => setEditTx(null)}
          onSaved={() => {
            setEditTx(null);
            if (drill) loadDrill(drill);
            load();
          }}
        />
      )}
      {confirmUI}
    </div>
  );
}
