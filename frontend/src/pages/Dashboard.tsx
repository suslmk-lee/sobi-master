import { ReactNode, useCallback, useEffect, useState } from "react";
import { GetCategoryCompare, GetDashboard } from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import {
  AlertTriangle,
  BarChart3,
  ChevronRight,
  CreditCard,
  GitCompare,
  Info,
  LayoutDashboard,
  PieChart,
  Siren,
  Target,
  TrendingUp,
  Users,
  Wallet,
  type LucideIcon,
} from "lucide-react";
import { won } from "../lib";
import IconTile from "../components/ui/IconTile";
import CountUp from "../components/CountUp";
import MonthPicker from "../MonthPicker";
import Bars from "../Bars";
import { DailyChart, DeltaBars, Donut, TrendChart } from "../charts";
import { autoColor } from "../PmChip";
import { DashboardSkeleton, Sk } from "../Skeleton";

// 카테고리별 전월 대비에서 기본으로 보여 줄 줄 수(나머지는 접는다)
const CMP_ROWS = 8;

// 전월 대비 증감 배지. prev 가 0이면 비교 불가로 표시하지 않는다.
function Delta({ cur, prev, goodWhenDown }: { cur: number; prev: number; goodWhenDown?: boolean }) {
  if (!prev) return null;
  const pct = Math.round(((cur - prev) / prev) * 100);
  if (pct === 0) return <span className="num text-xs text-zinc-400">전월과 동일</span>;
  const up = pct > 0;
  const good = goodWhenDown ? !up : up;
  return (
    <span className={`num text-xs ${good ? "text-emerald-300" : "text-rose-300"}`}>
      {up ? "▲" : "▼"} {Math.abs(pct)}% <span className="text-zinc-500">전월 대비</span>
    </span>
  );
}

// 카드 내부 섹션 헤더 (IconTile + 제목 + 우측 컨트롤)
function SectionHead({
  icon,
  title,
  sub,
  children,
}: {
  icon: LucideIcon;
  title: string;
  sub?: string;
  children?: ReactNode;
}) {
  return (
    <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <IconTile icon={icon} tone="mint" size="sm" />
        <div className="min-w-0">
          <h3 className="section-title">{title}</h3>
          {sub && <p className="label-mono mt-0.5">{sub}</p>}
        </div>
      </div>
      {children}
    </div>
  );
}

