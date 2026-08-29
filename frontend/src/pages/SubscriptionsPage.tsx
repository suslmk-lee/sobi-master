import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  DeleteSubscription,
  GetSubscriptionMonth,
  SaveSubscription,
} from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import { categoryOptions, formatAmount, parseAmount, Refs, won } from "../lib";
import MonthPicker from "../MonthPicker";
import { LineChart } from "../charts";
import { Sk } from "../Skeleton";

const BLANK = {
  id: 0,
  name: "",
  merchant: "",
  amount: "",
  cycle: "monthly",
  billingDay: "1",
  billingMonth: "1",
  startYm: "",
  endYm: "",
  nextAmount: "",
  nextAmountYm: "",
  categoryId: "",
  memberId: "",
  paymentMethodId: "",
  memo: "",
  active: true,
};
type Form = typeof BLANK;

const monthLabel = (ym: string) => `${Number(ym.slice(5))}월`;

// "YYYY-MM" 입력칸. 비울 수 있어야 해서 MonthPicker 대신 체크박스 + 선택기 조합을 쓴다.
function OptionalMonth({
  label,
  value,
  onChange,
  hint,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  hint: string;
}) {
  const on = value !== "";
  const thisYm = new Date().toISOString().slice(0, 7);
  return (
    <div className="sub-optional">
      <label className="check">
        <input
          type="checkbox"
          checked={on}
          onChange={(e) => onChange(e.target.checked ? thisYm : "")}
        />
        {label}
      </label>
      {on ? (
        <MonthPicker value={value} onChange={onChange} />
      ) : (
        <span className="muted small">{hint}</span>
      )}
    </div>
  );
}

