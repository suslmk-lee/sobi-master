import { useCallback, useEffect, useState } from "react";
import { GetDashboard } from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import { won } from "../lib";
import Bars from "../Bars";
import { DailyChart, Donut, TrendChart } from "../charts";
import { autoColor } from "../PmChip";
import { DashboardSkeleton } from "../Skeleton";

// 전월 대비 증감 배지. prev 가 0이면 비교 불가로 표시하지 않는다.
function Delta({ cur, prev, goodWhenDown }: { cur: number; prev: number; goodWhenDown?: boolean }) {
  if (!prev) return null;
  const pct = Math.round(((cur - prev) / prev) * 100);
  if (pct === 0) return <span className="delta flat">전월과 동일</span>;
  const up = pct > 0;
  const good = goodWhenDown ? !up : up;
  return (
    <span className={`delta ${good ? "good" : "bad"}`}>
      {up ? "▲" : "▼"} {Math.abs(pct)}% <em>전월 대비</em>
    </span>
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

  if (err)
    return (
      <div className="card">
        <p className="error">대시보드 데이터를 불러오지 못했습니다: {err}</p>
        <button onClick={load}>다시 시도</button>
      </div>
    );
  if (!sum) return <DashboardSkeleton />;
  const net = sum.totalIncome - sum.totalExpense;
  const prevNet = prev ? prev.totalIncome - prev.totalExpense : 0;
  const expenseCats = sum.byCategory.filter((c) => c.kind === "expense");
  const transferCats = sum.byCategory.filter((c) => c.kind === "transfer");

  return (
    <div>
      <div className="toolbar">
        <input type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
        {sum.unclassifiedCount > 0 && (
          <button className="warn" onClick={goUnclassified}>
            미분류 거래 {sum.unclassifiedCount}건 — 지금 분류하기
          </button>
        )}
      </div>

      {alerts.length > 0 && (
        <div className="alerts">
          {alerts.map((a, i) => (
            <div className={`alert ${a.level}`} key={i}>
              <strong>{a.title}</strong>
              <span className="muted small">{a.detail}</span>
            </div>
          ))}
        </div>
      )}

      <div className="stat-grid">
        <div className="card stat">
          <span className="muted">수입</span>
          <strong className="income">{won(sum.totalIncome)}</strong>
          <Delta cur={sum.totalIncome} prev={prev?.totalIncome ?? 0} />
        </div>
        <div className="card stat">
          <span className="muted">지출</span>
          <strong className="expense">{won(sum.totalExpense)}</strong>
          <Delta cur={sum.totalExpense} prev={prev?.totalExpense ?? 0} goodWhenDown />
        </div>
        <div className="card stat">
          <span className="muted">투자/저축 이체</span>
          <strong className="transfer">{won(sum.totalTransfer)}</strong>
          <Delta cur={sum.totalTransfer} prev={prev?.totalTransfer ?? 0} />
        </div>
        <div className="card stat">
          <span className="muted">수지 (수입 − 지출)</span>
          <strong className={net >= 0 ? "income" : "expense"}>{won(net)}</strong>
          <Delta cur={net} prev={prevNet} />
        </div>
      </div>

      <div className="card">
        <h3>최근 6개월 수입·지출 추이</h3>
        <TrendChart points={trend} />
      </div>

      {budgets.length > 0 && (
        <div className="card">
          <h3>예산 현황 ({Number(month.slice(5))}월)</h3>
          <div className="budget-status">
            {budgets.map((b) => {
              const pct = Math.min(100, b.pct);
              return (
                <div className="budget-stat" key={b.categoryId}>
                  <div className="budget-stat-head">
                    <span>{b.category}</span>
                    <span className={b.over ? "expense" : "muted"}>
                      {won(b.spent)} / {won(b.amount)} ({b.pct}%)
                    </span>
                  </div>
                  <div className="bar-track">
                    <div
                      className={`bar-fill ${b.over ? "over" : b.pct >= 80 ? "near" : "achieved"}`}
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  <span className={`small ${b.over ? "expense" : "muted"}`}>
                    {b.over ? `${won(-b.remaining)} 초과` : `${won(b.remaining)} 남음`}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <div className="dash-grid">
        <div className="card">
          <h3>이번 달 일별 지출 · 누적</h3>
          <DailyChart points={daily} />
        </div>
        <div className="card">
          <h3>카테고리별 지출 비중</h3>
          <Donut rows={expenseCats} centerLabel="총 지출" />
        </div>
        <div className="card">
          <h3>누구에게 쓴 돈인가</h3>
          <Donut rows={sum.byMember} centerLabel="총 지출" />
        </div>
        <Bars title="지출 TOP 10 가맹점" rows={top} />
        <Bars title="결제수단별 지출 (어느 카드인가)" rows={sum.byPaymentMethod} colors={pmColors} />
        <Bars title="이체 (투자·저축)" rows={transferCats} />
      </div>

      {cards.length > 0 && (
        <div className="card">
          <h3>카드 실적 현황</h3>
          <div className="mini-cards">
            {cards.map((st) => {
              const pct =
                st.card.perfTarget > 0
                  ? Math.min(100, Math.round((st.spent / st.card.perfTarget) * 100))
                  : 0;
              const color = st.card.color || autoColor(st.card.name);
              return (
                <div className="mini-card" key={st.card.id}>
                  <strong>
                    <span className="cc-dot" style={{ background: color }} />
                    {st.card.issuer ? `${st.card.issuer} ` : ""}{st.card.name}
                  </strong>
                  <span className="muted small">{st.periodStart} ~ {st.periodEnd}</span>
                  <span>{won(st.spent)}{st.card.perfTarget > 0 && <> / {won(st.card.perfTarget)}</>}</span>
                  {st.card.perfTarget > 0 && (
                    <>
                      <div className="bar-track">
                        <div
                          className="bar-fill"
                          style={{ width: `${pct}%`, background: color }}
                        />
                      </div>
                      {st.achieved ? (
                        <span className="badge achieved">실적 달성 ✓</span>
                      ) : (
                        <span className="badge shortfall">{won(st.remaining)} 남음</span>
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