export default function Dashboard({
  month,
  setMonth,
  goUnclassified,
}: {
  month: string;
  setMonth: (m: string) => void;
  goUnclassified: () => void;
}) {
  const [sum, setSum] = useState<store.MonthlySummary | null>(null);
  const [prev, setPrev] = useState<store.MonthlySummary | null>(null);
  const [trend, setTrend] = useState<store.MonthPoint[]>([]);
  const [daily, setDaily] = useState<store.DayPoint[]>([]);
  const [top, setTop] = useState<store.NamedAmount[]>([]);
  const [cards, setCards] = useState<store.CardStatus[]>([]);
  const [alerts, setAlerts] = useState<main.Alert[]>([]);
  const [budgets, setBudgets] = useState<store.BudgetStatus[]>([]);
  // 결제수단 이름 → 표시 색 (지정 색 우선, 없으면 자동 색)
  const [pmColors, setPmColors] = useState<Record<string, string>>({});
  // 카테고리별 전월 대비 (집계 기준·펼침 여부가 바뀌면 따로 조회)
  const [cmp, setCmp] = useState<store.CategoryCompare | null>(null);
  const [cmpLevel, setCmpLevel] = useState<"main" | "sub">("main");
  const [cmpAll, setCmpAll] = useState(false);

  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    try {
      const [y, m] = month.split("-").map(Number);
      const dash = await GetDashboard(y, m);
      setSum(dash.summary);
      setPrev(dash.prev);
      setTrend(dash.trend);
      setDaily(dash.daily);
      setTop(dash.top);
      setCards(dash.cards);
      setAlerts(dash.alerts);
      setBudgets(dash.budgets);
      const colors: Record<string, string> = {};
      for (const pm2 of dash.paymentMethods) {
        colors[pm2.name] = pm2.color || autoColor(pm2.name);
      }
      setPmColors(colors);
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    }
  }, [month]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    const [y, m] = month.split("-").map(Number);
    setCmpAll(false);
    GetCategoryCompare(y, m, cmpLevel)
      .then(setCmp)
      .catch(() => setCmp(null));
  }, [month, cmpLevel]);

  if (err)
    return (
      <div className="surface p-5 sm:p-6">
        <p className="mb-3 text-sm text-rose-300">대시보드 데이터를 불러오지 못했습니다: {err}</p>
        <button className="btn-ghost" onClick={load}>다시 시도</button>
      </div>
    );
  if (!sum) return <DashboardSkeleton />;
  const net = sum.totalIncome - sum.totalExpense;
  const prevNet = prev ? prev.totalIncome - prev.totalExpense : 0;
  const expenseCats = sum.byCategory.filter((c) => c.kind === "expense");
  const transferCats = sum.byCategory.filter((c) => c.kind === "transfer");

  return (
    <div className="space-y-4 lg:space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={LayoutDashboard} tone="mint" />
          <div className="min-w-0">
            <h2 className="text-2xl font-light tracking-tight text-white">대시보드</h2>
            <p className="label-mono num">{month}</p>
          </div>
        </div>
        <MonthPicker value={month} onChange={setMonth} />
      </div>

      {sum.unclassifiedCount > 0 && (
        <button
          onClick={goUnclassified}
          className="surface flex w-full items-center gap-3 border-l-2 !border-l-amber-300/60 px-5 py-3.5 text-left"
        >
          <AlertTriangle className="h-5 w-5 shrink-0 text-amber-300" />
          <span className="min-w-0 flex-1 text-sm text-amber-200">
            미분류 거래 <span className="num font-medium">{sum.unclassifiedCount}</span>건 — 지금 분류하기
          </span>
          <ChevronRight className="h-4 w-4 shrink-0 text-amber-300/70" />
        </button>
      )}

      {alerts.length > 0 && (
        <div className="space-y-2">
          {alerts.map((a, i) => {
            const tone =
              a.level === "danger"
                ? { bar: "!border-l-rose-300/60", icon: "text-rose-300", Icon: Siren }
                : a.level === "warn"
                ? { bar: "!border-l-amber-300/60", icon: "text-amber-300", Icon: AlertTriangle }
                : { bar: "!border-l-sky-300/60", icon: "text-sky-300", Icon: Info };
            return (
              <div
                className={`surface flex items-start gap-3 border-l-2 px-5 py-3.5 ${tone.bar}`}
                key={i}
              >
                <tone.Icon className={`mt-0.5 h-4 w-4 shrink-0 ${tone.icon}`} />
                <div className="min-w-0">
                  <strong className="text-sm font-medium text-white">{a.title}</strong>
                  <span className="ml-2 text-xs text-zinc-400">{a.detail}</span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* KPI: 수지 히어로 + 수입·지출·이체 타일 */}
      <div className="grid items-stretch gap-4 lg:grid-cols-3 lg:gap-5">
        <div className="hero-grain flex flex-col justify-between gap-6 p-5 sm:p-6">
          <div className="flex items-center gap-3">
            <IconTile icon={Wallet} tone="mint" size="sm" />
            <div className="label-mono">수지 (수입 − 지출)</div>
          </div>
          <div>
            <div
              className={`num text-4xl font-light tracking-tight ${
                net >= 0 ? "text-emerald-300" : "text-rose-300"
              }`}
            >
              <CountUp value={net} format={won} />
            </div>
            <div className="mt-2">
              <Delta cur={net} prev={prevNet} />
            </div>
          </div>
        </div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 lg:col-span-2">
          <div className="surface-inset flex flex-col gap-2 p-4">
            <p className="label-mono">수입</p>
            <p className="num text-2xl font-light tracking-tight text-emerald-300">
              <CountUp value={sum.totalIncome} format={won} />
            </p>
            <Delta cur={sum.totalIncome} prev={prev?.totalIncome ?? 0} />
          </div>
          <div className="surface-inset flex flex-col gap-2 p-4">
            <p className="label-mono">지출</p>
            <p className="num text-2xl font-light tracking-tight text-rose-300">
              <CountUp value={sum.totalExpense} format={won} />
            </p>
            <Delta cur={sum.totalExpense} prev={prev?.totalExpense ?? 0} goodWhenDown />
          </div>
          <div className="surface-inset flex flex-col gap-2 p-4">
            <p className="label-mono">투자/저축 이체</p>
            <p className="num text-2xl font-light tracking-tight text-sky-300">
              <CountUp value={sum.totalTransfer} format={won} />
            </p>
            <Delta cur={sum.totalTransfer} prev={prev?.totalTransfer ?? 0} />
          </div>
        </div>
      </div>

      {/* 총액이 왜 늘었는지는 카테고리를 쪼개야 보인다 */}
      <div className="surface p-5 sm:p-6">
        <SectionHead icon={GitCompare} title="카테고리별 전월 대비" sub="증감이 큰 순">
          <div className="glass-pill flex gap-1 p-1" role="group" aria-label="집계 기준">
            <button
              className={`pill-item ${cmpLevel === "main" ? "pill-item-active" : ""}`}
              aria-pressed={cmpLevel === "main"}
              onClick={() => setCmpLevel("main")}
            >
              주
            </button>
            <button
              className={`pill-item ${cmpLevel === "sub" ? "pill-item-active" : ""}`}
              aria-pressed={cmpLevel === "sub"}
              onClick={() => setCmpLevel("sub")}
            >
              부
            </button>
          </div>
        </SectionHead>
        {!cmp ? (
          <Sk h={200} r={12} />
        ) : (
          <>
            <p className="mb-3 text-sm text-zinc-400">
              <span className="num">
                {Number(cmp.prevMonth.slice(5))}월 {won(cmp.prevTotal)}
              </span>{" "}
              →{" "}
              <span className="num">
                {Number(cmp.month.slice(5))}월{" "}
                <strong className="font-medium text-white">{won(cmp.total)}</strong>
              </span>
              {cmp.prevTotal > 0 && (
                <>
                  {" "}
                  <span className={`num ${cmp.total > cmp.prevTotal ? "text-rose-300" : "text-emerald-300"}`}>
                    {cmp.total > cmp.prevTotal ? "▲" : "▼"} {won(Math.abs(cmp.total - cmp.prevTotal))}
                  </span>
                </>
              )}
              {" · 증감이 큰 순"}
            </p>
            <DeltaBars rows={cmpAll ? cmp.rows : cmp.rows.slice(0, CMP_ROWS)} />
            {cmp.rows.length > CMP_ROWS && (
              <button className="btn-ghost mt-3 !py-1.5 !text-xs" onClick={() => setCmpAll((v) => !v)}>
                {cmpAll ? "변화 큰 항목만 보기" : `나머지 ${cmp.rows.length - CMP_ROWS}개 더 보기`}
              </button>
            )}
          </>
        )}
      </div>

      <div className="surface p-5 sm:p-6">
        <SectionHead icon={TrendingUp} title="최근 6개월 수입·지출 추이" />
        <TrendChart points={trend} />
      </div>

      {budgets.length > 0 && (
        <div className="surface p-5 sm:p-6">
          <SectionHead
            icon={Target}
            title={`예산 현황 (${Number(month.slice(5))}월)`}
            sub={`${budgets.length} budgets`}
          />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {budgets.map((b) => {
              const pct = Math.min(100, b.pct);
              const fill = b.over ? "bg-rose-400" : b.pct >= 80 ? "bg-amber-400" : "bg-mint-400";
              return (
                <div className="surface-inset flex flex-col gap-2 p-4" key={b.categoryId}>
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="truncate text-sm font-medium text-white">{b.category}</span>
                    <span className={`num text-xs ${b.over ? "text-rose-300" : "text-zinc-400"}`}>
                      {b.pct}%
                    </span>
                  </div>
                  <div className="h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                    <div className={`h-full rounded-full ${fill}`} style={{ width: `${pct}%` }} />
                  </div>
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="num text-xs text-zinc-400">
                      {won(b.spent)} / {won(b.amount)}
                    </span>
                    <span className={`num text-xs ${b.over ? "text-rose-300" : "text-zinc-500"}`}>
                      {b.over ? `${won(-b.remaining)} 초과` : `${won(b.remaining)} 남음`}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <div className="grid items-stretch gap-4 lg:grid-cols-2 lg:gap-5">
        <div className="surface h-full p-5 sm:p-6">
          <SectionHead icon={BarChart3} title="이번 달 일별 지출 · 누적" />
          <DailyChart points={daily} />
        </div>
        <div className="surface h-full p-5 sm:p-6">
          <SectionHead icon={PieChart} title="카테고리별 지출 비중" />
          <Donut rows={expenseCats} centerLabel="총 지출" />
        </div>
        <div className="surface h-full p-5 sm:p-6">
          <SectionHead icon={Users} title="누구에게 쓴 돈인가" />
          <Donut rows={sum.byMember} centerLabel="총 지출" />
        </div>
        <Bars title="지출 TOP 10 가맹점" rows={top} />
        <Bars title="결제수단별 지출 (어느 카드인가)" rows={sum.byPaymentMethod} colors={pmColors} />
        <Bars title="이체 (투자·저축)" rows={transferCats} />
      </div>

      {cards.length > 0 && (
        <div className="surface p-5 sm:p-6">
          <SectionHead icon={CreditCard} title="카드 실적 현황" sub={`${cards.length} cards`} />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {cards.map((st) => {
              const pct =
                st.card.perfTarget > 0
                  ? Math.min(100, Math.round((st.spent / st.card.perfTarget) * 100))
                  : 0;
              const color = st.card.color || autoColor(st.card.name);
              return (
                <div className="surface-inset flex flex-col gap-2 p-4" key={st.card.id}>
                  <strong className="flex items-center gap-2 text-sm font-medium text-white">
                    <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: color }} />
                    <span className="truncate">
                      {st.card.issuer ? `${st.card.issuer} ` : ""}
                      {st.card.name}
                    </span>
                  </strong>
                  <span className="label-mono num !normal-case">
                    {st.periodStart} ~ {st.periodEnd}
                  </span>
                  <span className="num text-sm text-zinc-200">
                    {won(st.spent)}
                    {st.card.perfTarget > 0 && (
                      <span className="text-zinc-500"> / {won(st.card.perfTarget)}</span>
                    )}
                  </span>
                  {st.excluded > 0 && (
                    <span className="num text-xs text-zinc-500" title="세금·공과금 등 실적 제외로 표시한 결제">
                      실적 제외 {won(st.excluded)}
                    </span>
                  )}
                  {st.card.perfTarget > 0 && (
                    <>
                      <div className="h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                        <div className="h-full rounded-full" style={{ width: `${pct}%`, background: color }} />
                      </div>
                      {st.achieved ? (
                        <span className="badge self-start bg-emerald-500/10 text-emerald-300">실적 달성 ✓</span>
                      ) : (
                        <span className="badge num self-start bg-amber-500/10 text-amber-300">
                          {won(st.remaining)} 남음
                        </span>
                      )}
                    </>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
