import { useCallback, useEffect, useState } from "react";
import {
  ListCategories,
  ListMembers,
  ListPaymentMethods,
} from "../wailsjs/go/main/App";
import { Refs, thisMonth } from "./lib";
import Dashboard from "./pages/Dashboard";
import Transactions from "./pages/Transactions";
import CardsPage from "./pages/CardsPage";
import SubscriptionsPage from "./pages/SubscriptionsPage";
import CashbackPage from "./pages/CashbackPage";
import StatsPage from "./pages/StatsPage";
import SettingsPage from "./pages/SettingsPage";

type Tab =
  | "dashboard"
  | "transactions"
  | "cards"
  | "subscriptions"
  | "cashback"
  | "stats"
  | "settings";

const TABS: { key: Tab; label: string }[] = [
  { key: "dashboard", label: "대시보드" },
  { key: "transactions", label: "거래내역" },
  { key: "cards", label: "카드" },
  { key: "subscriptions", label: "정기결제" },
  { key: "cashback", label: "캐시백" },
  { key: "stats", label: "통계" },
  { key: "settings", label: "설정" },
];

export default function App() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [month, setMonth] = useState(thisMonth());
  const [unclassifiedOnly, setUnclassifiedOnly] = useState(false);
  const [theme, setTheme] = useState<"light" | "dark">(
    () => (localStorage.getItem("sobi-theme") as "light" | "dark") || "light"
  );

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("sobi-theme", theme);
  }, [theme]);

  // 유리 카드 위에서 마우스를 따라 빛이 비치게 한다. 커서가 올라간 카드 하나에만
  // 카드 안 좌표(--mx/--my)를 넘기고, 프레임당 한 번으로 묶어 움직일 때만 다시 그린다.
  useEffect(() => {
    let raf = 0;
    let last: PointerEvent | null = null;
    const apply = () => {
      raf = 0;
      const e = last;
      const card = (e?.target as Element | null)?.closest?.(".card") as HTMLElement | null;
      if (!e || !card) return;
      const r = card.getBoundingClientRect();
      card.style.setProperty("--mx", `${e.clientX - r.left}px`);
      card.style.setProperty("--my", `${e.clientY - r.top}px`);
    };
    const onMove = (e: PointerEvent) => {
      last = e;
      if (!raf) raf = requestAnimationFrame(apply);
    };
    window.addEventListener("pointermove", onMove, { passive: true });
    return () => {
      window.removeEventListener("pointermove", onMove);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);
  const [refs, setRefs] = useState<Refs>({
    members: [],
    categories: [],
    paymentMethods: [],
  });
  const [dbError, setDbError] = useState("");

  const loadRefs = useCallback(async () => {
    try {
      const [members, categories, paymentMethods] = await Promise.all([
        ListMembers(),
        ListCategories(),
        ListPaymentMethods(),
      ]);
      // 카테고리 순서는 백엔드가 정한 그대로 쓴다(설정에서 지정한 표시 순서).
      // 종류 → 주의 순서 → 그 주의 부들 순서로 이미 정렬돼 오므로 여기서 다시 섞지 않는다.
      setRefs({ members, categories, paymentMethods });
      setDbError("");
    } catch (e: any) {
      setDbError(String(e));
    }
  }, []);

  useEffect(() => {
    loadRefs();
  }, [loadRefs]);

  return (
    <div id="app">
      <header className="app-header">
        <div className="brand">
          <div className="brand-mark" aria-hidden="true">₩</div>
          <div className="brand-text">
            <h1>소비마스터</h1>
            <span>우리 집 가계부</span>
          </div>
        </div>
        <nav className="app-nav">
          {TABS.map((t) => (
            <button
              key={t.key}
              className={tab === t.key ? "active" : ""}
              onClick={() => setTab(t.key)}
            >
              {t.label}
            </button>
          ))}
        </nav>
        <button
          className="theme-toggle"
          title={theme === "dark" ? "라이트 모드" : "다크 모드"}
          onClick={() => setTheme((t) => (t === "dark" ? "light" : "dark"))}
        >
          {theme === "dark" ? "☀️" : "🌙"}
        </button>
      </header>
      {dbError && (
        <div className="card banner-error">
          <strong>데이터베이스 연결 오류</strong>
          <p className="small">{dbError}</p>
          <p className="muted small">
            config.json 의 database_url 과 인터넷 연결을 확인한 뒤 다시 시도하세요.
            상세 내용은 설정 폴더의 sobi.log 에 기록됩니다.
          </p>
          <button onClick={loadRefs}>다시 연결</button>
        </div>
      )}
      {/* 탭 전환 시 각 화면이 마운트되며 데이터를 새로 불러온다 */}
      <main>
        {tab === "dashboard" && (
          <Dashboard
            month={month}
            setMonth={setMonth}
            goUnclassified={() => {
              setUnclassifiedOnly(true);
              setTab("transactions");
            }}
          />
        )}
        {tab === "transactions" && (
          <Transactions
            refs={refs}
            month={month}
            setMonth={setMonth}
            unclassifiedOnly={unclassifiedOnly}
            setUnclassifiedOnly={setUnclassifiedOnly}
          />
        )}
        {tab === "cards" && <CardsPage reloadRefs={loadRefs} />}
        {tab === "subscriptions" && (
          <SubscriptionsPage refs={refs} month={month} setMonth={setMonth} />
        )}
        {tab === "cashback" && <CashbackPage month={month} setMonth={setMonth} />}
        {tab === "stats" && <StatsPage refs={refs} month={month} setMonth={setMonth} />}
        {tab === "settings" && <SettingsPage refs={refs} reload={loadRefs} />}
      </main>
    </div>
  );
}