export default function SubscriptionsPage({
  refs,
  month,
  setMonth,
}: {
  refs: Refs;
  month: string;
  setMonth: (m: string) => void;
}) {
  const [data, setData] = useState<main.SubMonth | null>(null);
  const [f, setF] = useState<Form>({ ...BLANK });
  const [editing, setEditing] = useState(false);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [y, m] = month.split("-").map(Number);
      setData(await GetSubscriptionMonth(y, m));
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

  const reset = () => {
    setF({ ...BLANK });
    setEditing(false);
  };

  const edit = (s: store.Subscription) => {
    setF({
      id: s.id,
      name: s.name,
      merchant: s.merchant,
      amount: formatAmount(String(s.amount)),
      cycle: s.cycle,
      billingDay: String(s.billingDay),
      billingMonth: String(s.billingMonth || 1),
      startYm: s.startYm,
      endYm: s.endYm,
      nextAmount: s.nextAmount ? formatAmount(String(s.nextAmount)) : "",
      nextAmountYm: s.nextAmountYm,
      categoryId: s.categoryId ? String(s.categoryId) : "",
      memberId: s.memberId ? String(s.memberId) : "",
      paymentMethodId: s.paymentMethodId ? String(s.paymentMethodId) : "",
      memo: s.memo,
      active: s.active,
    });
    setEditing(true);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!f.name.trim()) return setErr("이름을 입력하세요.");
    try {
      await SaveSubscription(
        new store.Subscription({
          id: f.id,
          name: f.name.trim(),
          merchant: f.merchant.trim(),
          amount: parseAmount(f.amount),
          cycle: f.cycle,
          billingDay: Number(f.billingDay) || 1,
          billingMonth: f.cycle === "yearly" ? Number(f.billingMonth) || 1 : 0,
          startYm: f.startYm,
          endYm: f.endYm,
          nextAmount: parseAmount(f.nextAmount),
          nextAmountYm: f.nextAmountYm,
          categoryId: f.categoryId ? Number(f.categoryId) : undefined,
          memberId: f.memberId ? Number(f.memberId) : undefined,
          paymentMethodId: f.paymentMethodId ? Number(f.paymentMethodId) : undefined,
          memo: f.memo,
          active: f.active,
        })
      );
      reset();
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const remove = async (s: store.Subscription) => {
    if (!window.confirm(`"${s.name}" 정기결제를 삭제할까요?\n기록된 거래는 그대로 남습니다.`)) return;
    try {
      await DeleteSubscription(s.id);
      if (f.id === s.id) reset();
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const toggleActive = async (s: store.Subscription) => {
    try {
      await SaveSubscription(new store.Subscription({ ...s, active: !s.active }));
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const cats = categoryOptions(refs.categories, "expense");
  const items = data?.items ?? [];
  const due = items.filter((i) => i.due);
  const off = items.filter((i) => !i.due);
  const ending = items.filter((i) => i.ending);
  const changing = items.filter((i) => i.changingYm);

  return (
    <div>
      <div className="toolbar wrap">
        <MonthPicker value={month} onChange={setMonth} />
        <span className="muted small">
          등록한 정기결제를 실제 거래와 대조합니다. 거래를 자동으로 만들지는 않습니다.
        </span>
      </div>

      {err && <p className="error">{err}</p>}

      {loading ? (
        <Sk h={92} r={16} />
      ) : (
        <div className="stat-grid">
          <div className="card stat">
            <span className="muted">이번 달 예정</span>
            <strong className="expense">{won(data?.planned ?? 0)}</strong>
            <span className="muted small">{data?.dueCount ?? 0}건</span>
          </div>
          <div className="card stat">
            <span className="muted">실제 빠진 금액</span>
            <strong>{won(data?.seenTotal ?? 0)}</strong>
            <span className="muted small">
              {data?.seenCount ?? 0} / {data?.dueCount ?? 0}건 확인
            </span>
          </div>
          <div className="card stat">
            <span className="muted">월 환산 (연결제 ÷12 포함)</span>
            <strong>{won(data?.monthlyEquivalent ?? 0)}</strong>
          </div>
          <div className="card stat">
            <span className="muted">연간 예상</span>
            <strong>{won(data?.yearlyProjection ?? 0)}</strong>
          </div>
        </div>
      )}

      {(ending.length > 0 || changing.length > 0) && (
        <div className="alerts">
          {ending.map((i) => (
            <div className="alert warn" key={`e${i.sub.id}`}>
              <strong>{i.sub.name} — 이번 달이 마지막</strong>
              <span className="muted small">
                {i.sub.endYm} 이후로는 결제되지 않습니다.
              </span>
            </div>
          ))}
          {changing.map((i) => (
            <div className="alert info" key={`c${i.sub.id}`}>
              <strong>
                {i.sub.name} — {i.changingYm}부터 {won(i.changingAmount)}
              </strong>
              <span className="muted small">
                현재 {won(i.amount)} → {won(i.changingAmount)} (
                {i.changingAmount > i.amount ? "인상" : "인하"})
              </span>
            </div>
          ))}
        </div>
      )}

      <div className="card">
        <h3>{editing ? "정기결제 수정" : "정기결제 등록"}</h3>
        <form onSubmit={submit}>
          <div className="form-row">
            <input
              type="text"
              placeholder="이름 (예: 구글 제미나이)"
              value={f.name}
              onChange={(e) => setF({ ...f, name: e.target.value })}
            />
            <input
              type="text"
              placeholder="대조할 가맹점명 (비우면 이름으로 대조)"
              value={f.merchant}
              onChange={(e) => setF({ ...f, merchant: e.target.value })}
            />
            <input
              type="text"
              inputMode="numeric"
              className="amt"
              placeholder="요금"
              value={f.amount}
              onChange={(e) => setF({ ...f, amount: formatAmount(e.target.value) })}
            />
            <select value={f.cycle} onChange={(e) => setF({ ...f, cycle: e.target.value })}>
              <option value="monthly">매월</option>
              <option value="yearly">매년</option>
            </select>
            {f.cycle === "yearly" && (
              <select
                value={f.billingMonth}
                onChange={(e) => setF({ ...f, billingMonth: e.target.value })}
                title="결제 월"
              >
                {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
                  <option key={m} value={m}>{m}월</option>
                ))}
              </select>
            )}
            <select
              value={f.billingDay}
              onChange={(e) => setF({ ...f, billingDay: e.target.value })}
              title="결제일"
            >
              {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                <option key={d} value={d}>{d}일</option>
              ))}
            </select>
          </div>

          <div className="form-row">
            <select
              value={f.categoryId}
              onChange={(e) => setF({ ...f, categoryId: e.target.value })}
            >
              <option value="">카테고리 선택</option>
              {cats.map((c) => (
                <option key={c.id} value={c.id}>{c.label}</option>
              ))}
            </select>
            <select
              value={f.memberId}
              onChange={(e) => setF({ ...f, memberId: e.target.value })}
            >
              <option value="">귀속자 선택</option>
              {refs.members.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </select>
            <select
              value={f.paymentMethodId}
              onChange={(e) => setF({ ...f, paymentMethodId: e.target.value })}
            >
              <option value="">결제수단 선택</option>
              {refs.paymentMethods.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
            <input
              type="text"
              placeholder="메모"
              value={f.memo}
              onChange={(e) => setF({ ...f, memo: e.target.value })}
            />
          </div>

          <div className="form-row sub-schedule">
            <OptionalMonth
              label="시작 월"
              value={f.startYm}
              onChange={(v) => setF({ ...f, startYm: v })}
              hint="제한 없음"
            />
            <OptionalMonth
              label="종료 월 (이 달까지 결제)"
              value={f.endYm}
              onChange={(v) => setF({ ...f, endYm: v })}
              hint="무기한"
            />
          </div>

          <div className="form-row sub-schedule">
            <OptionalMonth
              label="요금 변경 예정"
              value={f.nextAmountYm}
              onChange={(v) => setF({ ...f, nextAmountYm: v })}
              hint="변경 예정 없음"
            />
            {f.nextAmountYm && (
              <input
                type="text"
                inputMode="numeric"
                className="amt"
                placeholder="변경 후 요금"
                value={f.nextAmount}
                onChange={(e) => setF({ ...f, nextAmount: formatAmount(e.target.value) })}
              />
            )}
          </div>

          <div className="form-row">
            <button type="submit">{editing ? "수정 저장" : "등록"}</button>
            {editing && (
              <button type="button" className="ghost-btn" onClick={reset}>취소</button>
            )}
          </div>
        </form>
      </div>

      <div className="card">
        <h3>이번 달 결제 예정 ({monthLabel(month)})</h3>
        {loading ? (
          <Sk h={120} r={12} />
        ) : due.length === 0 ? (
          <p className="muted">이번 달에 빠질 정기결제가 없습니다.</p>
        ) : (
          <ul className="sub-list">
            {due.map((i) => (
              <li key={i.sub.id} className={i.seen ? "seen" : "pending"}>
                <span className="sub-name">
                  {i.sub.name}
                  {i.sub.cycle === "yearly" && <span className="badge">연 결제</span>}
                </span>
                <span className="sub-meta muted small">
                  매월 {i.sub.billingDay}일
                  {i.sub.categoryName && ` · ${i.sub.categoryName}`}
                  {i.sub.paymentMethodName && ` · ${i.sub.paymentMethodName}`}
                  {i.sub.endYm && ` · ${i.sub.endYm}까지`}
                </span>
                <span className="sub-amount">{won(i.amount)}</span>
                {i.seen ? (
                  <span className="badge achieved">
                    빠짐 · {won(i.seenAmount)}
                    {i.seenDate ? ` (${i.seenDate.slice(5)})` : ""}
                    {i.diff !== 0 && (
                      <em className={i.diff > 0 ? "over" : ""}>
                        {i.diff > 0 ? " +" : " "}
                        {won(i.diff)}
                      </em>
                    )}
                  </span>
                ) : (
                  <span className="badge shortfall">예정</span>
                )}
                <button className="ghost" title="수정" onClick={() => edit(i.sub)}>수정</button>
                <button className="ghost" title="끄기" onClick={() => toggleActive(i.sub)}>끄기</button>
                <button className="ghost" title="삭제" onClick={() => remove(i.sub)}>✕</button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {off.length > 0 && (
        <div className="card">
          <h3>이번 달 해당 없음 <span className="muted small">종료·시작 전·연결제 비결제월·꺼둠</span></h3>
          <ul className="sub-list">
            {off.map((i) => (
              <li key={i.sub.id} className="inactive">
                <span className="sub-name">{i.sub.name}</span>
                <span className="sub-meta muted small">
                  {!i.sub.active
                    ? "꺼둠"
                    : i.sub.endYm && month > i.sub.endYm
                    ? `${i.sub.endYm} 종료`
                    : i.sub.startYm && month < i.sub.startYm
                    ? `${i.sub.startYm} 시작 예정`
                    : i.sub.cycle === "yearly"
                    ? `매년 ${i.sub.billingMonth}월 결제`
                    : ""}
                </span>
                <span className="sub-amount muted">{won(i.amount)}</span>
                <button className="ghost" onClick={() => edit(i.sub)}>수정</button>
                <button className="ghost" onClick={() => toggleActive(i.sub)}>
                  {i.sub.active ? "끄기" : "켜기"}
                </button>
                <button className="ghost" onClick={() => remove(i.sub)}>✕</button>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="card">
        <h3>정기결제 지출 추이 (최근 {data?.trend?.length ?? 6}개월)</h3>
        {loading ? (
          <Sk h={180} r={12} />
        ) : !data?.trend?.length ? (
          <p className="muted">추이를 만들 데이터가 없습니다.</p>
        ) : (
          <LineChart
            xLabels={(data.trendMonths ?? []).map(monthLabel)}
            series={[{ name: "정기결제", color: "#5b82f0", values: data.trend }]}
          />
        )}
      </div>
    </div>
  );
}
