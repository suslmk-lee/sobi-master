import { useCallback, useEffect, useState } from "react";
import { GetCashback, MarkPaid } from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import { won } from "../lib";
import MonthPicker from "../MonthPicker";
import { autoColor } from "../PmChip";
import { Sk } from "../Skeleton";

// 캐시백 화면.
//
// 카드마다 결제액의 일정 비율이 기본으로 쌓이고, 카드에 따라서는 정해진 기한 안에
// 카드대금을 갚으면 추가분을 한 번 더 준다. 추가분은 기한을 넘기면 못 받으므로
// "지금 갚으면 받을 수 있는 건"을 맨 위에 남은 일수와 함께 띄우는 게 이 화면의 핵심이다.

const bpPct = (bp: number) => `${(bp / 100).toFixed(bp % 100 === 0 ? 0 : 2)}%`;

// 남은 일수를 D-표기로. 0이면 오늘이 마감.
const dday = (n: number) => (n === 0 ? "오늘 마감" : `D-${n}`);

export default function CashbackPage({
  month,
  setMonth,
}: {
  month: string;
  setMonth: (m: string) => void;
}) {
  const [rep, setRep] = useState<store.CashbackReport | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    try {
      const [y, m] = month.split("-").map(Number);
      setRep(await GetCashback(y, m));
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    }
  }, [month]);

  useEffect(() => {
    load();
  }, [load]);

  // paidAt: "" = 오늘로 기록, "-" = 기록 지우기
  const mark = async (ids: number[], paidAt: string, label: string) => {
    if (ids.length === 0) return;
    setBusy(true);
    setMsg("");
    try {
      await MarkPaid(ids, paidAt);
      setMsg(`${ids.length}건 ${label}`);
      await load();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  if (err)
    return (
      <div className="card">
        <p className="error">캐시백 정보를 불러오지 못했습니다: {err}</p>
        <button onClick={load}>다시 시도</button>
      </div>
    );

  if (!rep) {
    return (
      <div>
        <div className="toolbar">
          <MonthPicker value={month} onChange={setMonth} />
        </div>
        <div className="card"><Sk h={160} r={12} /></div>
      </div>
    );
  }

  if (!rep.hasCashbackCard) {
    return (
      <div>
        <div className="toolbar">
          <MonthPicker value={month} onChange={setMonth} />
        </div>
        <div className="card">
          <h3>캐시백</h3>
          <p className="muted">
            캐시백이 설정된 카드가 없습니다. <strong>카드 탭</strong>에서 카드를 수정해
            기본 적립률(예: 1%)과, 기한 내 납부 시 주는 추가 적립률·기한일(예: 1% / 5일)을
            입력하면 여기에 집계됩니다.
          </p>
        </div>
      </div>
    );
  }

  const pendingTotal = rep.pending.reduce((s, i) => s + i.possible, 0);

  return (
    <div>
      <div className="toolbar">
        <MonthPicker value={month} onChange={setMonth} />
      </div>

      {msg && <p className="muted small toast-line">{msg}</p>}

      {/* 기한이 남은 건 — 이 화면의 핵심. 월과 무관하게 항상 보여준다. */}
      <div className="card cb-pending">
        <div className="stats-head">
          <h3>지금 갚으면 추가 적립</h3>
          {rep.pending.length > 0 && (
            <button
              disabled={busy}
              onClick={() => mark(rep.pending.map((i) => i.txId), "", "납부 처리했습니다.")}
            >
              전부 오늘 납부 처리
            </button>
          )}
        </div>
        {rep.pending.length === 0 ? (
          <p className="muted">기한이 남은 미납부 결제가 없습니다. 놓친 추가 적립이 없네요.</p>
        ) : (
          <>
            <p className="muted small">
              아래 {rep.pending.length}건을 기한 안에 갚으면 <strong>{won(pendingTotal)}</strong>을
              더 받습니다. 기한이 급한 순서입니다.
            </p>
            <ul className="cb-list">
              {rep.pending.map((it) => (
                <li key={it.txId} className={it.daysLeft <= 1 ? "urgent" : ""}>
                  <span className={`cb-dday ${it.daysLeft <= 1 ? "hot" : ""}`}>{dday(it.daysLeft)}</span>
                  <span className="cb-date muted small">{it.date.slice(5)}</span>
                  <span className="cb-merchant">{it.merchant}</span>
                  <span className="cb-card muted small">
                    <span className="cc-dot" style={{ background: autoColor(it.card) }} />
                    {it.card}
                  </span>
                  <span className="cb-amt">{won(it.amount)}</span>
                  <span className="cb-bonus income">+{won(it.possible)}</span>
                  <button
                    className="ghost-btn"
                    disabled={busy}
                    onClick={() => mark([it.txId], "", "납부 처리했습니다.")}
                  >
                    납부 처리
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>

      {/* 월별 카드 집계 */}
      {rep.cards.map((c) => {
        const total = c.totalBase + c.totalBonus;
        return (
          <div className="card" key={c.card.id}>
            <div className="stats-head">
              <h3>
                <span className="cc-dot" style={{ background: c.card.color || autoColor(c.card.name) }} />
                {c.card.name}
                <span className="muted small">
                  {" "}기본 {bpPct(c.card.cashbackBp)}
                  {c.card.cashbackBonusBp > 0 &&
                    ` · ${c.card.cashbackBonusDays}일 내 납부 시 +${bpPct(c.card.cashbackBonusBp)}`}
                </span>
              </h3>
              <span className="cb-total">
                {Number(month.slice(5))}월 적립 <strong className="income">{won(total)}</strong>
              </span>
            </div>

            <div className="cb-summary">
              <div className="cb-metric">
                <span className="muted small">대상 결제</span>
                <strong>{won(c.totalAmount)}</strong>
              </div>
              <div className="cb-metric">
                <span className="muted small">기본 적립</span>
                <strong className="income">{won(c.totalBase)}</strong>
              </div>
              <div className="cb-metric">
                <span className="muted small">추가 적립(획득)</span>
                <strong className="income">{won(c.totalBonus)}</strong>
              </div>
              <div className="cb-metric">
                <span className="muted small">놓친 추가분</span>
                <strong className={c.missedBonus > 0 ? "expense" : ""}>{won(c.missedBonus)}</strong>
              </div>
            </div>

            {c.items.length === 0 ? (
              <p className="muted">이 달 결제 내역이 없습니다.</p>
            ) : (
              <table className="tx-table cb-table">
                <thead>
                  <tr>
                    <th>날짜</th>
                    <th>내용</th>
                    <th className="num">금액</th>
                    <th className="num">기본</th>
                    <th className="num">추가</th>
                    <th>납부</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {c.items.map((it) => (
                    <tr key={it.txId}>
                      <td>{it.date.slice(5)}</td>
                      <td>{it.merchant}</td>
                      <td className="num">{won(it.amount)}</td>
                      <td className="num income">{won(it.base)}</td>
                      <td className="num">
                        {it.status === "earned" && it.bonus > 0 && (
                          <span className="income">{won(it.bonus)}</span>
                        )}
                        {it.status === "pending" && (
                          <span className="badge shortfall">{dday(it.daysLeft)} · +{won(it.possible)}</span>
                        )}
                        {it.status === "missed" && (
                          <span className="muted" title={`놓친 추가분 ${won(it.possible)}`}>—</span>
                        )}
                      </td>
                      <td>
                        {it.paidAt ? (
                          <span className="muted small">
                            {it.paidAt.slice(5)} ({it.paidInDays}일)
                          </span>
                        ) : (
                          <span className="muted small">미납부</span>
                        )}
                      </td>
                      <td className="actions">
                        {it.paidAt ? (
                          <button
                            className="ghost"
                            title="납부 기록 취소"
                            disabled={busy}
                            onClick={() => mark([it.txId], "-", "납부 기록을 지웠습니다.")}
                          >↩</button>
                        ) : (
                          <button
                            className="ghost"
                            title="오늘 납부 처리"
                            disabled={busy}
                            onClick={() => mark([it.txId], "", "납부 처리했습니다.")}
                          >✓</button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        );
      })}
    </div>
  );
}
