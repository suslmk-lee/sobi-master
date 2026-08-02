import { useCallback, useEffect, useState } from "react";
import {
  GetCardCategoryBreakdown,
  GetCardPaces,
  GetCategoryDetail,
  GetCategoryMatrix,
  GetCategoryMemberBreakdown,
  GetCategoryTrend,
  GetCumulativeCompare,
  GetDailyByDimension,
  GetMemberCategoryBreakdown,
  GetMemberStats,
  GetMemberTrend,
  GetRecurringStatus,
  GetWeekdayAverages,
  GetYearSummary,
} from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import { Refs, won } from "../lib";
import Bars from "../Bars";
import { Heatmap, LineChart, LineSeries, PaceSparkline, StackedBars, TrendChart } from "../charts";
import { autoColor } from "../PmChip";
import { Sk } from "../Skeleton";

type Dim = "paymentMethod" | "member" | "category";
type Mode = "daily" | "cumulative";

const DIM_LABEL: { key: Dim; label: string }[] = [
  { key: "paymentMethod", label: "카드/결제수단" },
  { key: "member", label: "귀속자" },
  { key: "category", label: "카테고리" },
];

const WD = ["일", "월", "화", "수", "목", "금", "토"];

// "2026-03" → "3월"
const monthLabel = (ym: string) => `${Number(ym.slice(5))}월`;

// 전년 대비 증감 배지
function Yoy({ cur, prev, goodWhenDown }: { cur: number; prev: number; goodWhenDown?: boolean }) {
  if (!prev) return <span className="delta flat">전년 데이터 없음</span>;
  const pct = Math.round(((cur - prev) / prev) * 100);
  if (pct === 0) return <span className="delta flat">전년과 동일</span>;
  const up = pct > 0;
  const good = goodWhenDown ? !up : up;
  return (
    <span className={`delta ${good ? "good" : "bad"}`}>
      {up ? "▲" : "▼"} {Math.abs(pct)}% <em>전년 대비</em>
    </span>
  );
}

