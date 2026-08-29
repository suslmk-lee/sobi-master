import {
  DragEvent as ReactDragEvent,
  FormEvent,
  useCallback,
  useEffect,
  useState,
} from "react";
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
  MoveCategory,
  MoveCategoryTo,
  GetLastBackup,
  ListBudgets,
  ListRules,
  SaveRule,
  SetBudget,
  SetCategoryParent,
} from "../../wailsjs/go/main/App";
import { main, store } from "../../wailsjs/go/models";
import {
  categoryOptions,
  formatAmount,
  mainCategories,
  parseAmount,
  Refs,
  thisMonth,
  won,
} from "../lib";
import MonthPicker from "../MonthPicker";

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
          <MonthPicker value={ym} onChange={setYm} />
        )}
      </div>
      <p className="muted small">
        {scope === "default"
          ? "지출 카테고리별 매월 기본 예산입니다. 대시보드에서 사용률·초과를 추적합니다."
          : "해당 월에만 적용할 예산입니다. 비워서 저장하면 그 달은 매월 기본 예산을 따릅니다."}
        {" "}주에 예산을 걸면 하위 부 지출까지 합산해 판정합니다.
      </p>
      <div className="budget-list">
        {expenseCats.map((c) => (
          <div className="budget-row" key={c.id}>
            <span className="budget-name" title={c.fullName}>
              {c.parentId ? `　└ ${c.name}` : c.name}
            </span>
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
          {categoryOptions(refs.categories).map((c) => (
            <option key={c.id} value={c.id}>{c.label}</option>
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

// 카테고리: 주(대분류)/부(소분류) 2단 계층을 종류(지출/수입/이체)별 트리로 보여준다.
// 부의 종류는 주를 따르고, 통계·예산은 기본적으로 주 기준으로 합산된다.
function CategorySection({ refs, reload }: { refs: Refs; reload: () => void }) {
  const [err, setErr] = useState("");
  // 어느 주 아래에 부를 추가하는 중인지
  const [addingUnder, setAddingUnder] = useState<number | null>(null);
  const [subName, setSubName] = useState("");
  // 새 주 카테고리
  const [mainName, setMainName] = useState("");
  const [mainKind, setMainKind] = useState("expense");

  const run = async (fn: () => Promise<unknown>) => {
    setErr("");
    try {
      await fn();
      reload();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const addMain = async (e: FormEvent) => {
    e.preventDefault();
    if (!mainName.trim()) return;
    await run(async () => {
      await AddCategory(mainName.trim(), mainKind, 0);
      setMainName("");
    });
  };

  const addSub = async (e: FormEvent, parentId: number) => {
    e.preventDefault();
    if (!subName.trim()) return;
    await run(async () => {
      await AddCategory(subName.trim(), "expense", parentId); // 종류는 주를 따른다
      setSubName("");
      setAddingUnder(null);
    });
  };

  // 삭제 시 영향을 알린다: 이 카테고리를 쓰던 거래는 미분류가 되고, 부는 주로 올라간다.
  const remove = (name: string, id: number, subCount: number) => {
    const extra = subCount > 0 ? `\n부 카테고리 ${subCount}개는 주 카테고리로 올라갑니다.` : "";
    if (!window.confirm(`"${name}" 카테고리를 삭제할까요?\n이 카테고리로 분류된 거래는 미분류가 됩니다.${extra}`)) {
      return;
    }
    run(() => DeleteCategory(id));
  };

  // 종류별 → 주별로 부를 묶은 트리 (순서는 백엔드가 정한 표시 순서 그대로)
  const treeOf = (kind: string) =>
    mainCategories(refs.categories, kind).map((m) => ({
      main: m,
      subs: refs.categories.filter((c) => c.parentId === m.id),
    }));

  // ---- 표시 순서 드래그 ----
  // 같은 그룹 안에서만 옮길 수 있다. group 키가 같은 행끼리만 서로 드롭을 받는다.
  // (주는 "같은 종류의 주끼리", 부는 "같은 주 아래 부끼리")
  const [drag, setDrag] = useState<{ id: number; group: string } | null>(null);
  const [overId, setOverId] = useState<number | null>(null);

  const canDropOn = (group: string, id: number) =>
    !!drag && drag.group === group && drag.id !== id;

  // 드래그 손잡이 + 그 행을 드롭 대상으로 만드는 속성 묶음.
  // 손잡이만 draggable 로 둬서 행 안의 셀렉트·버튼 조작을 방해하지 않는다.
  const dragProps = (id: number, group: string, index: number) => ({
    onDragOver: (e: ReactDragEvent) => {
      if (!canDropOn(group, id)) return;
      e.preventDefault(); // 이걸 해야 드롭이 허용된다
      setOverId(id);
    },
    onDragLeave: () => setOverId((prev) => (prev === id ? null : prev)),
    onDrop: (e: ReactDragEvent) => {
      e.preventDefault();
      const d = drag;
      setOverId(null);
      setDrag(null);
      if (!d || !canDropOn(group, id)) return;
      run(() => MoveCategoryTo(d.id, index));
    },
  });

  const grip = (id: number, group: string) => (
    <span
      className="cat-grip"
      draggable
      title="끌어서 순서 바꾸기 (↑/↓ 키로도 이동)"
      tabIndex={0}
      onDragStart={(e) => {
        setDrag({ id, group });
        e.dataTransfer.effectAllowed = "move";
        // Firefox 는 데이터가 없으면 드래그를 시작하지 않는다
        e.dataTransfer.setData("text/plain", String(id));
      }}
      onDragEnd={() => {
        setDrag(null);
        setOverId(null);
      }}
      onKeyDown={(e) => {
        if (e.key === "ArrowUp" || e.key === "ArrowDown") {
          e.preventDefault();
          run(() => MoveCategory(id, e.key === "ArrowUp" ? -1 : 1));
        }
      }}
    >
      ☰
    </span>
  );

  const allMains = mainCategories(refs.categories);

  return (
    <div className="card">
      <h3>카테고리 (주/부)</h3>
      <p className="muted small">
        주 아래에 부를 두면 통계·예산이 주 기준으로 합산됩니다(부 단위 상세는 통계 탭에서 토글).
        거래는 주·부 어디에나 분류할 수 있습니다.
      </p>
      {err && <p className="error">{err}</p>}

      {Object.entries(KIND_LABEL).map(([kind, label]) => {
        const tree = treeOf(kind);
        if (tree.length === 0) return null;
        return (
          <div className="cat-kind" key={kind}>
            <h4 className="cat-kind-title">
              {label} <span className="muted small">{tree.length}개 주 카테고리</span>
            </h4>
            <ul className="cat-tree">
              {tree.map(({ main, subs }, mainIdx) => (
                <li className="cat-group" key={main.id}>
                  <div
                    className={
                      "cat-row cat-row-main" + (overId === main.id ? " drag-over" : "")
                    }
                    {...dragProps(main.id, `main:${kind}`, mainIdx)}
                  >
                    {grip(main.id, `main:${kind}`)}
                    <span className="cat-name">{main.name}</span>
                    {subs.length > 0 && (
                      <span className="muted small">부 {subs.length}</span>
                    )}
                    <button
                      className="ghost"
                      title="부 카테고리 추가"
                      onClick={() => {
                        setAddingUnder(addingUnder === main.id ? null : main.id);
                        setSubName("");
                      }}
                    >＋ 부</button>
                    <button
                      className="ghost"
                      title="삭제"
                      onClick={() => remove(main.name, main.id, subs.length)}
                    >✕</button>
                  </div>

                  {addingUnder === main.id && (
                    <form className="cat-row cat-row-add" onSubmit={(e) => addSub(e, main.id)}>
                      <input
                        type="text"
                        autoFocus
                        placeholder={`${main.name} 아래 부 카테고리명`}
                        value={subName}
                        onChange={(e) => setSubName(e.target.value)}
                      />
                      <button type="submit">추가</button>
                      <button
                        type="button"
                        className="ghost"
                        onClick={() => setAddingUnder(null)}
                      >취소</button>
                    </form>
                  )}

                  {subs.map((s, subIdx) => (
                    <div
                      className={
                        "cat-row cat-row-sub" + (overId === s.id ? " drag-over" : "")
                      }
                      key={s.id}
                      {...dragProps(s.id, `sub:${main.id}`, subIdx)}
                    >
                      {grip(s.id, `sub:${main.id}`)}
                      <span className="cat-name">
                        <span className="cat-branch">└</span>
                        {s.name}
                      </span>
                      <select
                        className="cat-parent"
                        value={String(s.parentId)}
                        onChange={(e) => run(() => SetCategoryParent(s.id, Number(e.target.value)))}
                        title="상위(주) 카테고리 이동"
                      >
                        {allMains
                          .filter((m) => m.kind === s.kind)
                          .map((m) => (
                            <option key={m.id} value={m.id}>{m.name}</option>
                          ))}
                        <option value="0">주 카테고리로 올리기</option>
                      </select>
                      <button
                        className="ghost"
                        title="삭제"
                        onClick={() => remove(s.name, s.id, 0)}
                      >✕</button>
                    </div>
                  ))}
                </li>
              ))}
            </ul>
          </div>
        );
      })}

      <form className="form-row cat-add-main" onSubmit={addMain}>
        <input
          type="text"
          placeholder="새 주 카테고리명"
          value={mainName}
          onChange={(e) => setMainName(e.target.value)}
        />
        <select value={mainKind} onChange={(e) => setMainKind(e.target.value)}>
          {Object.entries(KIND_LABEL).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
        <button type="submit">주 카테고리 추가</button>
      </form>
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

// 설정 하위 메뉴. 한 번에 한 섹션만 넓게 보여 가독성을 높인다.
type Section = "categories" | "members" | "payments" | "budget" | "rules" | "backup";

const SECTIONS: { key: Section; label: string; desc: string }[] = [
  { key: "categories", label: "카테고리", desc: "주/부 분류 체계" },
  { key: "members", label: "귀속자", desc: "가족 구성원" },
  { key: "payments", label: "결제수단", desc: "카드·현금·계좌" },
  { key: "budget", label: "예산", desc: "카테고리별 한도" },
  { key: "rules", label: "자동 분류 규칙", desc: "학습된 핑거프린트" },
  { key: "backup", label: "로컬 백업", desc: "SQLite 스냅샷" },
];

export default function SettingsPage({
  refs,
  reload,
}: {
  refs: Refs;
  reload: () => void;
}) {
  const [section, setSection] = useState<Section>("categories");

  const del = async (fn: (id: number) => Promise<void>, id: number) => {
    await fn(id);
    reload();
  };

  const counts: Record<Section, number | null> = {
    categories: refs.categories.length,
    members: refs.members.length,
    payments: refs.paymentMethods.length,
    budget: null,
    rules: null,
    backup: null,
  };

  return (
    <div className="settings-layout">
      <nav className="settings-nav">
        {SECTIONS.map((s) => (
          <button
            key={s.key}
            className={section === s.key ? "active" : ""}
            onClick={() => setSection(s.key)}
          >
            <span className="sn-label">
              {s.label}
              {counts[s.key] !== null && <span className="sn-count">{counts[s.key]}</span>}
            </span>
            <span className="sn-desc">{s.desc}</span>
          </button>
        ))}
      </nav>

      <div className="settings-panel">
        {section === "categories" && <CategorySection refs={refs} reload={reload} />}

        {section === "members" && (
          <div className="card">
            <h3>귀속자 (가족 구성원)</h3>
            <p className="muted small">돈이 누구를 위해 쓰였는지 구분하는 기준입니다.</p>
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
        )}

        {section === "payments" && (
          <div className="card">
            <h3>결제수단 (카드/현금/계좌)</h3>
            <p className="muted small">
              카드의 결제일·실적기간·실적한도는 카드 탭에서 관리합니다.
            </p>
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
        )}

        {section === "budget" && <BudgetSection refs={refs} />}
        {section === "rules" && <RuleSection refs={refs} />}
        {section === "backup" && <BackupSection />}
      </div>
    </div>
  );
}
