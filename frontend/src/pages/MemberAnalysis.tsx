import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import {
  Bar,
  CartesianGrid,
  Cell,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { CalendarDays, Grid3x3, Repeat, TrendingUp } from "lucide-react";
import {
  GetMemberAnalysis,
  GetMemberHabit,
  GetMemberMonthly,
  GetMemberTx,
} from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import { categoryOptions, Refs, won } from "../lib";
import Bars from "../Bars";
import EditModal from "../EditModal";
import TxDrillModal from "../TxDrillModal";
import { ChartEmpty, ChartTip, DeltaBars, Heatmap, LegendDot, compact, cv, niceMax, useAxisTick } from "../charts";
import { autoColor } from "../PmChip";
import { Sk } from "../Skeleton";
import { useThemeColors } from "../appearance/AppearanceContext";
import { PctDelta, SectionHead, Seg } from "../components/ui/StatsUi";

// 구성원 분석: 한 사람(귀속자)의 소비를 요약·추이·습관으로 본다.
// 총액은 보험·병원비 같은 한 번의 큰 지출에 크게 흔들리므로, 늘었는지 줄었는지는
// 고정비(고정비 표시 카테고리·정기결제)와 큰 지출(건당 20만 원 이상)을 뺀 '생활 소비'로 본다.

type Period = "month" | "recent30" | "prev";
type MatrixBy = "category" | "merchant" | "payment";

const PERIODS: { key: Period; label: string }[] = [
  { key: "month", label: "이번 달" },
  { key: "recent30", label: "최근 30일" },
  { key: "prev", label: "전월" },
];
const MATRIX_BY: { key: MatrixBy; label: string }[] = [
  { key: "category", label: "카테고리" },
  { key: "merchant", label: "가맹점" },
  { key: "payment", label: "결제수단" },
];
const KIND_LABEL: Record<string, string> = { living: "생활 소비", fixed: "고정비", big: "큰 지출" };
const DOW = ["일", "월", "화", "수", "목", "금", "토"];
const WEEK_ORDER = [1, 2, 3, 4, 5, 6, 0]; // 월~일

const monthLabel = (ym: string) => `${Number(ym.slice(5))}월`;
// 건당 금액 축: 만 단위로 줄이면 1만·1만처럼 겹치므로 천 단위로
const perVisit = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(v % 1000 ? 1 : 0)}천` : String(v));

const pctOf = (cur: number, base: number) => (base > 0 ? Math.round(((cur - base) / base) * 100) : 0);
// "YYYY-MM" → 그 달의 첫날·말일
const monthRange = (ym: string) => {
  const [y, m] = ym.split("-").map(Number);
  const last = new Date(y, m, 0).getDate();
  return { from: `${ym}-01`, to: `${ym}-${String(last).padStart(2, "0")}` };
};
// 변화 정도를 말로: ±3% 안은 비슷, 15% 미만은 '소폭'
const changeWord = (pct: number) =>
  Math.abs(pct) < 3 ? "비슷" : `${Math.abs(pct) < 15 ? "소폭 " : ""}${pct > 0 ? "증가" : "감소"}`;
// 월말 예상은 추정이라 ±10% 안은 '비슷'으로 본다
const projWord = (pct: number) => (Math.abs(pct) < 10 ? "비슷" : pct > 0 ? "많음" : "적음");
// 진행 중인 달의 처음 며칠은 표본이 너무 작아 %를 보여 주지 않는다
const EARLY_DAYS = 7;
// 같은 날까지 생활 소비가 평소 범위의 어디쯤인지
const rangePos = (v: number, min: number, max: number) =>
  v < min ? "평소보다 적은" : v > max ? "평소보다 많은" : "평소 범위의";

type Drill = { title: string; from: string; to: string; by: string; key: string; categoryId?: number };

export default function MemberAnalysis({ refs, month }: { refs: Refs; month: string }) {
  const tc = useThemeColors();
  const tick = useAxisTick();
  const [memberId, setMemberId] = useState<number>(refs.members[0]?.id ?? 0);
  const [period, setPeriod] = useState<Period>("month");
  const [a, setA] = useState<store.MemberAnalysis | null>(null);
  const [err, setErr] = useState("");
  const [showAll, setShowAll] = useState(true); // 월별 추이에 고정비·큰 지출까지 쌓을지

  const [habitCat, setHabitCat] = useState(0); // 0 이면 분석이 고른 기본 카테고리
  const [habit, setHabit] = useState<store.MemberHabit | null>(null);
  const [matrixBy, setMatrixBy] = useState<MatrixBy>("category");
  const [matrixMonths, setMatrixMonths] = useState(6);
  const [matrix, setMatrix] = useState<store.CategoryTrend | null>(null);

  const [drill, setDrill] = useState<Drill | null>(null);
  const [drillTx, setDrillTx] = useState<store.Transaction[]>([]);
  const [editTx, setEditTx] = useState<store.Transaction | null>(null);
  const [bump, setBump] = useState(0); // 거래를 고치면 다시 계산

  const [y, m] = month.split("-").map(Number);

  useEffect(() => {
    if (memberId === 0 && refs.members.length > 0) setMemberId(refs.members[0].id);
  }, [memberId, refs.members]);

  useEffect(() => {
    if (memberId === 0) return;
    setA(null);
    GetMemberAnalysis(y, m, memberId, period)
      .then((r) => {
        setA(r);
        setErr("");
      })
      .catch((e) => setErr(String(e)));
  }, [y, m, memberId, period, bump]);

  // 사람을 바꾸면 반복 소비 카테고리는 그 사람의 기본값으로 돌아간다
  useEffect(() => setHabitCat(0), [memberId]);
  const habitId = habitCat || a?.habitCategoryId || 0;
  useEffect(() => {
    if (memberId === 0 || habitId === 0) {
      setHabit(null);
      return;
    }
    GetMemberHabit(y, m, memberId, habitId, period)
      .then(setHabit)
      .catch(() => setHabit(null));
  }, [y, m, memberId, habitId, period, bump]);

  useEffect(() => {
    if (memberId === 0) return;
    GetMemberMonthly(y, m, matrixMonths, memberId, matrixBy)
      .then(setMatrix)
      .catch(() => setMatrix(null));
  }, [y, m, memberId, matrixBy, matrixMonths, bump]);

  const loadDrill = useCallback(async () => {
    if (!drill) return;
    try {
      setDrillTx(
        (await GetMemberTx(memberId, drill.from, drill.to, drill.by, drill.key, drill.categoryId ?? 0)) ?? []
      );
    } catch {
      setDrillTx([]);
    }
  }, [drill, memberId]);
  useEffect(() => {
    setDrillTx([]);
    loadDrill();
  }, [loadDrill]);

  const who = a?.member ?? refs.members.find((x) => x.id === memberId)?.name ?? "";
  const openPeriod = (by: string, key: string, label: string, categoryId?: number) =>
    a && setDrill({ title: `${who} · ${label} · ${a.label}`, from: a.from, to: a.to, by, key, categoryId });
  const openMonth = (ym: string, by: string, key: string, label: string) => {
    const r = monthRange(ym);
    setDrill({ title: `${who} · ${label} · ${monthLabel(ym)}`, ...r, by, key });
  };

  const pmColors = useMemo(
    () => Object.fromEntries(refs.paymentMethods.map((p) => [p.name, p.color || autoColor(p.name)])),
    [refs.paymentMethods]
  );
  const expenseCats = refs.categories.filter((c) => c.kind === "expense");

  if (refs.members.length === 0)
    return <div className="surface p-5 text-sm text-zinc-500 sm:p-6">설정에서 귀속자(가족 구성원)를 먼저 추가하세요.</div>;
  if (err)
    return <div className="surface p-5 text-sm text-rose-300 sm:p-6">구성원 분석을 불러오지 못했습니다: {err}</div>;

  return (
    <>
      <SummaryCard
        a={a}
        memberId={memberId}
        setMemberId={setMemberId}
        period={period}
        setPeriod={setPeriod}
        habit={habit}
      />

      <div className="surface p-5 sm:p-6">
        <SectionHead
          icon={TrendingUp}
          title="월별 추이"
          sub="막대를 누르면 그 달 그 종류의 내역 · 점선은 생활 소비"
        >
          <label className="flex cursor-pointer items-center gap-2 text-xs text-zinc-300">
            <input type="checkbox" checked={showAll} onChange={(e) => setShowAll(e.target.checked)} />
            고정비·큰 지출도 쌓기
          </label>
        </SectionHead>
        {!a ? (
          <Sk h={260} r={12} />
        ) : (
          <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
            <MonthlyChart
              months={a.months}
              showAll={showAll}
              onPick={(ym, kind) => openMonth(ym, "kind", kind, KIND_LABEL[kind])}
              tc={tc}
              tick={tick}
            />
            <TrendPanel a={a} onBig={() => openPeriod("kind", "big", "큰 지출")} />
          </div>
        )}
      </div>

      <div className="grid items-stretch gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)] lg:gap-5">
        <div className="surface p-5 sm:p-6">
          <SectionHead
            icon={TrendingUp}
            title="무엇이 늘었나"
            sub={
              !a
                ? undefined
                : a.avgWindows === 0
                ? `${a.label} · 비교 기록 없음 · 누르면 내역`
                : a.inProgress
                ? `${a.label} ${a.elapsedDays}일까지 ↔ 최근 ${a.avgWindows}개월 같은 날까지 평균${
                    a.elapsedDays < EARLY_DAYS ? " · 아직 초반이라 참고용" : ""
                  }`
                : `${a.label} ↔ 최근 ${a.avgWindows}개월 평균 · 누르면 내역`
            }
          />
          {!a ? (
            <Sk h={200} r={12} />
          ) : (
            <DeltaBars
              rows={a.catChanges.map(
                (c) =>
                  new store.CategoryDelta({
                    name: c.name,
                    current: c.current,
                    prev: c.avg,
                    delta: c.current - c.avg,
                    pct: pctOf(c.current, c.avg),
                    isNew: c.avg === 0 && c.current > 0,
                    gone: c.current === 0 && c.avg > 0,
                  })
              )}
              onPick={(name) => openPeriod("category", name, name)}
            />
          )}
        </div>
        <div className="flex flex-col">
          {!a ? (
            <div className="surface p-5 sm:p-6">
              <Sk h={200} r={12} />
            </div>
          ) : (
            <>
              <Bars
                title="어느 카드로 (결제수단별)"
                rows={a.byPayment.map((p) => new store.NamedAmount({ name: p.name, kind: "", amount: p.amount }))}
                colors={pmColors}
                onPick={(name) => openPeriod("payment", name, name)}
              />
              <CashNote a={a} />
            </>
          )}
        </div>
      </div>

      <div className="surface p-5 sm:p-6">
        <SectionHead icon={Repeat} title="반복 소비 — 자주 쓰는 곳과 습관" sub="고른 카테고리를 얼마나 자주, 어디서, 얼마에 쓰는지">
          <select
            className="input-field !w-auto"
            value={habitId}
            onChange={(e) => setHabitCat(Number(e.target.value))}
          >
            {habitId === 0 && <option value={0}>카테고리 선택</option>}
            {categoryOptions(expenseCats).map((c) => (
              <option key={c.id} value={c.id}>
                {c.label}
              </option>
            ))}
          </select>
        </SectionHead>
        {habitId === 0 ? (
          <p className="text-sm text-zinc-500">최근 90일 기록이 없습니다. 카테고리를 골라 보세요.</p>
        ) : !habit ? (
          <Sk h={240} r={12} />
        ) : (
          <HabitView
            habit={habit}
            periodLabel={a?.label ?? ""}
            onPlace={(name) => openPeriod("merchant", name, name, habit.categoryId)}
            tc={tc}
            tick={tick}
          />
        )}
      </div>

      <div className="surface p-5 sm:p-6">
        <SectionHead icon={Grid3x3} title="월 × 항목" sub="칸을 누르면 그 달 내역">
          <Seg value={matrixBy} options={MATRIX_BY} onChange={setMatrixBy} />
          <select
            className="input-field !w-auto"
            value={matrixMonths}
            onChange={(e) => setMatrixMonths(Number(e.target.value))}
          >
            {[6, 12].map((n) => (
              <option key={n} value={n}>
                최근 {n}개월
              </option>
            ))}
          </select>
        </SectionHead>
        {!matrix ? (
          <Sk h={200} r={12} />
        ) : matrix.series.length === 0 ? (
          <p className="text-sm text-zinc-500">이 기간에 기록된 지출이 없습니다.</p>
        ) : (
          <Heatmap
            rows={matrix.series}
            months={matrix.months}
            onCell={(name, ym) => openMonth(ym, matrixBy, name, name)}
          />
        )}
      </div>

      {drill && (
        <TxDrillModal
          title={drill.key}
          ym={drill.from.slice(0, 7)}
          heading={drill.title}
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
            setBump((v) => v + 1);
            loadDrill();
          }}
        />
      )}
    </>
  );
}

// ---- ① 사람·기간 선택 + 요약 ----

function SummaryCard({
  a,
  memberId,
  setMemberId,
  period,
  setPeriod,
  habit,
}: {
  a: store.MemberAnalysis | null;
  memberId: number;
  setMemberId: (id: number) => void;
  period: Period;
  setPeriod: (p: Period) => void;
  habit: store.MemberHabit | null;
}) {
  const tc = useThemeColors();
  const insights = a ? buildInsights(a, habit) : [];
  // 구성원 색: 이름 해시는 겹칠 수 있어 테마의 구분 팔레트를 순서대로 쓴다
  const memberColor = (i: number) => tc.palette[i % tc.palette.length];
  return (
    <div className="surface p-5 sm:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-2" role="group" aria-label="구성원">
          {(a?.members ?? []).map((mc, i) => {
            const on = mc.id === memberId;
            return (
              <button
                key={mc.id}
                type="button"
                onClick={() => setMemberId(mc.id)}
                aria-pressed={on}
                className={`flex items-center gap-2.5 rounded-2xl border px-3.5 py-2 text-left transition-colors ${
                  on ? "border-mint-400/50 bg-mint-400/[0.08]" : "border-white/[0.08] hover:bg-white/[0.04]"
                }`}
              >
                <i className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: memberColor(i) }} />
                <span>
                  <span className="block text-sm text-zinc-100">{mc.name}</span>
                  <span className="num block text-[11px] text-zinc-500">{compact(mc.total)}</span>
                </span>
              </button>
            );
          })}
          {!a && <Sk h={44} w={320} r={14} />}
        </div>
        <Seg value={period} options={PERIODS} onChange={setPeriod} />
      </div>

      {!a ? (
        <div className="mt-4">
          <Sk h={110} r={14} />
        </div>
      ) : (
        <>
          {a.inProgress ? (
            <InProgressTiles a={a} />
          ) : (
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Tile label={`${a.label} 지출`}>
              <p className="num text-2xl font-light tracking-tight text-rose-300">{won(a.total)}</p>
              <p className="num text-xs text-zinc-500">
                가족 전체 지출의 {a.familyTotal > 0 ? Math.round((a.total / a.familyTotal) * 100) : 0}% · {a.txCount}건
              </p>
            </Tile>
            <Tile label={`${a.prevLabel} 대비`}>
              {a.prev > 0 ? (
                <p className="num text-2xl font-light tracking-tight">
                  <PctDelta pct={pctOf(a.total, a.prev)} />
                </p>
              ) : (
                <p className="text-sm text-zinc-500">비교할 지출 없음</p>
              )}
              <p className="num text-xs text-zinc-500">
                {a.prevLabel} {won(a.prev)}
              </p>
            </Tile>
            <Tile label={`최근 ${a.avgWindows || 3}개월 평균 대비`}>
              {a.avgWindows > 0 && a.avg3 > 0 ? (
                <>
                  <p className="num text-2xl font-light tracking-tight">
                    <PctDelta pct={pctOf(a.total, a.avg3)} suffix="총액" />
                  </p>
                  <p className="num text-xs text-zinc-500">
                    생활 소비만 보면 <PctDelta pct={pctOf(a.living, a.livingAvg3)} /> ({compact(a.living)} / 평균{" "}
                    {compact(a.livingAvg3)})
                  </p>
                </>
              ) : (
                <p className="text-sm text-zinc-500">비교할 기록이 아직 없습니다</p>
              )}
            </Tile>
            <Tile label="건당 평균">
              <p className="num text-2xl font-light tracking-tight text-white">
                {won(a.txCount ? a.total / a.txCount : 0)}
              </p>
              <p className="num text-xs text-zinc-500">
                생활 소비 {a.livingCount}건 · 건당 {won(a.livingCount ? a.living / a.livingCount : 0)}
              </p>
            </Tile>
          </div>
          )}
          {insights.length > 0 && (
            <div className="surface-inset mt-3 px-4 py-3">
              <p className="label-mono !normal-case">요약</p>
              <ul className="mt-1.5 space-y-1 text-[13px] leading-relaxed text-zinc-300">
                {insights.map((t, i) => (
                  <li key={i}>{t}</li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}
    </div>
  );
}

// 진행 중인 달의 요약: 며칠치로 %를 내면 한 건에 크게 흔들리므로
// 지금까지 금액 + 월말 예상(지난 3개월과 견줌) + 같은 날까지 생활 소비의 평소 범위로 보여 준다.
function InProgressTiles({ a }: { a: store.MemberAnalysis }) {
  const early = a.elapsedDays < EARLY_DAYS;
  const projPct = pctOf(a.projTotal, a.fullAvg3);
  const livingPct = pctOf(a.projLiving, a.fullLivingAvg3);
  const hasRange = a.avgWindows > 0;
  const pos = rangePos(a.living, a.rangeLivingMin, a.rangeLivingMax);
  return (
    <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <Tile label={`${a.label} 지출 · ${a.elapsedDays}일째 / ${a.periodDays}일`}>
        <p className="num text-2xl font-light tracking-tight text-rose-300">{won(a.total)}</p>
        <p className="num text-xs text-zinc-500">
          가족 전체 지출의 {a.familyTotal > 0 ? Math.round((a.total / a.familyTotal) * 100) : 0}% · {a.txCount}건
          {early && <span className="badge ml-1.5 bg-amber-500/10 text-amber-300">초반 · 참고용</span>}
        </p>
      </Tile>
      <Tile label="월말 예상">
        <p className="num text-2xl font-light tracking-tight text-white">약 {won(a.projTotal)}</p>
        <p className="num text-xs text-zinc-500">
          생활 {compact(a.projLiving)} + 고정비 {compact(a.projFixed)}
          {a.projBig > 0 && ` + 큰 지출 ${compact(a.projBig)}`}
        </p>
        {a.fullAvg3 > 0 && (
          <p className="num text-xs text-zinc-400">
            지난 3개월 평균 {compact(a.fullAvg3)}
            {Math.abs(projPct) < 10 ? "과 " : "보다 "}
            <span className={Math.abs(projPct) < 10 ? "text-zinc-200" : projPct > 0 ? "text-rose-300" : "text-emerald-300"}>
              {projWord(projPct)}
            </span>
            {!early && Math.abs(projPct) >= 10 && <> <PctDelta pct={projPct} /></>}
          </p>
        )}
      </Tile>
      <Tile label={`생활 소비 · ${a.elapsedDays}일까지`}>
        <p className="num text-2xl font-light tracking-tight text-white">{won(a.living)}</p>
        {hasRange ? (
          <p className="num text-xs text-zinc-500">
            평소 같은 날까지 {compact(a.rangeLivingMin)}~{compact(a.rangeLivingMax)} →{" "}
            <span className="text-zinc-200">{pos === "평소 범위의" ? "평소 범위" : `${pos} 편`}</span>
          </p>
        ) : (
          <p className="text-xs text-zinc-500">비교할 기록이 아직 없습니다</p>
        )}
        {a.fullLivingAvg3 > 0 && (
          <p className="num text-xs text-zinc-400">
            월말 예상 {compact(a.projLiving)} · 평소 {compact(a.fullLivingAvg3)}
            {!early && Math.abs(livingPct) >= 10 && <> <PctDelta pct={livingPct} /></>}
          </p>
        )}
      </Tile>
      <Tile label="건당 평균">
        <p className="num text-2xl font-light tracking-tight text-white">{won(a.txCount ? a.total / a.txCount : 0)}</p>
        <p className="num text-xs text-zinc-500">
          생활 소비 {a.livingCount}건 · 건당 {won(a.livingCount ? a.living / a.livingCount : 0)}
        </p>
      </Tile>
    </div>
  );
}

function Tile({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="surface-inset flex min-w-0 flex-col gap-2 p-4">
      <p className="label-mono !normal-case">{label}</p>
      {children}
    </div>
  );
}

// 요약 아래 한 줄 설명들: 총액과 생활 소비가 어떻게 다른지, 가장 크게 늘어난 것, 반복 소비 변화
function buildInsights(a: store.MemberAnalysis, habit: store.MemberHabit | null): string[] {
  if (a.inProgress) return buildProgressInsights(a);
  const out: string[] = [];
  if (a.avgWindows > 0 && a.avg3 > 0) {
    const tp = pctOf(a.total, a.avg3);
    const lp = pctOf(a.living, a.livingAvg3);
    const totalPart =
      Math.abs(tp) < 3 ? "총액은 최근 평균과 비슷하고" : `총액은 최근 평균보다 ${Math.abs(tp)}% ${tp > 0 ? "많고" : "적고"}`;
    const livingPart =
      a.livingAvg3 === 0
        ? "생활 소비는 비교할 기록이 없습니다"
        : Math.abs(lp) < 3
        ? "생활 소비는 평소와 비슷합니다"
        : `생활 소비는 ${Math.abs(lp)}% ${lp > 0 ? "늘었습니다" : "줄었습니다"}`;
    out.push(`${totalPart}, 고정비와 20만 원 이상 큰 지출을 빼면 ${livingPart}.`);
  }
  const up = a.catChanges.filter((c) => c.current > c.avg).sort((x, y) => y.current - y.avg - (x.current - x.avg))[0];
  if (up && a.avgWindows > 0) out.push(`가장 크게 늘어난 것은 ${up.name} +${won(up.current - up.avg)}입니다.`);
  if (habit) {
    const done = habit.months.filter((x) => !x.partial);
    const [p, c] = done.slice(-2);
    if (p && c && p.count > 0 && c.count > 0) {
      const name = habit.category.split(" > ").pop();
      const diff = c.count - p.count;
      out.push(
        `${name}은 ${monthLabel(p.month)}보다 ${diff === 0 ? "같은 횟수" : `${Math.abs(diff)}회 ${diff > 0 ? "더" : "덜"}`}(${
          c.count
        }회) 썼고, 건당 ${won(c.total / c.count)}입니다.`
      );
    }
  }
  return out;
}

// 진행 중인 달의 설명: 단정 대신 "이대로면 ~", "~한 페이스"로 말한다
function buildProgressInsights(a: store.MemberAnalysis): string[] {
  const out: string[] = [];
  if (a.elapsedDays < EARLY_DAYS)
    out.push(`아직 ${a.elapsedDays}일째라 한두 건에도 숫자가 크게 움직입니다. 아래는 참고용입니다.`);
  if (a.fullAvg3 > 0) {
    const pct = pctOf(a.projTotal, a.fullAvg3);
    const word = Math.abs(pct) < 10 ? "비슷한" : pct > 0 ? `${Math.abs(pct)}% 많은` : `${Math.abs(pct)}% 적은`;
    out.push(`이대로면 ${a.label}은 약 ${won(a.projTotal)}으로, 지난 3개월 평균(${compact(a.fullAvg3)})과 ${word} 수준이 예상됩니다.`);
  }
  if (a.avgWindows > 0) {
    out.push(
      `생활 소비는 ${a.elapsedDays}일까지 ${compact(a.living)}으로 ${rangePos(a.living, a.rangeLivingMin, a.rangeLivingMax)} 페이스입니다 (평소 같은 날까지 ${compact(
        a.rangeLivingMin
      )}~${compact(a.rangeLivingMax)}).`
    );
  }
  const big = a.bigItems[0];
  if (big) {
    out.push(
      `큰 지출 ${big.merchant || "(내용 없음)"} ${won(big.amount)}${
        a.bigItems.length > 1 ? ` 등 ${a.bigItems.length}건` : ""
      }은 남은 날로 늘려 잡지 않고 그대로만 넣었습니다.`
    );
  }
  return out;
}

// ---- ② 월별 추이 ----

function MonthlyChart({
  months,
  showAll,
  onPick,
  tc,
  tick,
}: {
  months: store.MemberMonth[];
  showAll: boolean;
  onPick: (ym: string, kind: string) => void;
  tc: ReturnType<typeof useThemeColors>;
  tick: ReturnType<typeof useAxisTick>;
}) {
  const C = { living: tc.accent, fixed: cv("sky-400"), big: cv("amber-400") };
  const data = months.map((x) => {
    const total = x.living + x.fixed + x.big;
    // 진행 중인 달: 월말 예상에서 지금까지를 뺀 '남은 예상분'을 위에 점선으로 얹는다
    const rest = x.partial ? Math.max(0, showAll ? x.projTotal - total : x.projLiving - x.living) : 0;
    return {
      ...x,
      label: monthLabel(x.month) + (x.partial ? "*" : ""),
      total,
      rest,
      livingLine: x.partial && x.projLiving > 0 ? x.projLiving : x.living,
    };
  });
  const hasProj = data.some((d) => d.rest > 0);
  if (data.every((d) => d.total === 0)) return <ChartEmpty>이 기간에 기록된 지출이 없습니다.</ChartEmpty>;
  const keys = (showAll ? ["living", "fixed", "big"] : ["living"]) as ("living" | "fixed" | "big")[];
  return (
    <div>
      <div className="mb-2 flex flex-wrap gap-4 text-xs text-zinc-400">
        {keys.map((k) => (
          <span key={k} className="inline-flex items-center gap-1.5">
            <LegendDot color={C[k]} />
            {KIND_LABEL[k]}
            {k === "fixed" && " (고정비 카테고리·정기결제)"}
            {k === "big" && " (건당 20만↑)"}
          </span>
        ))}
        <span className="inline-flex items-center gap-1.5">
          <LegendDot color={cv("zinc-200")} dashed />
          생활 소비 흐름
        </span>
        {hasProj && (
          <span className="inline-flex items-center gap-1.5">
            <i className="inline-block h-2.5 w-2.5 rounded-[3px] border border-dashed border-zinc-300" />
            월말 예상 (남은 날)
          </span>
        )}
      </div>
      <div style={{ height: 240 }}>
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data} margin={{ top: 16, right: 8, left: 0, bottom: 0 }}>
            <CartesianGrid stroke={tc.grid} strokeOpacity={0.4} vertical={false} />
            <XAxis dataKey="label" tick={tick} axisLine={{ stroke: tc.grid }} tickLine={false} />
            <YAxis tick={tick} tickFormatter={compact} axisLine={false} tickLine={false} width={48} />
            <Tooltip
              cursor={{ fill: "rgba(255,255,255,0.04)" }}
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                const d = payload[0].payload as (typeof data)[number];
                const rows = (["living", "fixed", "big"] as const).map((k) => ({
                  color: C[k],
                  value: won(d[k]),
                  name: KIND_LABEL[k],
                }));
                if (d.partial && d.projTotal > 0)
                  rows.push({ color: cv("zinc-300"), value: `약 ${won(d.projTotal)}`, name: "월말 예상 (추정)" });
                return <ChartTip head={`${d.label} · ${d.partial ? "지금까지" : "합계"} ${won(d.total)}`} rows={rows} />;
              }}
            />
            {keys.map((k, i) => (
              <Bar
                key={k}
                dataKey={k}
                stackId="s"
                fill={C[k]}
                radius={i === keys.length - 1 && !hasProj ? [4, 4, 0, 0] : 0}
                cursor="pointer"
                onClick={(d: any) => d?.payload && onPick(d.payload.month, k)}
                maxBarSize={48}
                isAnimationActive={false}
              />
            ))}
            {hasProj && (
              <Bar
                dataKey="rest"
                stackId="s"
                fill={cv("zinc-300", 0.06)}
                stroke={cv("zinc-300", 0.7)}
                strokeDasharray="4 3"
                radius={[4, 4, 0, 0]}
                maxBarSize={48}
                isAnimationActive={false}
              />
            )}
            <Line
              dataKey="livingLine"
              stroke={cv("zinc-200")}
              strokeDasharray="4 4"
              strokeWidth={1.5}
              dot={{ r: 3, fill: tc.grid, strokeWidth: 1.5 }}
              isAnimationActive={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
      {data.some((d) => d.partial) && (
        <p className="mt-1 text-[11px] text-zinc-500">
          * 진행 중인 달 — 진한 막대는 오늘까지, 점선은 월말 예상분(큰 지출은 늘려 잡지 않음)
        </p>
      )}
    </div>
  );
}

function TrendPanel({ a, onBig }: { a: store.MemberAnalysis; onBig: () => void }) {
  const pct = pctOf(a.recentAvg, a.priorAvg);
  return (
    <div className="surface-inset space-y-3 p-4">
      <div>
        <p className="label-mono !normal-case">생활 소비 흐름 (끝난 달 기준)</p>
        {a.priorAvg > 0 && a.recentLabel ? (
          <>
            <p className="mt-2 text-lg font-light text-white">
              생활 소비{" "}
              <span className={Math.abs(pct) < 3 ? "text-zinc-200" : pct > 0 ? "text-rose-300" : "text-emerald-300"}>
                {changeWord(pct)}
              </span>{" "}
              <PctDelta pct={pct} />
            </p>
            <p className="num mt-1 text-xs leading-relaxed text-zinc-400">
              {a.recentLabel} 평균 {compact(a.recentAvg)}
              <br />
              {a.priorLabel} 평균 {compact(a.priorAvg)}
            </p>
          </>
        ) : (
          <p className="mt-2 text-sm text-zinc-500">끝난 달이 4개월 이상 쌓이면 흐름을 판정합니다.</p>
        )}
      </div>
      <div className="h-px bg-white/[0.07]" />
      <div>
        <p className="label-mono !normal-case">{a.label} 큰 지출 (건당 20만↑)</p>
        {a.bigItems.length === 0 ? (
          <p className="mt-2 text-xs text-zinc-500">없음</p>
        ) : (
          <button type="button" onClick={onBig} className="mt-1.5 w-full space-y-1 text-left" title="큰 지출 내역 보기">
            {a.bigItems.map((b) => (
              <span key={b.id} className="flex items-center justify-between gap-2 rounded-md px-1 text-xs hover:bg-white/[0.04]">
                <span className="min-w-0 truncate text-zinc-300">
                  {b.merchant || "(내용 없음)"} <span className="text-zinc-500">· {b.category}</span>
                </span>
                <span className="num shrink-0 text-amber-300">{won(b.amount)}</span>
              </span>
            ))}
          </button>
        )}
      </div>
    </div>
  );
}

// ---- ⑤ 결제수단 보조 설명 ----

function CashNote({ a }: { a: store.MemberAnalysis }) {
  const cash = a.byPayment.find((p) => p.name === "현금");
  const share = cash && a.total > 0 ? Math.round((cash.amount / a.total) * 100) : 0;
  if (share < 30) return null;
  return (
    <p className="surface-inset mt-3 px-4 py-3 text-xs leading-relaxed text-zinc-300">
      현금이 {share}%입니다. 카드 실적이 모자란 달에는 현금 결제를 카드로 옮기면 실적을 채우기 쉽습니다.
    </p>
  );
}

// ---- ④ 반복 소비 ----

function HabitView({
  habit,
  periodLabel,
  onPlace,
  tc,
  tick,
}: {
  habit: store.MemberHabit;
  periodLabel: string;
  onPlace: (name: string) => void;
  tc: ReturnType<typeof useThemeColors>;
  tick: ReturnType<typeof useAxisTick>;
}) {
  const data = habit.months.map((x) => ({
    ...x,
    label: monthLabel(x.month) + (x.partial ? "*" : ""),
    avg: x.count > 0 ? Math.round(x.total / x.count) : null,
  }));
  // 끝난 두 달의 변화를 '횟수 영향'과 '건당 금액 영향'으로 나눈다 (합 = 합계 차이).
  // 진행 중인 달은 며칠치라 한 달 전체와 견주지 않고, 아래에 '같은 날까지 횟수'로 따로 보여 준다.
  const [p, c] = habit.months.filter((x) => !x.partial).slice(-2);
  const cur = habit.months[habit.months.length - 1];
  const decomp =
    p && c && p.count > 0 && c.count > 0
      ? (() => {
          const pAvg = p.total / p.count;
          const cAvg = c.total / c.count;
          const countEffect = Math.round((c.count - p.count) * pAvg);
          return { diff: c.total - p.total, countEffect, priceEffect: c.total - p.total - countEffect, pAvg, cAvg };
        })()
      : null;
  const totalVisits = habit.weekday.reduce((s, v) => s + v, 0);
  const maxDay = Math.max(1, ...habit.weekday);
  // 평일 중 한 번도 없는 요일 (기록이 충분할 때만 말한다)
  const missing =
    totalVisits >= 10 ? [1, 2, 3, 4, 5].filter((d) => habit.weekday[d] === 0).map((d) => DOW[d]) : [];

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,1.2fr)_minmax(0,0.8fr)]">
      <div className="surface-inset p-4">
        <div className="mb-2 flex items-center justify-between text-xs text-zinc-400">
          <span className="label-mono !normal-case">월별 횟수 × 건당 금액</span>
          <span className="flex gap-3">
            <span className="inline-flex items-center gap-1.5">
              <LegendDot color={tc.accent} />
              횟수
            </span>
            <span className="inline-flex items-center gap-1.5">
              <LegendDot color={cv("amber-400")} />
              건당
            </span>
          </span>
        </div>
        <div style={{ height: 200 }}>
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={data} margin={{ top: 18, right: 4, left: 0, bottom: 0 }}>
              <CartesianGrid stroke={tc.grid} strokeOpacity={0.4} vertical={false} />
              <XAxis dataKey="label" tick={tick} axisLine={{ stroke: tc.grid }} tickLine={false} />
              <YAxis yAxisId="n" tick={tick} axisLine={false} tickLine={false} width={30} allowDecimals={false}
                domain={[0, (max: number) => niceMax(Math.max(4, max * 2))]} />
              <YAxis yAxisId="p" orientation="right" tick={tick} axisLine={false} tickLine={false} width={40}
                tickFormatter={perVisit} domain={[0, (max: number) => niceMax(max * 1.25)]} />
              <Tooltip
                cursor={{ fill: "rgba(255,255,255,0.04)" }}
                content={({ active, payload }) => {
                  if (!active || !payload?.length) return null;
                  const d = payload[0].payload as (typeof data)[number];
                  return (
                    <ChartTip
                      head={d.label}
                      rows={[
                        { color: tc.accent, value: `${d.count}회`, name: `합계 ${won(d.total)}` },
                        { color: cv("amber-400"), value: d.avg ? won(d.avg) : "-", name: "건당" },
                      ]}
                    />
                  );
                }}
              />
              <Bar yAxisId="n" dataKey="count" fill={tc.accent} radius={[4, 4, 0, 0]} maxBarSize={36} isAnimationActive={false}>
                {data.map((d) => (
                  <Cell key={d.month} fillOpacity={d.partial ? 0.4 : 1} />
                ))}
              </Bar>
              <Line yAxisId="p" dataKey="avg" stroke={cv("amber-400")} strokeWidth={2} connectNulls
                dot={{ r: 3, fill: tc.grid, strokeWidth: 2 }} isAnimationActive={false} />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
        {decomp && (
          <div className="mt-2 space-y-1 text-xs">
            <p className="label-mono !normal-case">
              {monthLabel(p.month)} → {monthLabel(c.month)} 변화 나눠 보기
            </p>
            <Row k={`합계 (${compact(p.total)} → ${compact(c.total)})`} v={decomp.diff} />
            <Row k={`횟수 영향 (${p.count} → ${c.count}회)`} v={decomp.countEffect} />
            <Row k={`건당 금액 영향 (${won(decomp.pAvg)} → ${won(decomp.cAvg)})`} v={decomp.priceEffect} />
          </div>
        )}
        {habit.inProgress && cur?.partial && (
          <p className="mt-2 rounded-lg bg-white/[0.03] px-2.5 py-1.5 text-xs text-zinc-300">
            {monthLabel(cur.month)} {habit.elapsedDays}일까지 <strong className="num">{habit.sameDayCount}회</strong>
            <span className="text-zinc-500"> · 지난달 같은 날까지 {habit.sameDayPrevCount}회</span>
          </p>
        )}
      </div>

      <div className="surface-inset p-4">
        <div className="mb-2 flex items-center justify-between">
          <span className="label-mono !normal-case">{periodLabel} 자주 간 곳</span>
          <span className="text-[11px] text-zinc-500">누르면 그 가게 내역</span>
        </div>
        {habit.places.length === 0 ? (
          <ChartEmpty>이 기간 기록이 없습니다.</ChartEmpty>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="table-head border-b border-white/[0.08]">
                <th className="py-2 pr-2 text-left font-normal">가게</th>
                <th className="px-2 py-2 text-right font-normal">횟수</th>
                <th className="px-2 py-2 text-right font-normal">합계</th>
                <th className="px-2 py-2 text-right font-normal">건당</th>
                <th className="py-2 pl-2 text-right font-normal">마지막</th>
              </tr>
            </thead>
            <tbody>
              {habit.places.map((pl) => (
                <tr
                  key={pl.name}
                  className="table-row cursor-pointer text-zinc-200"
                  onClick={() => onPlace(pl.name)}
                  title={`${pl.name} 내역 보기`}
                >
                  <td className="max-w-[180px] truncate py-2 pr-2">
                    {pl.name}
                    {pl.new && <span className="badge ml-1.5 bg-sky-500/10 !text-[10px] text-sky-300">NEW</span>}
                  </td>
                  <td className="num px-2 py-2 text-right">{pl.count}회</td>
                  <td className="num px-2 py-2 text-right">{pl.total.toLocaleString("ko-KR")}</td>
                  <td className="num px-2 py-2 text-right">{Math.round(pl.total / pl.count).toLocaleString("ko-KR")}</td>
                  <td className="num py-2 pl-2 text-right text-zinc-500">
                    {Number(pl.last.slice(5, 7))}/{Number(pl.last.slice(8))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <p className="mt-2 text-[11px] text-zinc-500">NEW = 이 기간에 처음 간 곳</p>
      </div>

      <div className="surface-inset p-4">
        <p className="label-mono !normal-case flex items-center gap-1.5">
          <CalendarDays className="h-3.5 w-3.5" />
          요일별 횟수 ({monthLabel(habit.weekdayFrom.slice(0, 7))}~{monthLabel(habit.weekdayTo.slice(0, 7))} {totalVisits}회)
        </p>
        <div className="mt-3 flex h-[140px] items-end gap-2">
          {WEEK_ORDER.map((d) => {
            const n = habit.weekday[d];
            const zeroWeekday = n === 0 && missing.includes(DOW[d]);
            return (
              <div key={d} className="flex h-full flex-1 flex-col items-center gap-1">
                <span className={`num text-[11px] ${zeroWeekday ? "text-rose-300" : "text-zinc-300"}`}>{n}</span>
                <div className="flex w-full flex-1 items-end">
                  <div
                    className="w-full rounded-md"
                    style={{
                      height: `${Math.max(2, (n / maxDay) * 100)}%`,
                      background: zeroWeekday ? cv("rose-400", 0.5) : d === 0 || d === 6 ? cv("zinc-500", 0.6) : tc.accent,
                    }}
                  />
                </div>
                <span className={`text-[11px] ${zeroWeekday ? "text-rose-300" : "text-zinc-400"}`}>{DOW[d]}</span>
              </div>
            );
          })}
        </div>
        {missing.length > 0 && (
          <p className="mt-3 text-xs leading-relaxed text-zinc-400">
            {missing.join("·")}요일 기록이 한 번도 없습니다. 그날은 이 카테고리를 쓰지 않거나 다른 카테고리로 분류됐을 수 있습니다.
          </p>
        )}
      </div>
    </div>
  );
}

function Row({ k, v }: { k: string; v: number }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-zinc-400">{k}</span>
      <span className={`num ${v > 0 ? "text-rose-300" : v < 0 ? "text-emerald-300" : "text-zinc-300"}`}>
        {v > 0 ? "+" : v < 0 ? "−" : ""}
        {won(Math.abs(v))}
      </span>
    </div>
  );
}
