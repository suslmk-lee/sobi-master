import { useCallback, useEffect, useState } from "react";
import {
  Activity,
  BarChart3,
  Calendar,
  CalendarDays,
  CalendarRange,
  CreditCard,
  Gauge,
  GitCompare,
  Grid3x3,
  Layers,
  Repeat,
  TrendingUp,
  UserRound,
  Users,
  type LucideIcon,
} from "lucide-react";
import {
  GetCardCategoryBreakdown,
  GetCardPaces,
  GetCategoryDetail,
  GetCategoryMatrix,
  GetCategoryMemberBreakdown,
  GetCategoryMonthly,
  GetCategoryDetailTx,
  GetCategoryMonthlyTx,
  GetMatrixCellTx,
  GetCategoryTrend,
  GetCumulativeCompare,
  GetDailyByDimension,
  GetMemberCategoryBreakdown,
  GetMemberStats,
  GetMemberTrend,
  GetRecurringStatus,
  GetYearSummary,
  ListTransactions,
} from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import { categoryOptions, Refs, won } from "../lib";
import MonthPicker from "../MonthPicker";
import Bars from "../Bars";
import EditModal from "../EditModal";
import TxDrillModal from "../TxDrillModal";
import CompareChart from "../CompareChart";
import SpendCalendar from "../SpendCalendar";
import WeekdayPanel from "../WeekdayPanel";
import {
  Heatmap,
  LineChart,
  LineSeries,
  PaceSparkline,
  StackedBars,
  TrendChart,
} from "../charts";
import { autoColor } from "../PmChip";
import { Sk } from "../Skeleton";
import IconTile from "../components/ui/IconTile";
import { PctDelta, SectionHead, Seg } from "../components/ui/StatsUi";
import MemberAnalysis from "./MemberAnalysis";
import { useThemeColors } from "../appearance/AppearanceContext";

type Dim = "paymentMethod" | "member" | "category";
type Mode = "daily" | "cumulative";

// 카테고리 월별 상세를 쪼개는 기준
type CatBy = "merchant" | "sub" | "member" | "payment";
// 가맹점으로 쪼개면 줄이 길어진다. 기본은 상위 N개만 보여 주고 나머지는 접는다.
const CAT_ROW_LIMIT = 15;
const CAT_BY: { key: CatBy; label: string }[] = [
  { key: "merchant", label: "가맹점" },
  { key: "sub", label: "부 카테고리" },
  { key: "member", label: "귀속자" },
  { key: "payment", label: "결제수단" },
];

const DIM_LABEL: { key: Dim; label: string }[] = [
  { key: "paymentMethod", label: "카드/결제수단" },
  { key: "member", label: "귀속자" },
  { key: "category", label: "카테고리" },
];


// "2026-03" → "3월"
const monthLabel = (ym: string) => `${Number(ym.slice(5))}월`;

// 전년 대비 증감 배지
function Yoy({ cur, prev, goodWhenDown }: { cur: number; prev: number; goodWhenDown?: boolean }) {
  if (!prev) return <span className="text-xs text-zinc-500">전년 데이터 없음</span>;
  const pct = Math.round(((cur - prev) / prev) * 100);
  if (pct === 0) return <span className="text-xs text-zinc-400">전년과 동일</span>;
  const up = pct > 0;
  const good = goodWhenDown ? !up : up;
  return (
    <span className={`num text-xs ${good ? "text-emerald-300" : "text-rose-300"}`}>
      {up ? "▲" : "▼"} {Math.abs(pct)}% <span className="text-zinc-500">전년 대비</span>
    </span>
  );
}

// 통계 서브메뉴. 카테고리 상세·구성원 분석은 위에서 고른 대상(카테고리·사람)과 기간이
// 아래 모든 카드에 걸리므로 화면을 따로 둔다.
type View = "overview" | "category" | "member";
const VIEWS: { key: View; label: string; desc: string; icon: LucideIcon }[] = [
  { key: "overview", label: "전체 통계", desc: "추이 · 히트맵 · 귀속자 · 카드 · 연간", icon: BarChart3 },
  { key: "category", label: "카테고리 상세", desc: "한 카테고리를 요약 · 구성 · 월별로", icon: Layers },
  { key: "member", label: "구성원 분석", desc: "한 사람의 소비를 요약 · 추이 · 습관으로", icon: UserRound },
];

