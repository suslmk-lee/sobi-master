import { type ReactNode, useCallback, useEffect, useState } from "react";
import { Download, Loader2, PackageOpen, Sparkles, X } from "lucide-react";
import { CheckForUpdate, DownloadUpdate, GetAppVersion, OpenDownloadedUpdate } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../wailsjs/runtime/runtime";
import Portal from "./ui/Portal";

// 헤더의 버전 표시와 새 버전 알림.
// - 평소: 지금 버전(v1.0.0). 누르면 바로 업데이트를 확인한다.
// - 새 버전이 있으면: '새 버전 받기' 버튼. 누르면 바뀌는 점을 먼저 보여 주고 내려받기를 고를 수 있다.
// 앱이 켜지고 몇 초 뒤, 그리고 6시간마다 조용히 확인한다(실패해도 알리지 않는다).

const CHECK_DELAY_MS = 3000;
const CHECK_EVERY_MS = 6 * 60 * 60 * 1000;

const mb = (bytes: number) => `${(bytes / 1024 / 1024).toFixed(bytes >= 100 * 1024 * 1024 ? 0 : 1)}MB`;

export default function UpdateControl() {
  const [version, setVersion] = useState("");
  const [info, setInfo] = useState<main.UpdateInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [checking, setChecking] = useState(false);
  const [note, setNote] = useState(""); // 직접 확인했을 때의 한 줄 결과 (잠깐 보이고 사라진다)

  useEffect(() => {
    GetAppVersion()
      .then(setVersion)
      .catch(() => {});
  }, []);

  const check = useCallback(async (manual: boolean) => {
    if (manual) {
      setChecking(true);
      setNote("");
    }
    try {
      const r = await CheckForUpdate();
      setInfo(r);
      if (manual) {
        if (r.available) setOpen(true);
        else setNote("최신 버전입니다");
      }
    } catch (e) {
      if (manual) setNote("확인하지 못했습니다");
    } finally {
      if (manual) setChecking(false);
    }
  }, []);

  useEffect(() => {
    const first = window.setTimeout(() => check(false), CHECK_DELAY_MS);
    const every = window.setInterval(() => check(false), CHECK_EVERY_MS);
    return () => {
      window.clearTimeout(first);
      window.clearInterval(every);
    };
  }, [check]);

  useEffect(() => {
    if (!note) return;
    const t = window.setTimeout(() => setNote(""), 3500);
    return () => window.clearTimeout(t);
  }, [note]);

  return (
    <>
      {info?.available && (
        <button
          type="button"
          onClick={() => setOpen(true)}
          title={`새 버전 v${info.latest} — 바뀌는 점 보기`}
          className="glass-pill flex h-9 items-center gap-1.5 !border-mint-400/40 px-3 text-xs font-medium text-mint-300 transition-colors hover:text-white"
        >
          <Sparkles className="h-3.5 w-3.5" />
          새 버전 받기
          <span className="num text-mint-200/80">v{info.latest}</span>
        </button>
      )}
      <button
        type="button"
        onClick={() => check(true)}
        title="업데이트 확인"
        className="glass-pill num flex h-9 min-w-[3.5rem] items-center justify-center px-3 text-xs text-zinc-400 transition-colors hover:text-white"
      >
        {checking ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : note || (version ? `v${version}` : "")}
      </button>
      {open && info && <UpdateModal info={info} onClose={() => setOpen(false)} />}
    </>
  );
}

type Phase = "idle" | "downloading" | "done" | "error";

