import { FormEvent, useEffect, useState } from "react";
import { AlertTriangle, Info, Repeat, X } from "lucide-react";
import { DraftSubscriptionFromTx, LinkSubscriptionTx, SaveSubscription } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { formatAmount, parseAmount, won } from "./lib";
import Portal from "./components/ui/Portal";
import ScheduleFields, { SCHEDULE_DEFAULTS, scheduleFromSub } from "./ScheduleFields";
import { describeSchedule, holidayHint } from "./subSchedule";
import IconTile from "./components/ui/IconTile";

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
    ...SCHEDULE_DEFAULTS,
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
          billingMonth: String(d.billingMonth || 1),
          ...scheduleFromSub(d),
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
          billingMonth: f.cycle === "yearly" ? Number(f.billingMonth) || 1 : 0,
          dayRule: f.dayRule,
          weekNth: Number(f.weekNth) || 1,
          weekday: Number(f.weekday),
          holidayRule: f.holidayRule,
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

  // 모달은 Portal 로 body 에 띄운다. 페이지 안에 두면 조상의 transform/filter 가
  // position:fixed 의 기준 상자를 바꿔 모달이 화면 밖으로 밀려날 수 있다.
  return (
    <Portal>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 backdrop-blur-sm sm:p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget) onClose();
        }}
      >
        <form
          role="dialog"
          aria-modal="true"
          aria-label="정기결제로 등록"
          className="surface max-h-[92vh] w-full max-w-2xl overflow-y-auto p-4 sm:p-6"
          onSubmit={save}
        >
          <div className="mb-4 flex items-start justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <IconTile icon={Repeat} tone="sky" size="sm" />
              <h3 className="section-title">정기결제로 등록</h3>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="btn-icon shrink-0"
              aria-label="닫기"
              title="닫기"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          {!draft ? (
            err ? (
              <p className="text-xs text-rose-300">{err}</p>
            ) : (
              <p className="py-6 text-center text-sm text-zinc-500">거래를 읽는 중…</p>
            )
          ) : (
            <div className="space-y-3">
              <p className="text-xs text-zinc-400">
                이 거래가 있는 달은 이미 결제됐으므로{" "}
                <strong className="num font-medium text-white">{f.startYm}</strong>부터 적용합니다.
                {f.autoPost
                  ? " 매달 결제일에 거래가 자동으로 등록됩니다({holidayHint(f.holidayRule)})."
                  : " 매달 정기결제 탭에서 실제로 빠졌는지·금액이 달라졌는지만 확인합니다."}
              </p>

              {/* 같은 가맹점에 금액도 거의 같으면 같은 결제를 두 번 거는 것일 수 있다 */}
              {dups.length > 0 && (
                <div className="flex items-start gap-2.5 rounded-xl border border-amber-300/30 bg-amber-400/10 p-3">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-300" strokeWidth={1.75} />
                  <div className="flex min-w-0 flex-col gap-0.5">
                    <strong className="text-sm font-medium text-amber-200">
                      같은 결제가 이미 정기결제로 등록돼 있는 것 같습니다
                    </strong>
                    {dups.map((d) => (
                      <span key={d.id} className="text-xs text-zinc-400">
                        {d.name} · <span className="num">{won(d.amount)}</span> · {describeSchedule(d)}
                        {!d.active && " (꺼짐)"}
                      </span>
                    ))}
                    <span className="text-xs text-zinc-400">
                      가맹점과 금액이 같습니다. 요금이 바뀐 거라면 새로 만들지 말고 기존 항목을 수정하세요.
                    </span>
                  </div>
                </div>
              )}

              {/* 같은 가맹점이지만 금액이 다르면 다른 회선·상품으로 구분된다 — 안내만 */}
              {siblings.length > 0 && (
                <div className="flex items-start gap-2.5 rounded-xl border border-sky-300/30 bg-sky-400/10 p-3">
                  <Info className="mt-0.5 h-4 w-4 shrink-0 text-sky-300" strokeWidth={1.75} />
                  <div className="flex min-w-0 flex-col gap-0.5">
                    <strong className="text-sm font-medium text-sky-200">
                      같은 가맹점의 다른 정기결제 <span className="num">{siblings.length}</span>건
                    </strong>
                    {siblings.map((d) => (
                      <span key={d.id} className="text-xs text-zinc-400">
                        {d.name} · <span className="num">{won(d.amount)}</span>
                      </span>
                    ))}
                    <span className="text-xs text-zinc-400">
                      금액으로 구분해 각자 자기 요금과 짝지으니 함께 등록해도 됩니다(회선이 여럿인 통신비 등).
                    </span>
                  </div>
                </div>
              )}

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <label className="block sm:col-span-2">
                  <span className="field-label">이름</span>
                  <input
                    type="text"
                    className="input-field"
                    value={f.name}
                    onChange={(e) => setF({ ...f, name: e.target.value })}
                  />
                </label>
                <label className="block">
                  <span className="field-label">금액 (원)</span>
                  <input
                    type="text"
                    inputMode="numeric"
                    className="input-field num"
                    value={f.amount}
                    onChange={(e) => setF({ ...f, amount: formatAmount(e.target.value) })}
                  />
                </label>
                <label className="block sm:col-span-3">
                  <span className="field-label">대조할 가맹점명</span>
                  <input
                    type="text"
                    className="input-field"
                    value={f.merchant}
                    onChange={(e) => setF({ ...f, merchant: e.target.value })}
                  />
                  <span className="mt-1 block text-xs text-zinc-500">
                    거래 가맹점에 이 글자가 들어 있으면 같은 결제로 봅니다. 날짜·지점이 붙는 이름이면 공통 부분만 남기세요.
                  </span>
                </label>
                <label className="block">
                  <span className="field-label">주기</span>
                  <select
                    className="input-field"
                    value={f.cycle}
                    onChange={(e) => setF({ ...f, cycle: e.target.value })}
                  >
                    <option value="monthly">매월</option>
                    <option value="yearly">매년</option>
                  </select>
                </label>
                <label className="block">
                  <span className="field-label">시작 월</span>
                  <input
                    type="month"
                    className="input-field num"
                    value={f.startYm}
                    onChange={(e) => setF({ ...f, startYm: e.target.value })}
                  />
                </label>
                <div className="sm:col-span-3">
                  <ScheduleFields value={f} onChange={(p) => setF({ ...f, ...p })} />
                </div>
                <label className="surface-inset flex cursor-pointer flex-wrap items-center gap-x-2.5 gap-y-1 p-3 sm:col-span-3">
                  <input
                    type="checkbox"
                    className="h-4 w-4 accent-mint-400"
                    checked={f.autoPost}
                    onChange={(e) => setF({ ...f, autoPost: e.target.checked })}
                  />
                  <span className="text-sm text-zinc-200">매달 거래 자동 등록</span>
                  <span className="text-xs text-zinc-500">
                    끄면 등록은 직접 하고, 정기결제 탭에서 빠졌는지만 확인합니다
                  </span>
                </label>
              </div>
              <p className="text-xs text-zinc-500">
                카테고리 {draft.categoryName || "미지정"} · 귀속자 {draft.memberName || "미지정"} · 결제수단{" "}
                {draft.paymentMethodName || "미지정"} — 거래에서 그대로 가져옵니다.
              </p>
              {err && <p className="text-xs text-rose-300">{err}</p>}
            </div>
          )}
          <div className="mt-5 flex items-center justify-end gap-2">
            <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
            <button type="submit" className="btn-primary" disabled={!draft || busy}>
              {busy ? "등록 중…" : "정기결제로 등록"}
            </button>
          </div>
        </form>
      </div>
    </Portal>
  );
}
