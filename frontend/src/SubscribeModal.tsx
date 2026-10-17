import { createPortal } from "react-dom";
import { FormEvent, useEffect, useState } from "react";
import { DraftSubscriptionFromTx, LinkSubscriptionTx, SaveSubscription } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { formatAmount, parseAmount, won } from "./lib";

// 거래 한 건을 정기결제로 등록하는 모달.
// 초안(이름·금액·결제일·다음 달 시작)은 백엔드가 그 거래로 채워 주고, 여기서는 확인·조정만 한다.
// 카테고리·귀속자·결제수단은 거래 것을 그대로 물려받는다.
export default function SubscribeModal({
  txId,
  onClose,
  onSaved,
}: {
  txId: number;
  onClose: () => void;
  onSaved: (sub: store.Subscription) => void;
}) {
  const [draft, setDraft] = useState<store.Subscription | null>(null);
  const [dups, setDups] = useState<store.Subscription[]>([]);
  const [siblings, setSiblings] = useState<store.Subscription[]>([]);
  const [f, setF] = useState({
    name: "",
    merchant: "",
    amount: "",
    cycle: "monthly",
    billingDay: "1",
    billingMonth: "1",
    startYm: "",
    autoPost: true,
  });
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    DraftSubscriptionFromTx(txId)
      .then((r) => {
        const d = r.draft;
        setDraft(d);
        setDups(r.duplicates ?? []);
        setSiblings(r.siblings ?? []);
        setF({
          name: d.name,
          merchant: d.merchant,
          amount: formatAmount(String(d.amount)),
          cycle: d.cycle,
          billingDay: String(d.billingDay),
          billingMonth: String(d.billingMonth),
          startYm: d.startYm,
          autoPost: d.autoPost,
        });
      })
      .catch((e) => setErr(String(e)));
  }, [txId]);

  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (!draft) return;
    setErr("");
    if (!f.name.trim()) {
      setErr("이름을 입력하세요.");
      return;
    }
    const amount = parseAmount(f.amount);
    if (!amount || amount <= 0) {
      setErr("금액(양수)을 입력하세요.");
      return;
    }
    setBusy(true);
    try {
      const saved = await SaveSubscription(
        new store.Subscription({
          ...draft,
          id: 0,
          name: f.name.trim(),
          merchant: f.merchant.trim(),
          amount,
          cycle: f.cycle,
          billingDay: Number(f.billingDay) || 1,
          billingMonth: Number(f.billingMonth) || 1,
          startYm: f.startYm,
          autoPost: f.autoPost,
        })
      );
      // 원본 거래를 이 정기결제의 그 달 결제로 묶는다 — 거래내역의 그 행에 정기 표시가 붙고,
      // 같은 가맹점의 다른 정기결제가 이 거래를 가져가지 않게 된다.
      await LinkSubscriptionTx(saved.id, txId);
      onSaved(saved);
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  // 모달은 body 로 띄운다. 페이지 안에 두면 조상의 transform/filter 가 position:fixed 의
  // 기준 상자를 바꿔 모달이 화면 밖으로 밀려날 수 있다.
  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={save}>
        <h3>정기결제로 등록</h3>
        {!draft ? (
          err ? <p className="error">{err}</p> : <p className="muted">거래를 읽는 중…</p>
        ) : (
          <>
            <p className="muted small">
              이 거래가 있는 달은 이미 결제됐으므로 <strong>{f.startYm}</strong>부터 적용합니다.
              {f.autoPost
                ? " 매달 결제일에 거래가 자동으로 등록됩니다(주말·공휴일이면 다음 업무일)."
                : " 매달 정기결제 탭에서 실제로 빠졌는지·금액이 달라졌는지만 확인합니다."}
            </p>

            {/* 같은 가맹점에 금액도 거의 같으면 같은 결제를 두 번 거는 것일 수 있다 */}
            {dups.length > 0 && (
              <div className="alert warn sub-dup">
                <strong>같은 결제가 이미 정기결제로 등록돼 있는 것 같습니다</strong>
                {dups.map((d) => (
                  <span key={d.id} className="muted small">
                    {d.name} · {won(d.amount)} · 매{d.cycle === "yearly" ? "년" : "월"} {d.billingDay}일
                    {!d.active && " (꺼짐)"}
                  </span>
                ))}
                <span className="muted small">
                  가맹점과 금액이 같습니다. 요금이 바뀐 거라면 새로 만들지 말고 기존 항목을 수정하세요.
                </span>
              </div>
            )}

            {/* 같은 가맹점이지만 금액이 다르면 다른 회선·상품으로 구분된다 — 안내만 */}
            {siblings.length > 0 && (
              <div className="alert info sub-dup">
                <strong>같은 가맹점의 다른 정기결제 {siblings.length}건</strong>
                {siblings.map((d) => (
                  <span key={d.id} className="muted small">
                    {d.name} · {won(d.amount)}
                  </span>
                ))}
                <span className="muted small">
                  금액으로 구분해 각자 자기 요금과 짝지으니 함께 등록해도 됩니다(회선이 여럿인 통신비 등).
                </span>
              </div>
            )}

            <div className="modal-grid">
              <label className="field span2">
                이름
                <input
                  type="text"
                  value={f.name}
                  onChange={(e) => setF({ ...f, name: e.target.value })}
                />
              </label>
              <label className="field">
                금액 (원)
                <input
                  type="text"
                  inputMode="numeric"
                  value={f.amount}
                  onChange={(e) => setF({ ...f, amount: formatAmount(e.target.value) })}
                />
              </label>
              <label className="field span3">
                대조할 가맹점명
                <input
                  type="text"
                  value={f.merchant}
                  onChange={(e) => setF({ ...f, merchant: e.target.value })}
                />
                <span className="muted small">
                  거래 가맹점에 이 글자가 들어 있으면 같은 결제로 봅니다. 날짜·지점이 붙는 이름이면 공통 부분만 남기세요.
                </span>
              </label>
              <label className="field">
                주기
                <select value={f.cycle} onChange={(e) => setF({ ...f, cycle: e.target.value })}>
                  <option value="monthly">매월</option>
                  <option value="yearly">매년</option>
                </select>
              </label>
              {f.cycle === "yearly" && (
                <label className="field">
                  결제 월
                  <select
                    value={f.billingMonth}
                    onChange={(e) => setF({ ...f, billingMonth: e.target.value })}
                  >
                    {Array.from({ length: 12 }, (_, i) => (
                      <option key={i + 1} value={i + 1}>{i + 1}월</option>
                    ))}
                  </select>
                </label>
              )}
              <label className="field">
                결제일
                <input
                  type="number"
                  min={1}
                  max={31}
                  value={f.billingDay}
                  onChange={(e) => setF({ ...f, billingDay: e.target.value })}
                />
              </label>
              <label className="field">
                시작 월
                <input
                  type="month"
                  value={f.startYm}
                  onChange={(e) => setF({ ...f, startYm: e.target.value })}
                />
              </label>
              <label className="field span3 check-field">
                <input
                  type="checkbox"
                  checked={f.autoPost}
                  onChange={(e) => setF({ ...f, autoPost: e.target.checked })}
                />
                <span className="cf-text">매달 거래 자동 등록</span>
                <span className="cf-hint muted">
                  끄면 등록은 직접 하고, 정기결제 탭에서 빠졌는지만 확인합니다
                </span>
              </label>
            </div>
            <p className="muted small">
              카테고리 {draft.categoryName || "미지정"} · 귀속자 {draft.memberName || "미지정"} · 결제수단{" "}
              {draft.paymentMethodName || "미지정"} — 거래에서 그대로 가져옵니다.
            </p>
            {err && <p className="error">{err}</p>}
          </>
        )}
        <div className="modal-actions">
          <button type="button" className="ghost" onClick={onClose}>취소</button>
          <button type="submit" disabled={!draft || busy}>
            {busy ? "등록 중…" : "정기결제로 등록"}
          </button>
        </div>
      </form>
    </div>,
    document.body
  );
}
