// 백엔드(Wails 바인딩) 호출이 진행 중인지 앱 전체에서 알 수 있게 한다.
// 생성된 바인딩(wailsjs/go/main/App.js)은 호출할 때마다 window.go.main.App[이름] 을 찾아 부르므로,
// 그 함수들을 한 번 감싸 두면 어느 화면의 호출이든 따로 손대지 않고 셀 수 있다.
import { useEffect, useState } from "react";

type Listener = (pending: number) => void;

let pending = 0;
const listeners = new Set<Listener>();
const emit = () => listeners.forEach((l) => l(pending));

type AppBindings = Record<string, unknown> & { __busyTracked?: boolean };

// 뒤에서 조용히 도는 호출(업데이트 확인·내려받기)은 화면 로딩 표시에 넣지 않는다
const UNTRACKED = new Set([
  "GetAppVersion",
  "CheckForUpdate",
  "DownloadUpdate",
  "OpenDownloadedUpdate",
  "InstallUpdate",
  "TakeUpdateNotice",
  "GetAlerts",
]);

function wrap(app: AppBindings) {
  if (app.__busyTracked) return;
  for (const key of Object.keys(app)) {
    const fn = app[key];
    if (typeof fn !== "function" || UNTRACKED.has(key)) continue;
    app[key] = (...args: unknown[]) => {
      pending++;
      emit();
      const done = () => {
        pending--;
        emit();
      };
      let result: unknown;
      try {
        result = (fn as (...a: unknown[]) => unknown)(...args);
      } catch (e) {
        done();
        throw e;
      }
      return Promise.resolve(result).finally(done);
    };
  }
  app.__busyTracked = true;
}

// 앱 시작 시 한 번 부른다. 바인딩이 아직 주입되지 않았으면(웹 미리보기 등) 로드 뒤 다시 시도한다.
export function trackBackendCalls() {
  const app = (window as unknown as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
  if (app) {
    wrap(app);
    return;
  }
  window.addEventListener("load", () => {
    const later = (window as unknown as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
    if (later) wrap(later);
  }, { once: true });
}

// 호출이 delay(ms) 넘게 이어질 때만 true — 빠른 호출마다 표시가 깜빡이지 않게 한다.
export function useBackendBusy(delay = 150) {
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let timer: number | undefined;
    const on = (n: number) => {
      if (n > 0) {
        if (timer === undefined) timer = window.setTimeout(() => setBusy(true), delay);
      } else {
        if (timer !== undefined) window.clearTimeout(timer);
        timer = undefined;
        setBusy(false);
      }
    };
    listeners.add(on);
    on(pending);
    return () => {
      listeners.delete(on);
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [delay]);
  return busy;
}
