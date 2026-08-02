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
import StatsPage from "./pages/StatsPage";
import ImportPage from "./pages/ImportPage";
import SettingsPage from "./pages/SettingsPage";

type Tab = "dashboard" | "transactions" | "cards" | "stats" | "import" | "settings";

const TABS: { key: Tab; label: string }[] = [
  { key: "dashboard", label: "대시보드" },
  { key: "transactions", label: "거래내역" },
  { key: "cards", label: "카드" },
  { key: "stats", label: "통계" },
  { key: "import", label: "가져오기" },
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
      // 카테고리는 글자순(가나다)으로 정렬해 모든 드롭다운/목록에서 동일하게 보이게 한다
      categories.sort((a, b) => a.name.localeCompare(b.name, "ko"));
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
      <header>
        <h1>소비마스터</h1>
        <nav>
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
        {tab === "stats" && <StatsPage refs={refs} month={month} setMonth={setMonth} />}
        {tab === "import" && <ImportPage refs={refs} />}
        {tab === "settings" && <SettingsPage refs={refs} reload={loadRefs} />}
      </main>
    </div>
  );
}
