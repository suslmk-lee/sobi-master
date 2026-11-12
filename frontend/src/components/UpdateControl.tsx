import { type ReactNode, useCallback, useEffect, useState } from "react";
import { Download, Loader2, PackageOpen, RotateCw, Sparkles, X } from "lucide-react";
import {
  CheckForUpdate,
  DownloadUpdate,
  GetAppVersion,
  InstallUpdate,
  OpenDownloadedUpdate,
  TakeUpdateNotice,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../wailsjs/runtime/runtime";
import Portal from "./ui/Portal";
import IconTile from "./ui/IconTile";

// 헤더의 버전 표시와 새 버전 알림.
// - 평소: 지금 버전(v1.0.0). 누르면 바로 업데이트를 확인한다.
// - 새 버전이 있으면: '새 버전 받기' 버튼. 누르면 바뀌는 점을 먼저 보여 주고 업데이트를 고를 수 있다.
//   macOS 에서는 [설치하고 다시 시작] 한 번으로 내려받기·설치·재시작까지 앱이 한다(selfupdate.go).
//   자동 설치를 못 하는 경우(또는 지난 자동 설치가 실패했으면) 설치 파일을 열어 직접 옮기게 안내한다.
// - 업데이트 뒤 처음 켜지면 버전 칩에 결과를 잠깐 보여 준다.
// 앱이 켜지고 몇 초 뒤, 그리고 6시간마다 조용히 확인한다(실패해도 알리지 않는다).

const CHECK_DELAY_MS = 3000;
const CHECK_EVERY_MS = 6 * 60 * 60 * 1000;

type Note = { text: string; tone?: "ok" | "warn"; ms: number };

const mb = (bytes: number) => `${(bytes / 1024 / 1024).toFixed(bytes >= 100 * 1024 * 1024 ? 0 : 1)}MB`;

export default function UpdateControl() {
  const [version, setVersion] = useState("");
  const [info, setInfo] = useState<main.UpdateInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [checking, setChecking] = useState(false);
  const [note, setNote] = useState<Note | null>(null); // 버전 칩 자리에 잠깐 보이는 한 줄 결과
  const [manual, setManual] = useState(false); // 지난 자동 설치가 실패함 → 이번엔 직접 설치로 안내

  useEffect(() => {
    GetAppVersion()
      .then(setVersion)
      .catch(() => {});
    TakeUpdateNotice()
      .then((n) => {
        if (n.kind === "done") setNote({ text: `v${n.to}으로 업데이트했습니다`, tone: "ok", ms: 8000 });
        if (n.kind === "failed") {
          setNote({ text: "업데이트하지 못했습니다", tone: "warn", ms: 8000 });
          setManual(true);
        }
      })
      .catch(() => {});
  }, []);

  const check = useCallback(async (manual: boolean) => {
    if (manual) {
      setChecking(true);
      setNote(null);
    }
    try {
      const r = await CheckForUpdate();
      setInfo(r);
      if (manual) {
        if (r.available) setOpen(true);
        else setNote({ text: "최신 버전입니다", ms: 3500 });
      }
    } catch (e) {
      if (manual) setNote({ text: "확인하지 못했습니다", ms: 3500 });
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
    const t = window.setTimeout(() => setNote(null), note.ms);
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
        className={`glass-pill num flex h-9 min-w-[3.5rem] items-center justify-center px-3 text-xs transition-colors hover:text-white ${
          note?.tone === "ok" ? "text-mint-300" : note?.tone === "warn" ? "text-amber-300" : "text-zinc-400"
        }`}
      >
        {checking ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : note?.text || (version ? `v${version}` : "")}
      </button>
      {open && info && <UpdateModal info={info} manual={manual} onClose={() => setOpen(false)} />}
    </>
  );
}

// idle → downloading → (자동 설치) installing → restarting: 앱이 종료되고 새 버전이 열린다
//                      → (직접 설치) done: 설치 파일 열기 안내
// 내려받기에 실패하면 error 에서 다시 시도, 설치 준비에 실패하면 done 으로 넘어가 직접 설치를 안내한다.
type Phase = "idle" | "downloading" | "installing" | "restarting" | "done" | "error";

function UpdateModal({ info, manual, onClose }: { info: main.UpdateInfo; manual: boolean; onClose: () => void }) {
  const auto = info.autoInstall && !manual;
  const [phase, setPhase] = useState<Phase>("idle");
  const [prog, setProg] = useState({ done: 0, total: info.assetSize });
  const [path, setPath] = useState("");
  const [err, setErr] = useState("");
  const busy = phase === "downloading" || phase === "installing" || phase === "restarting";

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

  const start = async () => {
    setErr("");
    let file = path;
    if (!file) {
      setPhase("downloading");
      setProg({ done: 0, total: info.assetSize });
      try {
        file = await DownloadUpdate();
        setPath(file);
      } catch (e) {
        setErr(String(e));
        setPhase("error");
        return;
      }
    }
    if (!auto) {
      setPhase("done");
      return;
    }
    setPhase("installing");
    try {
      await InstallUpdate();
      setPhase("restarting");
    } catch (e) {
      setErr(`자동으로 설치하지 못했습니다: ${String(e)}`);
      setPhase("done");
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
            <IconTile icon={Sparkles} tone="mint" size="md" />
            <div className="min-w-0 flex-1">
              <h3 className="section-title">새 버전 v{info.latest}</h3>
              <p className="num mt-1 text-sm text-zinc-400">
                지금 v{info.current} · 바뀌는 점을 확인하고 {auto ? "업데이트하세요" : "내려받으세요"}
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

          {(phase === "installing" || phase === "restarting") && (
            <p className="flex items-center gap-2 text-sm text-zinc-300">
              <Loader2 className="h-4 w-4 animate-spin text-mint-300" />
              {phase === "installing"
                ? "설치 파일을 확인하고 새 버전을 준비하는 중…"
                : `앱을 닫고 v${info.latest}로 다시 엽니다. 잠시만 기다려 주세요.`}
            </p>
          )}

          {phase === "done" && (
            <div className="surface-inset space-y-1.5 px-4 py-3 text-xs leading-relaxed text-zinc-300">
              <p>
                {auto ? "받은 설치 파일" : "다운로드 폴더에 받았습니다"}: <span className="text-white">{fileName}</span>
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
            ) : phase === "installing" || phase === "restarting" ? null : phase === "done" ? (
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
                {auto && phase === "idle" && (
                  <p className="mr-auto text-xs text-zinc-500">내려받아 설치하고 앱이 저절로 다시 열립니다.</p>
                )}
                <button type="button" className="btn-primary" onClick={start} disabled={busy}>
                  {busy ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : auto ? (
                    <RotateCw className="h-4 w-4" />
                  ) : (
                    <Download className="h-4 w-4" />
                  )}
                  {auto ? (phase === "error" ? "다시 시도" : "설치하고 다시 시작") : phase === "error" ? "다시 내려받기" : "내려받기"}
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