function StatsNav({ value, onChange }: { value: View; onChange: (v: View) => void }) {
  return (
    <nav className="surface flex gap-1 overflow-x-auto p-2" aria-label="통계 메뉴">
      {VIEWS.map((v) => {
        const Icon = v.icon;
        const active = value === v.key;
        return (
          <button
            key={v.key}
            type="button"
            className={`pill-item !rounded-xl text-left ${active ? "pill-item-active" : ""}`}
            aria-current={active ? "page" : undefined}
            onClick={() => onChange(v.key)}
          >
            <span className="flex items-center gap-2 text-sm">
              <Icon className="h-4 w-4 shrink-0" strokeWidth={1.75} />
              {v.label}
            </span>
            <span className="label-mono mt-0.5 block !text-[10px]">{v.desc}</span>
          </button>
        );
      })}
    </nav>
  );
}

const PACE_BADGE = "badge self-start";

export default function StatsPage({
  refs,
  month,
  setMonth,
}: {
  refs: Refs;
  month: string;
  setMonth: (m: string) => void;
}) {
  const theme = useThemeColors();
  const [view, setView] = useState<View>("overview");
  const [dim, setDim] = useState<Dim>("paymentMethod");
  const [mode, setMode] = useState<Mode>("cumulative");

  const [dimData, setDimData] = useState<Record<string, store.DailyByDimension>>({});
  const [compare, setCompare] = useState<store.CumCompare | null>(null);
  const [catTrend, setCatTrend] = useState<store.CategoryTrend | null>(null);
  const [cardCat, setCardCat] = useState<store.CardCategory[]>([]);
  const [paces, setPaces] = useState<store.CardPace[]>([]);
  const [recurring, setRecurring] = useState<main.RecurringItem[]>([]);
  const [year, setYear] = useState<store.YearSummary | null>(null);
  const [memberCat, setMemberCat] = useState<store.CardCategory[]>([]);
  const [memberTrend, setMemberTrend] = useState<store.CategoryTrend | null>(null);
  const [memberStats, setMemberStats] = useState<store.MemberStat[]>([]);
  const [catMember, setCatMember] = useState<store.CardCategory[]>([]);
  const [matrix, setMatrix] = useState<store.CategoryTrend | null>(null);

  // 카테고리 집계 기준: 주(부를 상위로 합산) / 부(말단 그대로)
  const [catLevel, setCatLevel] = useState<"main" | "sub">("main");

  // 카테고리 상세 드릴다운 (선택 카테고리·기간에 따라 별도 조회)
  const expenseCats = refs.categories.filter((c) => c.kind === "expense");
  const [catId, setCatId] = useState<number>(0);
  const [catPeriod, setCatPeriod] = useState<"month" | "recent30" | "prev">("month");
  const [detail, setDetail] = useState<store.CategoryDetail | null>(null);
  // 월 × 항목 표: 어느 달에 무엇 때문에 늘었는지 보려고 따로 조회한다
  const [catBy, setCatBy] = useState<CatBy>("merchant");
  const [catMonths, setCatMonths] = useState(6);
  const [catMonthly, setCatMonthly] = useState<store.CategoryTrend | null>(null);
  const [catAll, setCatAll] = useState(false);
  // 표의 칸을 누르면 그 달·그 항목의 거래를 모달로 열어 고칠 수 있게 한다.
  // kind 로 어느 표에서 열렸는지 구분한다 (item = 월별 상세, matrix = 카테고리 히트맵).
  // day = 지출 달력에서 고른 하루(key 는 "YYYY-MM-DD", label 은 "10월 12일 (월)")
  // detail = 카테고리 상세 막대 한 줄(by 기준 항목 key, 기간은 상세의 기간 토글, label 은 모달 제목)
  const [drill, setDrill] = useState<{
    kind: "item" | "matrix" | "day" | "detail";
    key: string;
    ym: string;
    label?: string;
    by?: CatBy;
  } | null>(null);
  const [drillTx, setDrillTx] = useState<store.Transaction[]>([]);
  const [editTx, setEditTx] = useState<store.Transaction | null>(null);
  const [bump, setBump] = useState(0); // 수정 후 표·상세를 다시 읽기 위한 카운터

  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [y, m] = month.split("-").map(Number);
      const [pmD, memD, catD, cmp, ct, cc, pc, rec, yr, mcat, mtr, mst, cmem, mtx] =
        await Promise.all([
          GetDailyByDimension(y, m, "paymentMethod"),
          GetDailyByDimension(y, m, "member"),
          GetDailyByDimension(y, m, catLevel === "sub" ? "categorySub" : "category"),
          GetCumulativeCompare(y, m),
          GetCategoryTrend(y, m, 6, catLevel),
          GetCardCategoryBreakdown(y, m),
          GetCardPaces(),
          GetRecurringStatus(y, m),
          GetYearSummary(y),
          GetMemberCategoryBreakdown(y, m),
          GetMemberTrend(y, m, 6),
          GetMemberStats(y, m),
          GetCategoryMemberBreakdown(y, m, catLevel),
          GetCategoryMatrix(y, m, 6, catLevel),
        ]);
      setDimData({ paymentMethod: pmD, member: memD, category: catD });
      setCompare(cmp);
      setCatTrend(ct);
      setCardCat(cc);
      setPaces(pc);
      setRecurring(rec);
      setYear(yr);
      setMemberCat(mcat);
      setMemberTrend(mtr);
      setMemberStats(mst);
      setCatMember(cmem);
      setMatrix(mtx);
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setLoading(false);
    }
  }, [month, catLevel, bump]);

  // 보이는 화면의 데이터만 읽는다(서브메뉴를 바꾸면 그 화면 기준으로 새로 읽는다)
  useEffect(() => {
    if (view === "overview") load();
  }, [load, view]);

  // 카테고리 미선택 시 첫 지출 카테고리를 기본 선택
  useEffect(() => {
    if (catId === 0 && expenseCats.length > 0) setCatId(expenseCats[0].id);
  }, [catId, expenseCats]);

  // 선택 카테고리 상세 조회 (월/카테고리/기간 변경 시)
  useEffect(() => {
    if (catId === 0 || view !== "category") return;
    const [y, m] = month.split("-").map(Number);
    GetCategoryDetail(y, m, catId, catPeriod)
      .then(setDetail)
      .catch(() => setDetail(null));
  }, [view, month, catId, catPeriod, bump]);

  // 선택 카테고리의 월 × 항목 표 (기간 토글과 무관하게 항상 최근 N개월 기준)
  useEffect(() => {
    if (catId === 0 || view !== "category") return;
    const [y, m] = month.split("-").map(Number);
    GetCategoryMonthly(y, m, catMonths, catId, catBy)
      .then(setCatMonthly)
      .catch(() => setCatMonthly(null));
  }, [view, month, catId, catBy, catMonths, bump]);

  // 기준·카테고리를 바꾸면 다시 상위 항목부터 보고, 열려 있던 드릴다운은 닫는다
  useEffect(() => {
    setCatAll(false);
    setDrill(null);
  }, [catId, catBy, catMonths, catLevel, month, catPeriod]);

  // 드릴다운: 고른 칸의 거래 목록.
  // 집계와 같은 기준으로 되짚어야 칸 금액과 목록 합계가 맞으므로 표마다 전용 조회를 쓴다.
  const loadDrill = useCallback(async () => {
    if (!drill) return;
    const [y, m] = month.split("-").map(Number);
    try {
      setDrillTx(
        drill.kind === "day"
          ? await ListTransactions(
              new store.TxFilter({ from: drill.key, to: drill.key, direction: "expense", sort: "amount_desc" })
            )
          : drill.kind === "matrix"
          ? await GetMatrixCellTx(drill.ym, drill.key, catLevel)
          : catId === 0
          ? []
          : drill.kind === "detail"
          ? await GetCategoryDetailTx(y, m, catId, catPeriod, drill.by ?? "member", drill.key)
          : await GetCategoryMonthlyTx(drill.ym, catId, catBy, drill.key)
      );
    } catch {
      setDrillTx([]);
    }
  }, [drill, catId, catBy, catLevel, catPeriod, month]);

  useEffect(() => {
    loadDrill();
  }, [loadDrill]);

  if (err)
    return (
      <div className="surface p-5 sm:p-6">
        <p className="mb-3 text-sm text-rose-300">통계 데이터를 불러오지 못했습니다: {err}</p>
        <button className="btn-ghost" onClick={load}>다시 시도</button>
      </div>
    );

  // 선택된 차원의 일별 시리즈 → 색 부여 (카드는 지정색/자동색, 그 외는 팔레트)
  const cur = dimData[dim];
  const days = cur?.days ?? 0;
  const dimSeries: LineSeries[] = (cur?.series ?? []).map((s) => ({
    name: s.name,
    // "기타"는 묶음이라 고유 색이 없다 — 카드 색과 겹치지 않게 중립 회색
    color: s.name === "기타" ? theme.axis : dim === "paymentMethod" ? s.color || autoColor(s.name) : undefined,
    values: s.values,
  }));
  const dayLabels = Array.from({ length: days }, (_, i) => `${i + 1}일`);

  // 카테고리 추이
  const catSeries: LineSeries[] = (catTrend?.series ?? []).map((s) => ({
    name: s.name,
    values: s.values,
  }));
  const catLabels = (catTrend?.months ?? []).map(monthLabel);

  // 귀속자 추이
  const memberSeries: LineSeries[] = (memberTrend?.series ?? []).map((s) => ({
    name: s.name,
    values: s.values,
  }));
  const memberLabels = (memberTrend?.months ?? []).map(monthLabel);

  // 카테고리 상세: 월별 추이 단일 시리즈
  const detailTrend: LineSeries[] = detail
    ? [{ name: detail.category, color: theme.accent, values: detail.trend }]
    : [];
  const detailLabels = (detail?.months ?? []).map(monthLabel);
  // 월 × 항목 표에 실제로 그릴 줄 (총액순으로 이미 정렬돼 온다)
  const catMonthlyRows = catAll
    ? catMonthly?.series ?? []
    : (catMonthly?.series ?? []).slice(0, CAT_ROW_LIMIT);
  // 드릴다운 모달의 추이 데이터. 연 표에 이미 있으니 다시 조회하지 않는다.
  // 하루(day)·상세 막대(detail)는 월별 추이가 없는 목록이라 그래프를 그리지 않는다.
  const drillSrc = drill?.kind === "matrix" ? matrix : drill?.kind === "item" ? catMonthly : null;
  // 상세 막대 한 줄을 눌렀을 때: 그 기간에 그 항목(사람·카드·가맹점·부)이 이 카테고리에 쓴 내역
  const openDetailDrill = (by: CatBy, key: string) =>
    detail && setDrill({ kind: "detail", by, key, ym: month, label: `${detail.category} · ${key} · ${detail.label}` });
  const drillRow = drillSrc?.series.find((s) => s.name === drill?.key) ?? null;
  const drillView =
    drillSrc && drillRow ? { months: drillSrc.months, values: drillRow.values } : null;
  const detailPrevDelta =
    detail && detail.prev ? Math.round(((detail.total - detail.prev) / detail.prev) * 100) : 0;

  return (
    <div className="space-y-4 lg:space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={BarChart3} tone="mint" />
          <div className="min-w-0">
            <h2 className="text-2xl font-light tracking-tight text-white">통계</h2>
            <p className="label-mono num">{month}</p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <MonthPicker value={month} onChange={setMonth} />
          {/* 카테고리 관련 집계(일별·추이·히트맵·귀속자 구성)의 주/부 기준을 한 번에 바꾼다.
              카테고리 상세는 고른 카테고리 하나만 보므로 이 기준과 무관하다. */}
          {view === "overview" && (
            <>
              <span className="label-mono">카테고리 기준</span>
              <Seg<"main" | "sub">
                value={catLevel}
                options={[
                  { key: "main", label: "주" },
                  { key: "sub", label: "부" },
                ]}
                onChange={setCatLevel}
              />
            </>
          )}
        </div>
      </div>

      <StatsNav value={view} onChange={setView} />

      {view === "overview" && (
        <>
          {/* 대표 차트: 이번 달 지금까지를 지난달 이맘때와 견준다 */}
          <div className="surface p-5 sm:p-6">
            <SectionHead
              icon={GitCompare}
              title="전월 동일시점 누적 비교"
              sub="이번 달 지금까지 쓴 돈을 지난달 같은 날까지와 견줍니다"
            />
            {loading || !compare ? <Sk h={420} r={14} /> : <CompareChart data={compare} />}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={Activity} title="일별 · 누적 지출 추이">
              <Seg value={dim} options={DIM_LABEL} onChange={setDim} />
              <Seg<Mode>
                value={mode}
                options={[
                  { key: "daily", label: "일별" },
                  { key: "cumulative", label: "누적" },
                ]}
                onChange={setMode}
              />
            </SectionHead>
            {loading ? (
              <Sk h={240} r={12} />
            ) : (
              <LineChart
                series={dimSeries}
                xLabels={dayLabels}
                cumulative={mode === "cumulative"}
                height={240}
              />
            )}
          </div>

          <div className="grid items-stretch gap-4 lg:grid-cols-2 lg:gap-5">
            {/* 옆 달력과 높이를 맞춘다(카드가 늘어나고 막대 영역이 남는 높이를 채움) */}
            <div className="surface flex h-full flex-col p-5 sm:p-6">
              <SectionHead icon={CalendarDays} title="요일별 평균 지출" sub="지나간 날 기준 하루 평균" />
              <div className="flex min-h-0 flex-1 flex-col">
                {loading || !compare ? <Sk h={280} r={12} /> : <WeekdayPanel data={compare} />}
              </div>
            </div>

            {/* 요일별 평균을 날짜 단위로 펼친 달력. 날짜를 누르면 그날 내역이 열린다 */}
            <div className="surface h-full p-5 sm:p-6">
              <SectionHead icon={Calendar} title="지출 달력" sub="날짜를 누르면 그날 쓴 내역" />
              {loading || !compare ? (
                <Sk h={280} r={12} />
              ) : (
                <SpendCalendar
                  data={compare}
                  onPickDay={(date, label) => setDrill({ kind: "day", key: date, ym: date.slice(0, 7), label })}
                />
              )}
            </div>
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={TrendingUp} title="카테고리별 6개월 추이" />
            {loading ? (
              <Sk h={220} r={12} />
            ) : (
              <LineChart series={catSeries} xLabels={catLabels} />
            )}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={Grid3x3} title="카테고리 × 월 히트맵" />
            {loading || !matrix ? (
              <Sk h={220} r={12} />
            ) : (
              <Heatmap
                rows={matrix.series}
                months={matrix.months}
                onCell={(key, ym) => setDrill({ kind: "matrix", key, ym })}
              />
            )}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={Users} title={`카테고리별 귀속자 구성 (${Number(month.slice(5))}월)`} />
            {loading ? <Sk h={120} r={12} /> : <StackedBars data={catMember} />}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={Users} title={`귀속자별 카테고리 구성 (${Number(month.slice(5))}월)`} />
            {loading ? <Sk h={120} r={12} /> : <StackedBars data={memberCat} />}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={UserRound} title={`귀속자별 지출 요약 (${Number(month.slice(5))}월)`} />
            {loading ? (
              <Sk h={120} r={12} />
            ) : memberStats.length === 0 ? (
              <p className="text-sm text-zinc-500">데이터 없음</p>
            ) : (
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {memberStats.map((s) => {
                  const pct = s.prev ? Math.round(((s.total - s.prev) / s.prev) * 100) : 0;
                  return (
                    <div className="surface-inset flex flex-col gap-2 p-4" key={s.member}>
                      <strong className="text-sm font-medium text-white">
                        {s.member} <span className="num text-xs font-normal text-zinc-500">({s.share}%)</span>
                      </strong>
                      <span className="num text-2xl font-light tracking-tight text-rose-300">{won(s.total)}</span>
                      {s.prev > 0 ? (
                        <PctDelta pct={pct} suffix="전월 대비" />
                      ) : (
                        <span className="text-xs text-zinc-500">전월 데이터 없음</span>
                      )}
                      <span className="num text-xs text-zinc-500">
                        최다 카테고리: {s.topCategory || "-"}
                        {s.topCategoryAmount ? ` · ${won(s.topCategoryAmount)}` : ""}
                      </span>
                      <span className="num text-xs text-zinc-500">
                        최다 가맹점: {s.topMerchant || "-"}
                        {s.topMerchantAmount ? ` · ${won(s.topMerchantAmount)}` : ""}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={TrendingUp} title="귀속자별 6개월 추이" />
            {loading ? <Sk h={220} r={12} /> : <LineChart series={memberSeries} xLabels={memberLabels} />}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={CreditCard} title="카드별 카테고리 구성" />
            {loading ? <Sk h={120} r={12} /> : <StackedBars data={cardCat} />}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead
              icon={Repeat}
              title={`고정비 · 정기결제 (${Number(month.slice(5))}월)`}
              sub={recurring.length > 0 ? `${recurring.length} items` : undefined}
            />
            {loading ? (
              <Sk h={100} r={12} />
            ) : recurring.length === 0 ? (
              <p className="text-sm text-zinc-500">
                학습된 정기 거래가 없습니다. 거래를 분류하면 규칙이 쌓이고 여기에 표시됩니다.
              </p>
            ) : (
              <ul className="divide-y divide-white/[0.05]">
                {recurring.map((r, i) => (
                  <li
                    key={i}
                    className={`flex flex-wrap items-center gap-x-3 gap-y-1 py-2.5 text-sm ${
                      r.seen ? "text-zinc-400" : "text-zinc-100"
                    }`}
                  >
                    <span className="font-medium text-white">{r.label || r.merchant}</span>
                    <span className="min-w-0 flex-1 truncate text-xs text-zinc-500">{r.merchant}</span>
                    {r.seen ? (
                      <span className="badge num bg-emerald-500/10 text-emerald-300">
                        빠짐 · {won(r.seenAmount)}
                        {r.seenDate ? ` (${r.seenDate.slice(5)})` : ""}
                      </span>
                    ) : (
                      <span className="badge num bg-amber-500/10 text-amber-300">
                        예정 · {won(r.amountMin)}~{won(r.amountMax)}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={Gauge} title="카드 실적기간 누적 진행률" />
            {loading ? (
              <Sk h={120} r={12} />
            ) : paces.length === 0 ? (
              <p className="text-sm text-zinc-500">실적 목표가 설정된 카드가 없습니다. 카드 탭에서 실적한도를 입력하세요.</p>
            ) : (
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {paces.map((p) => {
                  const c = p.card.color || autoColor(p.card.name);
                  const remaining = p.target - p.spent;
                  return (
                    <div className="surface-inset flex flex-col gap-2 p-4" key={p.card.id}>
                      <strong className="flex items-center gap-2 text-sm font-medium text-white">
                        <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: c }} />
                        <span className="truncate">
                          {p.card.issuer ? `${p.card.issuer} ` : ""}
                          {p.card.name}
                        </span>
                      </strong>
                      <span className="label-mono num !normal-case">
                        {p.periodStart} ~ {p.periodEnd} · {p.elapsed}/{p.days}일차
                      </span>
                      <PaceSparkline pace={p} />
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="num text-sm text-zinc-200">
                          {won(p.spent)} <span className="text-zinc-500">/ 목표 {won(p.target)}</span>
                        </span>
                        {p.achieved ? (
                          <span className={`${PACE_BADGE} bg-emerald-500/10 text-emerald-300`}>실적 달성 ✓</span>
                        ) : p.onTrack ? (
                          <span className={`${PACE_BADGE} bg-emerald-500/10 text-emerald-300`}>달성 페이스</span>
                        ) : (
                          <span className={`${PACE_BADGE} bg-amber-500/10 text-amber-300`}>미달 우려</span>
                        )}
                      </div>
                      <span className="num text-xs text-zinc-500">
                        {p.achieved
                          ? `목표 ${won(p.spent - p.target)} 초과`
                          : `${won(remaining)} 남음 · 현재 페이스 예상 ${won(p.projected)}`}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <div className="surface p-5 sm:p-6">
            <SectionHead icon={CalendarRange} title={`연간 요약 (${Number(month.slice(0, 4))}년)`} />
            {loading || !year ? (
              <Sk h={220} r={12} />
            ) : (
              <>
                <TrendChart points={year.months} />
                <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-3">
                  <div className="surface-inset flex flex-col gap-2 p-4">
                    <p className="label-mono">수입</p>
                    <p className="num text-2xl font-light tracking-tight text-emerald-300">{won(year.totalIncome)}</p>
                    <Yoy cur={year.totalIncome} prev={year.prevIncome} />
                  </div>
                  <div className="surface-inset flex flex-col gap-2 p-4">
                    <p className="label-mono">지출</p>
                    <p className="num text-2xl font-light tracking-tight text-rose-300">{won(year.totalExpense)}</p>
                    <Yoy cur={year.totalExpense} prev={year.prevExpense} goodWhenDown />
                  </div>
                  <div className="surface-inset flex flex-col gap-2 p-4">
                    <p className="label-mono">투자/저축 이체</p>
                    <p className="num text-2xl font-light tracking-tight text-sky-300">{won(year.totalTransfer)}</p>
                    <Yoy cur={year.totalTransfer} prev={year.prevTransfer} />
                  </div>
                </div>
              </>
            )}
          </div>

          {!loading && year && <Bars title="연간 카테고리별 지출" rows={year.byCategory.slice(0, 10)} />}
        </>
      )}

      {/* 고른 카테고리 하나를 요약·구성·월별 상세까지 한 화면에서 본다 */}
      {view === "category" && (
        <>
          <div className="surface p-5 sm:p-6">
            <SectionHead icon={Layers} title="카테고리 상세 분석">
              <Seg
                value={catPeriod}
                options={[
                  { key: "month", label: "이번 달" },
                  { key: "recent30", label: "최근 30일" },
                  { key: "prev", label: "전월" },
                ]}
                onChange={setCatPeriod}
              />
              {/* 주를 고르면 하위 부 지출까지 합산해서 보여준다 */}
              <select
                className="input-field !w-auto"
                value={catId}
                onChange={(e) => setCatId(Number(e.target.value))}
              >
                {categoryOptions(expenseCats).map((c) => (
                  <option key={c.id} value={c.id}>{c.label}</option>
                ))}
              </select>
            </SectionHead>
            {!detail ? (
              <Sk h={200} r={12} />
            ) : detail.total === 0 && detail.trend.every((v) => v === 0) ? (
              <p className="text-sm text-zinc-500">이 카테고리의 지출 내역이 없습니다.</p>
            ) : (
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div className="surface-inset flex min-w-0 flex-col gap-2 p-4">
                  <p className="label-mono !normal-case">{detail.label} 지출</p>
                  <p className="num text-2xl font-light tracking-tight text-rose-300">{won(detail.total)}</p>
                  <p className="num text-xs text-zinc-500">전체 지출의 {detail.share}%</p>
                </div>
                <div className="surface-inset flex min-w-0 flex-col gap-2 p-4">
                  <p className="label-mono !normal-case">{detail.prevLabel} 대비</p>
                  {detail.prev > 0 ? (
                    <p className="num text-2xl font-light tracking-tight">
                      <PctDelta pct={detailPrevDelta} />
                    </p>
                  ) : (
                    <p className="text-sm text-zinc-500">{detail.prevLabel} 데이터 없음</p>
                  )}
                  <p className="num text-xs text-zinc-500">{detail.prevLabel} {won(detail.prev)}</p>
                </div>
                <div className="surface-inset flex min-w-0 flex-col gap-2 p-4">
                  <p className="label-mono">예산</p>
                  {detail.budget > 0 ? (
                    <>
                      <p
                        className={`num text-2xl font-light tracking-tight ${
                          detail.over ? "text-rose-300" : "text-white"
                        }`}
                      >
                        {detail.budgetPct}%
                      </p>
                      <p className="num text-xs text-zinc-500">{won(detail.total)} / {won(detail.budget)}</p>
                    </>
                  ) : (
                    <p className="text-sm text-zinc-500">
                      {catPeriod === "recent30" ? "기간 예산 없음" : "미설정"}
                    </p>
                  )}
                </div>
              </div>
            )}
          </div>

          {detail && !(detail.total === 0 && detail.trend.every((v) => v === 0)) && (
            <>
              <div className="grid items-stretch gap-4 lg:grid-cols-2 lg:gap-5">
                {/* 주 카테고리면 하위 부 구성부터 보여준다 */}
                {detail.isMain && detail.bySub.length > 0 && (
                  <Bars title="무엇에 (부 카테고리별)" rows={detail.bySub} onPick={(k) => openDetailDrill("sub", k)} />
                )}
                <Bars title="누가 (귀속자별)" rows={detail.byMember} onPick={(k) => openDetailDrill("member", k)} />
                <Bars title="어느 카드로 (결제수단별)" rows={detail.byPayment} onPick={(k) => openDetailDrill("payment", k)} />
                <Bars title="어디서 (TOP 가맹점)" rows={detail.topMerchants} onPick={(k) => openDetailDrill("merchant", k)} />
                <div className="surface h-full p-5 sm:p-6">
                  <SectionHead icon={TrendingUp} title="월별 추이" />
                  <LineChart series={detailTrend} xLabels={detailLabels} height={180} showLegend={false} />
                </div>
              </div>

              {/* 달마다 총액만 보면 왜 늘었는지 모른다. 항목을 쪼개야 그게 보인다. */}
              <div className="surface p-5 sm:p-6">
                <SectionHead icon={Grid3x3} title="월별 상세 — 무엇에 썼나">
                  <Seg value={catBy} options={CAT_BY} onChange={setCatBy} />
                  <select
                    className="input-field !w-auto"
                    value={catMonths}
                    onChange={(e) => setCatMonths(Number(e.target.value))}
                  >
                    {[3, 6, 12, 24].map((n) => (
                      <option key={n} value={n}>최근 {n}개월</option>
                    ))}
                  </select>
                </SectionHead>
                {!catMonthly ? (
                  <Sk h={200} r={12} />
                ) : catMonthly.series.length === 0 ? (
                  <p className="text-sm text-zinc-500">이 기간에 기록된 지출이 없습니다.</p>
                ) : (
                  <>
                    <Heatmap
                      rows={catMonthlyRows}
                      months={catMonthly.months}
                      onCell={(key, ym) => setDrill({ kind: "item", key, ym })}
                    />
                    {catMonthly.series.length > CAT_ROW_LIMIT && (
                      <button className="btn-ghost mt-3 !py-1.5 !text-xs" onClick={() => setCatAll((v) => !v)}>
                        {catAll
                          ? "상위 항목만 보기"
                          : `나머지 ${catMonthly.series.length - CAT_ROW_LIMIT}개 항목 더 보기`}
                      </button>
                    )}
                  </>
                )}
              </div>
            </>
          )}
        </>
      )}

      {/* 한 사람의 소비: 요약·추이·무엇이 늘었나·반복 소비·카드·월별 표 (자체 팝업을 가진다) */}
      {view === "member" && <MemberAnalysis refs={refs} month={month} />}

      {/* 표의 칸 하나를 열어 보는 모달 — 월별 상세 표와 히트맵이 같이 쓴다 */}
      {drill && (drillView || drill.kind === "day" || drill.kind === "detail") && (
        <TxDrillModal
          title={drill.key}
          ym={drill.ym}
          heading={
            drill.kind === "day" ? `${drill.label} 지출` : drill.kind === "detail" ? drill.label : undefined
          }
          months={drillView?.months ?? []}
          values={drillView?.values ?? []}
          txs={drillTx}
          onPickMonth={(ym) => setDrill({ ...drill, ym })}
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
            setBump((v) => v + 1); // 표·요약 다시 계산
            loadDrill(); // 열려 있는 내역도 갱신
          }}
        />
      )}
    </div>
  );
}
