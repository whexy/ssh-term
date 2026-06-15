import { WTerm } from "@wterm/dom";
import "@wterm/dom/css";

// ── DOM refs ──────────────────────────────────────────────────────────────────
const formView      = document.getElementById("form-view")!;
const terminalView  = document.getElementById("terminal-view")!;
const connectForm   = document.getElementById("connect-form") as HTMLFormElement;
const hostEl        = document.getElementById("host")         as HTMLInputElement;
const portEl        = document.getElementById("port")         as HTMLInputElement;
const usernameEl    = document.getElementById("username")     as HTMLInputElement;
const authMethodEl  = document.getElementById("auth-method")  as HTMLSelectElement;
const privateKeyEl  = document.getElementById("private-key")  as HTMLTextAreaElement;
const passwordEl    = document.getElementById("password")     as HTMLInputElement;
const authKeyDiv    = document.getElementById("auth-key")!;
const authPassDiv   = document.getElementById("auth-password")!;
const errorMsg      = document.getElementById("error-msg")!;
const connectBtn    = document.getElementById("connect-btn")  as HTMLButtonElement;
const connLabel     = document.getElementById("conn-label")!;
const disconnectBtn = document.getElementById("disconnect-btn")!;
const terminalEl    = document.getElementById("terminal-el")!;

// ── Persist username ──────────────────────────────────────────────────────────
const savedUser = localStorage.getItem("ssh:username");
if (savedUser) usernameEl.value = savedUser;

// ── Auth method toggle ────────────────────────────────────────────────────────
authMethodEl.addEventListener("change", () => {
  const isKey = authMethodEl.value === "privateKey";
  authKeyDiv.style.display  = isKey ? "" : "none";
  authPassDiv.style.display = isKey ? "none" : "";
});

// ── State ─────────────────────────────────────────────────────────────────────
let ws: WebSocket | null = null;
let term: WTerm | null = null;


function showError(msg: string) {
  errorMsg.textContent = msg;
  errorMsg.style.display = "block";
  connectBtn.disabled = false;
  connectBtn.textContent = "Connect";
}

function clearError() {
  errorMsg.style.display = "none";
  errorMsg.textContent = "";
}

// ── Connect ───────────────────────────────────────────────────────────────────
connectForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  clearError();
  connectBtn.disabled = true;
  connectBtn.textContent = "Connecting…";

  const host     = hostEl.value.trim();
  const port     = parseInt(portEl.value, 10) || 22;
  const username = usernameEl.value.trim();

  if (!host || !username) {
    showError("Host and username are required.");
    return;
  }

  localStorage.setItem("ssh:username", username);

  // ── Init terminal first so we know the actual cols/rows ───────────────────
  formView.style.display     = "none";
  terminalView.style.display = "flex";
  connLabel.textContent      = `${username}@${host}:${port}`;

  let resizeTimer: ReturnType<typeof setTimeout> | null = null;

  term = new WTerm(terminalEl, {
    onData: (data) => ws?.send(data),
    // Debounce: only send the final size after 150 ms of no further changes.
    // This prevents flooding vim/tmux with SIGWINCH during a window drag.
    onResize: (cols, rows) => {
      if (resizeTimer) clearTimeout(resizeTimer);
      resizeTimer = setTimeout(() => {
        if (ws?.readyState === WebSocket.OPEN)
          ws.send(`\x1b[RESIZE:${cols};${rows}]`);
      }, 150);
    },
  });
  await term.init();

  // ── Now open WebSocket — term.cols/rows are guaranteed correct ────────────
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  ws = new WebSocket(`${proto}//${location.host}/ws`);
  ws.binaryType = "arraybuffer";

  ws.onopen = () => {
    const params: Record<string, unknown> = {
      host, port, username,
      cols: term!.cols,
      rows: term!.rows,
    };
    if (authMethodEl.value === "privateKey") {
      params.privateKey = privateKeyEl.value.trim();
    } else {
      params.password = passwordEl.value;
    }
    ws!.send(JSON.stringify(params));
  };

  ws.onmessage = (event: MessageEvent) => {
    if (!term) return;
    if (event.data instanceof ArrayBuffer) {
      term.write(new Uint8Array(event.data as ArrayBuffer));
    } else {
      const data = event.data as string;
      try {
        const msg = JSON.parse(data);
        if (msg.error) { showError(msg.error); ws?.close(); return; }
      } catch { /* not JSON, regular terminal data */ }
      term.write(data);
    }
  };

  ws.onerror = () => showError("WebSocket connection failed.");

  ws.onclose = () => {
    if (term) term.write("\r\n\x1b[31m[disconnected]\x1b[0m\r\n");
    if (terminalView.style.display !== "none") {
      // leave terminal visible so user can read the disconnect message
    }
    ws = null;
  };

  term.focus();
});

// ── Disconnect ────────────────────────────────────────────────────────────────
disconnectBtn.addEventListener("click", () => {
  ws?.close();
  term?.destroy();
  term = null;

  terminalView.style.display  = "none";
  formView.style.display      = "";
  connectBtn.disabled         = false;
  connectBtn.textContent      = "Connect";
  clearError();
  // Clear terminal element so a fresh one is created next time
  terminalEl.innerHTML = "";
});