function UpdateModal({ info, onClose }: { info: main.UpdateInfo; onClose: () => void }) {
  const [phase, setPhase] = useState<Phase>("idle");
  const [prog, setProg] = useState({ done: 0, total: info.assetSize });
  const [path, setPath] = useState("");
  const [err, setErr] = useState("");
  const busy = phase === "downloading";

  useEffect(
    () =>
      EventsOn("update:progress", (done: number, total: number) =>
        setProg({ done, total: total > 0 ? total : info.assetSize })
      ),
    [info.assetSize]
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

  const download = async () => {
    setPhase("downloading");
    setErr("");
    setProg({ done: 0, total: info.assetSize });
    try {
      setPath(await DownloadUpdate());
      setPhase("done");
    } catch (e) {
      setErr(String(e));
      setPhase("error");
    }
  };

  const openFile = async () => {
    try {
      await OpenDownloadedUpdate();
    } catch (e) {
      setErr(String(e));
    }
  };

  const pct = prog.total > 0 ? Math.min(100, Math.round((prog.done / prog.total) * 100)) : 0;
  const fileName = path.split("/").pop() || info.assetName;

  return (
    <Portal>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 backdrop-blur-sm sm:p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget && !busy) onClose();
        }}
      >
        <div
          role="dialog"
          aria-modal="true"
          aria-label={`새 버전 v${info.latest}`}
          className="surface flex max-h-[88vh] w-full max-w-xl flex-col gap-4 p-5 sm:p-6"
        >
          <div className="flex items-start gap-3">
            <span className="icon-tile icon-tile-mint !h-9 !w-9 shrink-0 !rounded-xl" aria-hidden="true">
              <Sparkles className="h-[18px] w-[18px]" strokeWidth={1.75} />
            </span>
            <div className="min-w-0 flex-1">
              <h3 className="section-title">새 버전 v{info.latest}</h3>
              <p className="num mt-1 text-sm text-zinc-400">
                지금 v{info.current} · 바뀌는 점을 확인하고 내려받으세요
              </p>
            </div>
            <button type="button" className="btn-icon" aria-label="닫기" onClick={onClose} disabled={busy}>
              <X className="h-4 w-4" />
            </button>
          </div>

          {/* 바뀌는 점: 지금 버전 이후의 릴리스를 최신부터 */}
          <div className="surface-inset min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
            {info.notes.map((n) => (
              <section key={n.version}>
                <p className="num mb-1.5 flex items-baseline gap-2 text-sm font-medium text-white">
                  v{n.version}
                  {n.title && n.title.replace(/^v/, "") !== n.version && (
                    <span className="font-normal text-zinc-300">{n.title}</span>
                  )}
                  <span className="label-mono !normal-case">{n.date}</span>
                </p>
                {n.notes ? <Markdown text={n.notes} /> : <p className="text-sm text-zinc-500">설명이 없습니다.</p>}
              </section>
            ))}
          </div>

          {phase === "downloading" && (
            <div>
              <div className="h-2 overflow-hidden rounded-full bg-white/[0.06]">
                <div className="h-full rounded-full bg-mint-400 transition-[width]" style={{ width: `${pct}%` }} />
              </div>
              <p className="num mt-1.5 text-xs text-zinc-400">
                내려받는 중 {mb(prog.done)}
                {prog.total > 0 && ` / ${mb(prog.total)} (${pct}%)`}
              </p>
            </div>
          )}

          {phase === "done" && (
            <div className="surface-inset space-y-1.5 px-4 py-3 text-xs leading-relaxed text-zinc-300">
              <p>
                다운로드 폴더에 받았습니다: <span className="text-white">{fileName}</span>
              </p>
              <p className="text-zinc-400">1. [설치 파일 열기]를 누르면 열리는 창에서 sobi.app 을 Applications 로 끌어다 놓고 '대치'를 고릅니다.</p>
              <p className="text-zinc-400">2. 이 앱을 종료하고 다시 열면 새 버전으로 바뀝니다.</p>
            </div>
          )}

          {err && <p className="text-sm text-rose-300">{err}</p>}

          <div className="flex flex-wrap items-center justify-end gap-2">
            {!info.assetName ? (
              <>
                <p className="mr-auto text-xs text-zinc-500">이 컴퓨터용 설치 파일이 릴리스에 없습니다.</p>
                <button type="button" className="btn-ghost" onClick={onClose}>
                  닫기
                </button>
                <button type="button" className="btn-primary" onClick={() => BrowserOpenURL(info.pageUrl)}>
                  릴리스 페이지 열기
                </button>
              </>
            ) : phase === "done" ? (
              <>
                <button type="button" className="btn-ghost" onClick={onClose}>
                  닫기
                </button>
                <button type="button" className="btn-primary" onClick={openFile}>
                  <PackageOpen className="h-4 w-4" />
                  설치 파일 열기
                </button>
              </>
            ) : (
              <>
                <button type="button" className="btn-ghost" onClick={onClose} disabled={busy}>
                  나중에
                </button>
                <button type="button" className="btn-primary" onClick={download} disabled={busy}>
                  {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                  {phase === "error" ? "다시 내려받기" : "내려받기"}
                  {info.assetSize > 0 && <span className="num opacity-70">{mb(info.assetSize)}</span>}
                </button>
              </>
            )}
          </div>
        </div>
      </div>
    </Portal>
  );
}

// 릴리스 설명(마크다운)을 간단히 보여 준다: 제목(#), 목록(-, *, 1.), 문단. 강조 기호는 떼어 낸다.
function Markdown({ text }: { text: string }) {
  const out: ReactNode[] = [];
  let list: string[] = [];
  const inline = (s: string) => s.replace(/\*\*(.+?)\*\*/g, "$1").replace(/__(.+?)__/g, "$1").replace(/`([^`]+)`/g, "$1");
  const flush = () => {
    if (list.length === 0) return;
    out.push(
      <ul key={`l${out.length}`} className="list-disc space-y-0.5 pl-5">
        {list.map((l, i) => (
          <li key={i}>{inline(l)}</li>
        ))}
      </ul>
    );
    list = [];
  };
  for (const raw of text.replace(/\r/g, "").split("\n")) {
    const line = raw.trim();
    if (!line) {
      flush();
      continue;
    }
    const item = line.match(/^(?:[-*+]|\d+[.)])\s+(.*)$/);
    if (item) {
      list.push(item[1]);
      continue;
    }
    flush();
    const heading = line.match(/^#{1,6}\s+(.*)$/);
    out.push(
      heading ? (
        <p key={`h${out.length}`} className="pt-1 text-xs font-medium text-zinc-100">
          {inline(heading[1])}
        </p>
      ) : (
        <p key={`p${out.length}`}>{inline(line)}</p>
      )
    );
  }
  flush();
  return <div className="space-y-1.5 text-sm leading-relaxed text-zinc-300">{out}</div>;
}