// 세그먼트 토글 버튼 그룹
function Seg<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { key: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="seg">
      {options.map((o) => (
        <button
          key={o.key}
          className={value === o.key ? "active" : ""}
          onClick={() => onChange(o.key)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export default function StatsPage({
  refs,
  month,
  setMonth,
}: {
  refs: Refs;
  month: string;
  setMonth: (m: string) => void;
}) {
  const [dim, setDim] = useState<Dim>("paymentMethod");
  const [mode, setMode] = useState<Mode>("cumulative");

  const [dimData, setDimData] = useState<Record<string, store.DailyByDimension>>({});
  const [compare, setCompare] = useState<store.CumCompare | null>(null);
  const [weekday, setWeekday] = useState<store.WeekdayAvg[]>([]);
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

  // 카테고리 상세 드릴다운 (선택 카테고리·기간에 따라 별도 조회)
  const expenseCats = refs.categories.filter((c) => c.kind === "expense");
  const [catId, setCatId] = useState<number>(0);
  const [catPeriod, setCatPeriod] = useState<"month" | "recent30" | "prev">("month");
  const [detail, setDetail] = useState<store.CategoryDetail | null>(null);

  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [y, m] = month.split("-").map(Number);
      const [pmD, memD, catD, cmp, wd, ct, cc, pc, rec, yr, mcat, mtr, mst, cmem, mtx] =
        await Promise.all([
          GetDailyByDimension(y, m, "paymentMethod"),
          GetDailyByDimension(y, m, "member"),
          GetDailyByDimension(y, m, "category"),
          GetCumulativeCompare(y, m),
          GetWeekdayAverages(y, m),
          GetCategoryTrend(y, m, 6),
          GetCardCategoryBreakdown(y, m),
          GetCardPaces(),
          GetRecurringStatus(y, m),
          GetYearSummary(y),
          GetMemberCategoryBreakdown(y, m),
          GetMemberTrend(y, m, 6),
          GetMemberStats(y, m),
          GetCategoryMemberBreakdown(y, m),
          GetCategoryMatrix(y, m, 6),
        ]);
      setDimData({ paymentMethod: pmD, member: memD, category: catD });
      setCompare(cmp);
      setWeekday(wd);
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
  }, [month]);

  useEffect(() => {
    load();
  }, [load]);

  // 카테고리 미선택 시 첫 지출 카테고리를 기본 선택
  useEffect(() => {
    if (catId === 0 && expenseCats.length > 0) setCatId(expenseCats[0].id);
  }, [catId, expenseCats]);

  // 선택 카테고리 상세 조회 (월/카테고리/기간 변경 시)
  useEffect(() => {
    if (catId === 0) return;
    const [y, m] = month.split("-").map(Number);
    GetCategoryDetail(y, m, catId, catPeriod)
      .then(setDetail)
      .catch(() => setDetail(null));
  }, [month, catId, catPeriod]);

  if (err)
    return (
      <div className="card">
        <p className="error">통계 데이터를 불러오지 못했습니다: {err}</p>
        <button onClick={load}>다시 시도</button>
      </div>
    );

  // 선택된 차원의 일별 시리즈 → 색 부여 (카드는 지정색/자동색, 그 외는 팔레트)
  const cur = dimData[dim];
  const days = cur?.days ?? 0;
  const dimSeries: LineSeries[] = (cur?.series ?? []).map((s) => ({
    name: s.name,
    color: dim === "paymentMethod" ? s.color || autoColor(s.name) : undefined,
    values: s.values,
  }));
  const dayLabels = Array.from({ length: days }, (_, i) => String(i + 1));

  // 전월 동일시점 비교
  const cmpDay = compare?.compareDay ?? 0;
  const curAt = compare ? compare.current[cmpDay - 1] ?? 0 : 0;
  const prevAt = compare ? compare.previous[cmpDay - 1] ?? 0 : 0;
  const cmpDelta = prevAt ? Math.round(((curAt - prevAt) / prevAt) * 100) : 0;

  // 요일별 평균 → Bars 행
  const weekdayRows = weekday.map((w) => ({
    name: WD[w.weekday] + "요일",
    kind: "expense",
    amount: w.avg,
  }));

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
    ? [{ name: detail.category, color: "#5b82f0", values: detail.trend }]
    : [];
  const detailLabels = (detail?.months ?? []).map(monthLabel);
  const detailPrevDelta =
    detail && detail.prev ? Math.round(((detail.total - detail.prev) / detail.prev) * 100) : 0;

  return (
    <div>
      <div className="toolbar">
        <input type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
      </div>

      <div className="card">
        <div className="stats-head">
          <h3>일별 · 누적 지출 추이</h3>
          <div className="seg-row">
            <Seg value={dim} options={DIM_LABEL} onChange={setDim} />
            <Seg<Mode>
              value={mode}
              options={[
                { key: "daily", label: "일별" },
                { key: "cumulative", label: "누적" },
              ]}
              onChange={setMode}
            />
          </div>
        </div>
        {loading ? (
          <Sk h={220} r={12} />
        ) : (
          <LineChart
            series={dimSeries}
            xLabels={dayLabels}
            cumulative={mode === "cumulative"}
          />
        )}
      </div>

      <div className="dash-grid">
        <div className="card">
          <h3>전월 동일시점 누적 비교</h3>
          {loading || !compare ? (
            <Sk h={200} r={12} />
          ) : (
            <>
              <LineChart
                series={[
                  { name: `${monthLabel(compare.curMonth)} (이번 달)`, color: "#5b82f0", values: compare.current },
                  { name: `${monthLabel(compare.prevMonth)} (전월)`, color: "#94a3b8", values: compare.previous },
                ]}
                xLabels={Array.from({ length: compare.days }, (_, i) => String(i + 1))}
                height={200}
                marker={{ x: cmpDay, label: `${cmpDay}일` }}
                showLegend={false}
              />
              <div className="cmp-legend">
                <span>
                  <i style={{ background: "#5b82f0" }} /> 이번 달 {cmpDay}일까지{" "}
                  <strong>{won(curAt)}</strong>
                </span>
                <span>
                  <i style={{ background: "#94a3b8" }} /> 전월 같은 기간 {won(prevAt)}
                </span>
                {prevAt > 0 && (
                  <span className={`delta ${cmpDelta > 0 ? "bad" : "good"}`}>
                    {cmpDelta > 0 ? "▲" : "▼"} {Math.abs(cmpDelta)}% <em>전월 대비</em>
                  </span>
                )}
              </div>
            </>
          )}
        </div>

        <div className="card">
          <h3>요일별 평균 지출</h3>
          {loading ? <Sk h={200} r={12} /> : <Bars title="" rows={weekdayRows} />}
        </div>
      </div>

      <div className="card">
        <h3>카테고리별 6개월 추이</h3>
        {loading ? (
          <Sk h={220} r={12} />
        ) : (
          <LineChart series={catSeries} xLabels={catLabels} />
        )}
      </div>

      <div className="card">
        <div className="stats-head">
          <h3>카테고리 상세 분석</h3>
          <div className="seg-row">
            <Seg
              value={catPeriod}
              options={[
                { key: "month", label: "이번 달" },
                { key: "recent30", label: "최근 30일" },
                { key: "prev", label: "전월" },
              ]}
              onChange={setCatPeriod}
            />
            <select value={catId} onChange={(e) => setCatId(Number(e.target.value))}>
              {expenseCats.map((c) => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </div>
        </div>
        {!detail ? (
          <Sk h={200} r={12} />
        ) : detail.total === 0 && detail.trend.every((v) => v === 0) ? (
          <p className="muted">이 카테고리의 지출 내역이 없습니다.</p>
        ) : (
          <>
            <div className="cat-summary">
              <div className="cat-metric">
                <span className="muted small">{detail.label} 지출</span>
                <strong className="expense">{won(detail.total)}</strong>
                <span className="muted small">전체 지출의 {detail.share}%</span>
              </div>
              <div className="cat-metric">
                <span className="muted small">{detail.prevLabel} 대비</span>
                {detail.prev > 0 ? (
                  <span className={`delta ${detailPrevDelta > 0 ? "bad" : detailPrevDelta < 0 ? "good" : "flat"}`}>
                    {detailPrevDelta > 0 ? "▲" : detailPrevDelta < 0 ? "▼" : ""} {Math.abs(detailPrevDelta)}%
                  </span>
                ) : (
                  <span className="delta flat">{detail.prevLabel} 데이터 없음</span>
                )}
                <span className="muted small">{detail.prevLabel} {won(detail.prev)}</span>
              </div>
              <div className="cat-metric">
                <span className="muted small">예산</span>
                {detail.budget > 0 ? (
                  <>
                    <strong className={detail.over ? "expense" : ""}>{detail.budgetPct}%</strong>
                    <span className="muted small">{won(detail.total)} / {won(detail.budget)}</span>
                  </>
                ) : (
                  <span className="muted small">{catPeriod === "recent30" ? "기간 예산 없음" : "미설정"}</span>
                )}
              </div>
            </div>
            <div className="dash-grid">
              <Bars title="누가 (귀속자별)" rows={detail.byMember} />
              <Bars title="어느 카드로 (결제수단별)" rows={detail.byPayment} />
              <Bars title="어디서 (TOP 가맹점)" rows={detail.topMerchants} />
              <div className="card">
                <h3>월별 추이</h3>
                <LineChart series={detailTrend} xLabels={detailLabels} height={180} showLegend={false} />
              </div>
            </div>
          </>
        )}
      </div>

      <div className="card">
        <h3>카테고리 × 월 히트맵</h3>
        {loading || !matrix ? (
          <Sk h={220} r={12} />
        ) : (
          <Heatmap rows={matrix.series} months={matrix.months} />
        )}
      </div>

      <div className="card">
        <h3>카테고리별 귀속자 구성 ({Number(month.slice(5))}월)</h3>
        {loading ? <Sk h={120} r={12} /> : <StackedBars data={catMember} />}
      </div>

      <div className="card">
        <h3>귀속자별 카테고리 구성 ({Number(month.slice(5))}월)</h3>
        {loading ? <Sk h={120} r={12} /> : <StackedBars data={memberCat} />}
      </div>

      <div className="card">
        <h3>귀속자별 지출 요약 ({Number(month.slice(5))}월)</h3>
        {loading ? (
          <Sk h={120} r={12} />
        ) : memberStats.length === 0 ? (
          <p className="muted">데이터 없음</p>
        ) : (
          <div className="mini-cards">
            {memberStats.map((s) => {
              const pct = s.prev ? Math.round(((s.total - s.prev) / s.prev) * 100) : 0;
              return (
                <div className="mini-card" key={s.member}>
                  <strong>
                    {s.member} <span className="muted small">({s.share}%)</span>
                  </strong>
                  <span className="expense member-total">{won(s.total)}</span>
                  {s.prev > 0 ? (
                    <span className={`delta ${pct > 0 ? "bad" : pct < 0 ? "good" : "flat"}`}>
                      {pct > 0 ? "▲" : pct < 0 ? "▼" : ""} {Math.abs(pct)}% <em>전월 대비</em>
                    </span>
                  ) : (
                    <span className="delta flat">전월 데이터 없음</span>
                  )}
                  <span className="muted small">
                    최다 카테고리: {s.topCategory || "-"}
                    {s.topCategoryAmount ? ` · ${won(s.topCategoryAmount)}` : ""}
                  </span>
                  <span className="muted small">
                    최다 가맹점: {s.topMerchant || "-"}
                    {s.topMerchantAmount ? ` · ${won(s.topMerchantAmount)}` : ""}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="card">
        <h3>귀속자별 6개월 추이</h3>
        {loading ? <Sk h={220} r={12} /> : <LineChart series={memberSeries} xLabels={memberLabels} />}
      </div>

      <div className="card">
        <h3>카드별 카테고리 구성</h3>
        {loading ? <Sk h={120} r={12} /> : <StackedBars data={cardCat} />}
      </div>

      <div className="card">
        <h3>고정비 · 정기결제 ({Number(month.slice(5))}월)</h3>
        {loading ? (
          <Sk h={100} r={12} />
        ) : recurring.length === 0 ? (
          <p className="muted">
            학습된 정기 거래가 없습니다. 거래를 분류하면 규칙이 쌓이고 여기에 표시됩니다.
          </p>
        ) : (
          <ul className="recurring-list">
            {recurring.map((r, i) => (
              <li key={i} className={r.seen ? "seen" : "pending"}>
                <span className="rc-name">{r.label || r.merchant}</span>
                <span className="rc-merchant muted small">{r.merchant}</span>
                {r.seen ? (
                  <span className="badge achieved">
                    빠짐 · {won(r.seenAmount)}
                    {r.seenDate ? ` (${r.seenDate.slice(5)})` : ""}
                  </span>
                ) : (
                  <span className="badge shortfall">
                    예정 · {won(r.amountMin)}~{won(r.amountMax)}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="card">
        <h3>카드 실적기간 누적 진행률</h3>
        {loading ? (
          <Sk h={120} r={12} />
        ) : paces.length === 0 ? (
          <p className="muted">실적 목표가 설정된 카드가 없습니다. 카드 탭에서 실적한도를 입력하세요.</p>
        ) : (
          <div className="mini-cards">
            {paces.map((p) => {
              const c = p.card.color || autoColor(p.card.name);
              const remaining = p.target - p.spent;
              return (
                <div className="mini-card" key={p.card.id}>
                  <strong>
                    <span className="cc-dot" style={{ background: c }} />
                    {p.card.issuer ? `${p.card.issuer} ` : ""}
                    {p.card.name}
                  </strong>
                  <span className="muted small">
                    {p.periodStart} ~ {p.periodEnd} · {p.elapsed}/{p.days}일차
                  </span>
                  <PaceSparkline pace={p} />
                  <div className="pace-foot">
                    <span>
                      {won(p.spent)} <span className="muted">/ 목표 {won(p.target)}</span>
                    </span>
                    {p.achieved ? (
                      <span className="badge achieved">실적 달성 ✓</span>
                    ) : p.onTrack ? (
                      <span className="badge achieved">달성 페이스</span>
                    ) : (
                      <span className="badge shortfall">미달 우려</span>
                    )}
                  </div>
                  <span className="muted small">
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

      <div className="card">
        <h3>연간 요약 ({Number(month.slice(0, 4))}년)</h3>
        {loading || !year ? (
          <Sk h={220} r={12} />
        ) : (
          <>
            <TrendChart points={year.months} />
            <div className="year-totals">
              <div className="card stat">
                <span className="muted">수입</span>
                <strong className="income">{won(year.totalIncome)}</strong>
                <Yoy cur={year.totalIncome} prev={year.prevIncome} />
              </div>
              <div className="card stat">
                <span className="muted">지출</span>
                <strong className="expense">{won(year.totalExpense)}</strong>
                <Yoy cur={year.totalExpense} prev={year.prevExpense} goodWhenDown />
              </div>
              <div className="card stat">
                <span className="muted">투자/저축 이체</span>
                <strong className="transfer">{won(year.totalTransfer)}</strong>
                <Yoy cur={year.totalTransfer} prev={year.prevTransfer} />
              </div>
            </div>
            <Bars title="연간 카테고리별 지출" rows={year.byCategory.slice(0, 10)} />
          </>
        )}
      </div>
    </div>
  );
}
