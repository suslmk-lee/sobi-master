import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  AddCategory,
  AddMember,
  AddPaymentMethod,
  BackupNow,
  DeleteBudget,
  DeleteCategory,
  DeleteMember,
  DeletePaymentMethod,
  DeleteRule,
  GetLastBackup,
  ListBudgets,
  ListRules,
  SaveRule,
  SetBudget,
} from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import { formatAmount, parseAmount, Refs, thisMonth, won } from "../lib";

const KIND_LABEL: Record<string, string> = {
  income: "수입",
  expense: "지출",
  transfer: "이체",
};
const PM_LABEL: Record<string, string> = { card: "카드", cash: "현금", bank: "계좌" };

function AddRow({
  placeholder,
  options,
  onAdd,
}: {
  placeholder: string;
  options?: Record<string, string>;
  onAdd: (name: string, extra: string) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [extra, setExtra] = useState(options ? Object.keys(options)[0] : "");
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    await onAdd(name.trim(), extra);
    setName("");
  };
  return (
    <form className="form-row" onSubmit={submit}>
      <input
        type="text"
        placeholder={placeholder}
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      {options && (
        <select value={extra} onChange={(e) => setExtra(e.target.value)}>
          {Object.entries(options).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
      )}
      <button type="submit">추가</button>
    </form>
  );
}

// 카테고리 예산: "매월 기본"('*') 또는 "특정 월"(YYYY-MM) 예산을 정한다. 0 저장 시 해제.
function BudgetSection({ refs }: { refs: Refs }) {
  const [scope, setScope] = useState<"default" | "month">("default");
  const [ym, setYm] = useState(thisMonth());
  const [vals, setVals] = useState<Record<number, string>>({});
  const [saved, setSaved] = useState<Record<number, number>>({});
  const expenseCats = refs.categories.filter((c) => c.kind === "expense");

  const effectiveYm = scope === "default" ? "*" : ym;

  const load = useCallback(async () => {
    const list = await ListBudgets(effectiveYm);
    const m: Record<number, number> = {};
    const f: Record<number, string> = {};
    for (const b of list) {
      m[b.categoryId] = b.amount;
      f[b.categoryId] = formatAmount(String(b.amount));
    }
    setSaved(m);
    setVals(f);
  }, [effectiveYm]);
  useEffect(() => {
    load();
  }, [load]);

  const save = async (catId: number) => {
    await SetBudget(catId, effectiveYm, parseAmount(vals[catId] || ""));
    load();
  };
  const clear = async (catId: number) => {
    await DeleteBudget(catId, effectiveYm);
    setVals((p) => ({ ...p, [catId]: "" }));
    load();
  };
  const total = Object.values(saved).reduce((a, b) => a + b, 0);

  return (
    <div className="card">
      <h3>카테고리 예산</h3>
      <div className="seg-row" style={{ marginBottom: 10 }}>
        <div className="seg">
          <button
            className={scope === "default" ? "active" : ""}
            onClick={() => setScope("default")}
          >매월 기본</button>
          <button
            className={scope === "month" ? "active" : ""}
            onClick={() => setScope("month")}
          >특정 월</button>
        </div>
        {scope === "month" && (
          <input type="month" value={ym} onChange={(e) => setYm(e.target.value)} />
        )}
      </div>
      <p className="muted small">
        {scope === "default"
          ? "지출 카테고리별 매월 기본 예산입니다. 대시보드에서 사용률·초과를 추적합니다."
          : "해당 월에만 적용할 예산입니다. 비워서 저장하면 그 달은 매월 기본 예산을 따릅니다."}
      </p>
      <div className="budget-list">
        {expenseCats.map((c) => (
          <div className="budget-row" key={c.id}>
            <span className="budget-name">{c.name}</span>
            <input
              type="text"
              inputMode="numeric"
              placeholder={scope === "month" ? "기본 따름" : "예산 없음"}
              value={vals[c.id] || ""}
              onChange={(e) =>
                setVals((p) => ({ ...p, [c.id]: formatAmount(e.target.value) }))
              }
              onKeyDown={(e) => e.key === "Enter" && save(c.id)}
            />
            <button className="ghost" onClick={() => save(c.id)}>저장</button>
            {saved[c.id] > 0 && (
              <button className="ghost" onClick={() => clear(c.id)} title="예산 해제">✕</button>
            )}
          </div>
        ))}
      </div>
      <p className="muted small">
        {scope === "default" ? "설정된 총 기본 예산" : `${Number(ym.slice(5))}월 전용 예산 합계`} {won(total)}
      </p>
    </div>
  );
}

const RULE_BLANK = {
  id: 0,
  merchant: "",
  amountMin: "",
  amountMax: "",
  memberId: "",
  categoryId: "",
};
type RuleForm = typeof RULE_BLANK;

// 자동 분류 규칙: 학습된 규칙을 보고 직접 추가/수정/삭제한다.
function RuleSection({ refs }: { refs: Refs }) {
  const [rules, setRules] = useState<store.Rule[]>([]);
  const [f, setF] = useState<RuleForm>({ ...RULE_BLANK });
  const [err, setErr] = useState("");

  const load = useCallback(async () => setRules(await ListRules()), []);
  useEffect(() => {
    load();
  }, [load]);

  const nameOf = (list: { id: number; name: string }[], id?: number) =>
    list.find((x) => x.id === id)?.name || "";

  const edit = (r: store.Rule) =>
    setF({
      id: r.id,
      merchant: r.merchant,
      amountMin: formatAmount(String(r.amountMin)),
      amountMax: formatAmount(String(r.amountMax)),
      memberId: r.memberId ? String(r.memberId) : "",
      categoryId: r.categoryId ? String(r.categoryId) : "",
    });

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    const min = parseAmount(f.amountMin);
    const max = parseAmount(f.amountMax);
    if (!f.merchant.trim()) return setErr("가맹점명을 입력하세요.");
    if (max < min) return setErr("최대 금액이 최소보다 작습니다.");
    const memberName = nameOf(refs.members, Number(f.memberId));
    const categoryName = nameOf(refs.categories, Number(f.categoryId));
    const label = [memberName, categoryName].filter(Boolean).join(" · ");
    try {
      await SaveRule(
        new store.Rule({
          id: f.id,
          merchant: f.merchant.trim(),
          amountMin: min,
          amountMax: max,
          memberId: f.memberId ? Number(f.memberId) : undefined,
          categoryId: f.categoryId ? Number(f.categoryId) : undefined,
          label,
        })
      );
      setF({ ...RULE_BLANK });
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  return (
    <div className="card rules">
      <h3>자동 분류 규칙</h3>
      <p className="muted small">
        거래를 분류할 때마다 자동으로 만들어집니다. 직접 추가·수정할 수도 있습니다.
        가맹점명과 금액 구간이 일치하면 같은 분류를 적용합니다.
      </p>
      <form className="form-row" onSubmit={save}>
        <input
          type="text"
          placeholder="가맹점 (예: 스타벅스)"
          value={f.merchant}
          onChange={(e) => setF({ ...f, merchant: e.target.value })}
        />
        <input
          type="text"
          inputMode="numeric"
          placeholder="최소 금액"
          value={f.amountMin}
          onChange={(e) => setF({ ...f, amountMin: formatAmount(e.target.value) })}
        />
        <input
          type="text"
          inputMode="numeric"
          placeholder="최대 금액"
          value={f.amountMax}
          onChange={(e) => setF({ ...f, amountMax: formatAmount(e.target.value) })}
        />
        <select value={f.memberId} onChange={(e) => setF({ ...f, memberId: e.target.value })}>
          <option value="">귀속자</option>
          {refs.members.map((m) => (
            <option key={m.id} value={m.id}>{m.name}</option>
          ))}
        </select>
        <select value={f.categoryId} onChange={(e) => setF({ ...f, categoryId: e.target.value })}>
          <option value="">카테고리</option>
          {refs.categories.map((c) => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
        <button type="submit">{f.id ? "수정" : "추가"}</button>
        {f.id !== 0 && (
          <button type="button" className="ghost" onClick={() => setF({ ...RULE_BLANK })}>취소</button>
        )}
      </form>
      {err && <p className="error">{err}</p>}
      <table className="tx-table">
        <thead>
          <tr>
            <th>가맹점</th>
            <th className="num">금액 구간</th>
            <th>분류</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {rules.map((r) => (
            <tr key={r.id} className={f.id === r.id ? "row-editing" : ""}>
              <td>{r.merchant}</td>
              <td className="num">{won(r.amountMin)} ~ {won(r.amountMax)}</td>
              <td>{r.label || "-"}</td>
              <td className="actions">
                <button className="ghost" onClick={() => edit(r)} title="수정">✎</button>
                <button
                  className="ghost"
                  onClick={async () => {
                    await DeleteRule(r.id);
                    load();
                  }}
                  title="삭제"
                >✕</button>
              </td>
            </tr>
          ))}
          {rules.length === 0 && (
            <tr>
              <td colSpan={4} className="muted center">아직 규칙이 없습니다</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

// 로컬 백업: 운영은 Supabase, 정기적으로 로컬 SQLite 파일로 스냅샷 백업된다.
function BackupSection() {
  const [last, setLast] = useState<main.BackupInfo | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");

  const load = useCallback(async () => {
    try {
      setLast(await GetLastBackup());
    } catch {
      setLast(null);
    }
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  const backupNow = async () => {
    setBusy(true);
    setMsg("");
    try {
      const info = await BackupNow();
      setLast(info);
      const total = info.counts
        ? Object.values(info.counts).reduce((a, b) => a + b, 0)
        : 0;
      setMsg(`백업 완료 · ${total}건 · ${info.sizeKB}KB`);
    } catch (e: any) {
      setMsg(String(e));
    } finally {
      setBusy(false);
    }
  };

  const folder = last?.path ? last.path.replace(/[/\\][^/\\]+$/, "") : "";

  return (
    <div className="card">
      <h3>로컬 백업</h3>
      <p className="muted small">
        운영 데이터베이스는 Supabase입니다. 앱 실행 중 자동으로(시작 직후·6시간마다·종료 시)
        로컬 SQLite 파일로 스냅샷 백업하며, 최근 7개를 보관합니다.
      </p>
      <div className="backup-status">
        {last?.path ? (
          <>
            <span>
              최근 백업 <strong>{last.time}</strong>
              {last.sizeKB > 0 && <span className="muted small"> · {last.sizeKB}KB</span>}
            </span>
            <span className="muted small backup-path" title={folder}>{folder}</span>
          </>
        ) : (
          <span className="muted">아직 백업이 없습니다.</span>
        )}
      </div>
      <div className="form-row">
        <button onClick={backupNow} disabled={busy}>
          {busy ? "백업 중…" : "지금 백업"}
        </button>
      </div>
      {msg && <p className="muted small">{msg}</p>}
    </div>
  );
}

export default function SettingsPage({
  refs,
  reload,
}: {
  refs: Refs;
  reload: () => void;
}) {
  const del = async (fn: (id: number) => Promise<void>, id: number) => {
    await fn(id);
    reload();
  };

  return (
    <div className="settings-grid">
      <div className="card">
        <h3>귀속자 (가족 구성원)</h3>
        <ul className="item-list">
          {refs.members.map((m) => (
            <li key={m.id}>
              {m.name}
              <button className="ghost" onClick={() => del(DeleteMember, m.id)}>✕</button>
            </li>
          ))}
        </ul>
        <AddRow
          placeholder="이름 (예: 할머니)"
          onAdd={async (name) => {
            await AddMember(name);
            reload();
          }}
        />
      </div>

      <div className="card">
        <h3>카테고리</h3>
        <ul className="item-list">
          {refs.categories.map((c) => (
            <li key={c.id}>
              {c.name} <span className="badge">{KIND_LABEL[c.kind]}</span>
              <button className="ghost" onClick={() => del(DeleteCategory, c.id)}>✕</button>
            </li>
          ))}
        </ul>
        <AddRow
          placeholder="카테고리명"
          options={KIND_LABEL}
          onAdd={async (name, kind) => {
            await AddCategory(name, kind);
            reload();
          }}
        />
      </div>

      <div className="card">
        <h3>결제수단 (카드/현금/계좌)</h3>
        <ul className="item-list">
          {refs.paymentMethods.map((p) => (
            <li key={p.id}>
              {p.name} <span className="badge">{PM_LABEL[p.type]}</span>
              <button className="ghost" onClick={() => del(DeletePaymentMethod, p.id)}>✕</button>
            </li>
          ))}
        </ul>
        <AddRow
          placeholder="이름 (예: 신한카드 1234)"
          options={PM_LABEL}
          onAdd={async (name, typ) => {
            await AddPaymentMethod(name, typ);
            reload();
          }}
        />
      </div>

      <BudgetSection refs={refs} />

      <BackupSection />

      <RuleSection refs={refs} />
    </div>
  );
}
