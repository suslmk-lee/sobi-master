import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { MiniTrend } from "./charts";

// 표의 칸 하나를 열어 보는 모달.
// 위에는 그 항목의 월별 추이, 아래에는 고른 달의 거래 목록. 목록에서 바로 고칠 수 있다.
// 통계의 "월별 상세" 표와 "카테고리 × 월 히트맵"이 같이 쓴다.
export default function TxDrillModal({
  title,
  ym,
  months,
  values,
  txs,
  onPickMonth,
  onEdit,
  onClose,
}: {
  title: string;
  ym: string; // "YYYY-MM"
  months: string[]; // 추이에 그릴 월 목록 (없으면 그래프 생략)
  values: number[]; // months 와 같은 길이
  txs: store.Transaction[];
  onPickMonth: (ym: string) => void;
  onEdit: (t: store.Transaction) => void;
  onClose: () => void;
}) {
  const active = months.indexOf(ym);
  const total = values.reduce((s, v) => s + v, 0);
  const prev = active > 0 ? values[active - 1] : 0;
  const delta =
    active > 0 && prev > 0 ? Math.round(((values[active] - prev) / prev) * 100) : null;
  const sum = txs.reduce((s, t) => s + t.amount, 0);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal drill-modal" onClick={(e) => e.stopPropagation()}>
        <div className="stats-head">
          <h3>
            {title} · {Number(ym.slice(5))}월
            <span className="muted small"> ({ym})</span>
          </h3>
          <button className="ghost" onClick={onClose}>✕</button>
        </div>

        {/* 이 항목이 달마다 어떻게 움직였는지 먼저 보여 준다 */}
        {months.length > 0 && values.length === months.length && (
          <div className="drill-trend">
            <MiniTrend
              values={values}
              months={months}
              active={active}
              onPick={(i) => onPickMonth(months[i])}
            />
            <p className="muted small">
              {months.length}개월 합계 {won(total)}
              {delta !== null && (
                <>
                  {" · 전월 대비 "}
                  <span className={`delta ${delta > 0 ? "bad" : delta < 0 ? "good" : "flat"}`}>
                    {delta > 0 ? "▲" : delta < 0 ? "▼" : ""} {Math.abs(delta)}%
                  </span>
                </>
              )}
              {" · 값이 있는 달을 누르면 그 달로 이동합니다"}
            </p>
          </div>
        )}

        {txs.length === 0 ? (
          <p className="muted">거래를 불러오지 못했거나 내역이 없습니다.</p>
        ) : (
          <>
            <p className="muted small">
              {txs.length}건 · 합계 <strong className="expense">{won(sum)}</strong>
            </p>
            <div className="drill-scroll">
              <table className="tx-table">
                <thead>
                  <tr>
                    <th className="nowrap">날짜</th>
                    <th>내용</th>
                    <th className="nowrap">카테고리</th>
                    <th className="nowrap">귀속자</th>
                    <th className="nowrap">결제수단</th>
                    <th className="num">금액</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {txs.map((t) => (
                    <tr key={t.id}>
                      <td className="nowrap">{t.date}</td>
                      <td>
                        {t.merchant}
                        {t.memo && <span className="muted small"> · {t.memo}</span>}
                      </td>
                      <td className="nowrap">{t.categoryName || <span className="muted">미지정</span>}</td>
                      <td className="nowrap">{t.memberName || <span className="muted">미지정</span>}</td>
                      <td className="nowrap">{t.paymentMethodName || <span className="muted">미지정</span>}</td>
                      <td className="num expense">{won(t.amount)}</td>
                      <td className="actions">
                        <button className="ghost" onClick={() => onEdit(t)}>수정</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
