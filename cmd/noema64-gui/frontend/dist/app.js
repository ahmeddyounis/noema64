const files = ["a", "b", "c", "d", "e", "f", "g", "h"];
const ranks = ["1", "2", "3", "4", "5", "6", "7", "8"];
const pieces = {
  K: "♔", Q: "♕", R: "♖", B: "♗", N: "♘", P: "♙",
  k: "♚", q: "♛", r: "♜", b: "♝", n: "♞", p: "♟"
};

let state = null;
let selected = null;
let flipped = false;
let activeTab = "summary";
let settings = null;
let playerSide = "white";
let autoReply = true;
let pendingPromotion = null;

const timeControlPresets = {
  untimed: { initial_ms: 0, increment_ms: 0 },
  bullet: { initial_ms: 60000, increment_ms: 0 },
  blitz: { initial_ms: 300000, increment_ms: 0 },
  rapid: { initial_ms: 600000, increment_ms: 0 },
  classical: { initial_ms: 1800000, increment_ms: 0 }
};

const api = () => window.go?.appsvc?.Application;

function appErrorMessage(err) {
  if (!err) return "Unknown error";
  if (typeof err === "string") return err;
  if (err.message) return err.message;
  if (err.Message) return err.Message;
  return String(err);
}

function isAppErrorValue(value) {
  return value && typeof value === "object" && typeof value.message === "string" && typeof value.code === "string";
}

function showError(err, selector = "#statusText") {
  const target = document.querySelector(selector) || document.querySelector("#statusText");
  target.textContent = appErrorMessage(err);
}

async function call(name, ...args) {
  const svc = api();
  if (!svc || !svc[name]) throw new Error("Wails bindings are not available");
  const result = await svc[name](...args);
  if (isAppErrorValue(result)) throw new Error(result.message);
  return result;
}

async function refresh() {
  try {
    state = await call("GetGame");
    render();
  } catch (err) {
    document.querySelector("#statusText").textContent = String(err);
  }
}

function render() {
  if (!state?.snapshot) return;
  renderBoard();
  renderStatus();
  renderMoves();
  renderStrategy();
  renderDecision();
}

function renderStatus() {
  const s = state.snapshot;
  document.querySelector("#statusText").textContent = `${s.side_to_move} to move · ${s.outcome.status} · ${s.fen}`;
  document.querySelector("#clockText").textContent = formatClock(state.clock);
  document.querySelector("#modeText").textContent = state.last_decision?.mode || settings?.engine?.default_mode || "blunderguard";
}

function formatClock(clock) {
  if (!clock?.enabled) return "Untimed";
  return `White ${formatClockMS(clock.white_ms)} · Black ${formatClockMS(clock.black_ms)} · +${Math.round((clock.increment_ms || 0) / 1000)}s`;
}

function formatClockMS(ms) {
  ms = Math.max(0, Number(ms) || 0);
  const totalSeconds = Math.ceil(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = `${totalSeconds % 60}`.padStart(2, "0");
  return `${minutes}:${seconds}`;
}

function squareOrder() {
  const fs = flipped ? [...files].reverse() : files;
  const rs = flipped ? ranks : [...ranks].reverse();
  const out = [];
  for (const r of rs) for (const f of fs) out.push(f + r);
  return out;
}

function renderBoard() {
  const board = document.querySelector("#board");
  board.innerHTML = "";
  const legalTargets = selected
    ? state.snapshot.legal_moves.filter((m) => m.from === selected).map((m) => m.to)
    : [];
  const last = state.snapshot.move_history.at(-1);
  for (const sq of squareOrder()) {
    const file = files.indexOf(sq[0]);
    const rank = ranks.indexOf(sq[1]);
    const div = document.createElement("button");
    div.className = `square ${(file + rank) % 2 === 0 ? "dark" : "light"}`;
    div.setAttribute("role", "gridcell");
    div.setAttribute("aria-label", `${sq} ${state.snapshot.board[sq] || "empty"}`);
    div.dataset.square = sq;
    if (selected === sq) div.classList.add("selected");
    if (legalTargets.includes(sq)) div.classList.add("target");
    if (last && (last.uci.startsWith(sq) || last.uci.slice(2, 4) === sq)) div.classList.add("last-move");
    div.textContent = pieces[state.snapshot.board[sq]] || "";
    const coord = document.createElement("span");
    coord.className = "coord";
    coord.textContent = sq;
    div.appendChild(coord);
    div.addEventListener("click", () => squareClicked(sq));
    board.appendChild(div);
  }
}

async function squareClicked(sq) {
  if (!state?.snapshot) return;
  if (!selected) {
    if (state.snapshot.board[sq]) selected = sq;
    renderBoard();
    return;
  }
  const matches = state.snapshot.legal_moves.filter((m) => m.from === selected && m.to === sq);
  if (!matches.length) {
    selected = state.snapshot.board[sq] ? sq : null;
    renderBoard();
    return;
  }
  let legal = matches[0];
  if (matches.length > 1) {
    legal = await choosePromotion(matches);
    if (!legal) {
      renderBoard();
      return;
    }
  }
  selected = null;
  await makeMove(legal.uci);
}

function choosePromotion(moves) {
  const dialog = document.querySelector("#promotionDialog");
  return new Promise((resolve) => {
    pendingPromotion = { moves, resolve };
    dialog.showModal();
  });
}

function finishPromotion(promotion) {
  if (!pendingPromotion) return;
  const { moves, resolve } = pendingPromotion;
  pendingPromotion = null;
  document.querySelector("#promotionDialog").close();
  resolve(moves.find((m) => m.promotion === promotion) || null);
}

function renderMoves() {
  const list = document.querySelector("#moveList");
  list.innerHTML = "";
  for (const move of state.snapshot.move_history) {
    const item = document.createElement("li");
    item.textContent = `${move.san} (${move.uci})`;
    list.appendChild(item);
  }
}

function renderStrategy() {
  const mem = state.strategy_memory || {};
  document.querySelector("#confidence").textContent = Number(mem.plan?.confidence || 0).toFixed(2);
  const dl = document.querySelector("#strategyMemory");
  dl.innerHTML = "";
  const rows = [
    ["Plan", mem.plan?.summary],
    ["Status", mem.plan?.status],
    ["Phase", mem.phase],
    ["Targets", [...(mem.targets?.squares || []), ...(mem.targets?.pieces || []), ...(mem.targets?.pawns || [])].join(", ")],
    ["Opponent", mem.opponent_model?.likely_plan],
    ["Warnings", (mem.tactical_warnings || []).join("; ")],
    ["Commitments", (mem.commitments || []).join("; ")],
    ["Triggers", (mem.refutation_triggers || []).map((t) => t.condition || t).join("; ")],
    ["Last", mem.last_update?.summary]
  ];
  for (const [label, value] of rows) {
    const dt = document.createElement("dt");
    dt.textContent = label;
    const dd = document.createElement("dd");
    dd.textContent = value || "None";
    dl.append(dt, dd);
  }
}

function renderDecision() {
  const dec = state.last_decision;
  document.querySelector("#thinkingStage").textContent = dec ? "Decision finished" : "Idle";
  document.querySelector("#tabContent").textContent = tabText(dec);
  const box = document.querySelector("#candidates");
  box.innerHTML = "";
  if (!dec?.candidate_moves?.length) {
    box.textContent = "Candidate moves will appear here while the engine is thinking.";
    return;
  }
  for (const c of dec.candidate_moves) {
    const div = document.createElement("div");
    div.className = "candidate";
    const move = document.createElement("strong");
    move.textContent = c.san || c.uci;
    const detail = document.createElement("span");
    detail.append(document.createTextNode(c.purpose || ""));
    detail.append(document.createElement("br"));
    const meta = document.createElement("small");
    meta.textContent = `confidence ${formatScore(c.confidence)} · plan ${formatScore(c.plan_alignment_score)} · verifier ${c.verifier_score?.status || "not_checked"}`;
    detail.append(meta);
    if (c.risk) {
      detail.append(document.createElement("br"));
      const risk = document.createElement("small");
      risk.textContent = c.risk;
      detail.append(risk);
    }
    const score = document.createElement("small");
    score.className = "candidate-score";
    score.textContent = `#${c.rank || "-"} · final ${formatScore(c.final_score)}`;
    div.append(move, detail, score);
    box.appendChild(div);
  }
}

function formatScore(value) {
  const score = Number(value);
  return Number.isFinite(score) ? score.toFixed(2) : "0.00";
}

function tabText(dec) {
  if (!dec) return "No strategy yet. Noema64 will create a plan after its first decision.";
  switch (activeTab) {
    case "diff": return JSON.stringify(dec.strategy_diff, null, 2);
    case "verifier": return JSON.stringify(dec.verifier_trace, null, 2);
    case "raw": return JSON.stringify(dec, null, 2);
    default:
      return `${dec.selected_move?.san || dec.selected_move?.uci}: ${dec.explanation}\n\n${dec.position_summary}\n\nFallback used: ${dec.fallback_used}`;
  }
}

async function makeMove(move) {
  try {
    document.querySelector("#thinkingStage").textContent = "Applying user move";
    state = await call("MakeUserMove", move);
    render();
    if (autoReply && state?.snapshot?.outcome?.status === "ongoing" && state.snapshot.side_to_move !== playerSide) {
      await askEngine();
    }
  } catch (err) {
    showError(err);
    document.querySelector("#thinkingStage").textContent = "Move failed";
  }
}

async function askEngine() {
  try {
    document.querySelector("#thinkingStage").textContent = "Thinking: provider, repair, verifier, scoring";
    const result = await call("RequestEngineMove");
    state = result.state;
    render();
  } catch (err) {
    showError(err);
    document.querySelector("#thinkingStage").textContent = "Engine stopped";
  }
}

async function resignGame() {
  if (!state?.snapshot || state.snapshot.outcome?.status !== "ongoing") return;
  const side = playerSide || state.snapshot.side_to_move || "white";
  if (!window.confirm(`Resign as ${side}?`)) return;
  try {
    state = await call("Resign", side);
    selected = null;
    render();
  } catch (err) {
    showError(err);
  }
}

async function loadSettings() {
  settings = await call("GetSettings");
  document.querySelector("#settingMode").value = settings.engine.default_mode;
  document.querySelector("#settingPersonality").value = settings.engine.personality;
  document.querySelector("#settingSide").value = playerSide;
  document.querySelector("#settingTimeControl").value = settings.gui?.time_control || "untimed";
  document.querySelector("#settingClockInitial").value = Math.max(1, Math.round((settings.gui?.clock_initial_ms || 300000) / 60000));
  document.querySelector("#settingClockIncrement").value = Math.max(0, Math.round((settings.gui?.clock_increment_ms || 0) / 1000));
  document.querySelector("#settingAutoReply").checked = autoReply;
  document.querySelector("#settingMaxCandidates").value = settings.engine.max_candidates || 5;
  document.querySelector("#settingProvider").value = settings.llm.provider;
  document.querySelector("#settingEndpoint").value = settings.llm.endpoint || "";
  document.querySelector("#settingModel").value = settings.llm.model || "";
  document.querySelector("#settingTimeout").value = settings.llm.timeout_ms || 12000;
  document.querySelector("#settingKey").value = settings.llm.api_key || "";
  document.querySelector("#settingCloudAck").checked = !!settings.privacy.cloud_provider_warning_acknowledged;
  document.querySelector("#settingVerifier").checked = !!settings.verifier.enabled;
  document.querySelector("#settingVerifierPath").value = settings.verifier.path || "";
  document.querySelector("#settingVerifierMoveTime").value = settings.verifier.movetime_ms || 100;
  document.querySelector("#settingVerifierMaxLoss").value = settings.verifier.max_centipawn_loss || 180;
  document.querySelector("#settingTraceEnabled").checked = !!settings.engine.trace_enabled;
  document.querySelector("#settingLogDir").value = settings.logging.output_dir || "logs";
  document.querySelector("#settingRaw").checked = !!settings.privacy.log_raw_prompts;
  document.querySelector("#settingRawResponses").checked = !!settings.privacy.log_raw_llm_responses;
  syncTimeControlInputsFromPreset(false);
  syncProviderDisclosure();
}

async function saveSettings() {
  try {
    playerSide = document.querySelector("#settingSide").value;
    if (playerSide === "random") playerSide = Math.random() < 0.5 ? "white" : "black";
    autoReply = document.querySelector("#settingAutoReply").checked;
    const timeControl = timeControlForNewGame();
    settings.engine.default_mode = document.querySelector("#settingMode").value;
    settings.engine.personality = document.querySelector("#settingPersonality").value;
    settings.engine.max_candidates = Number(document.querySelector("#settingMaxCandidates").value) || settings.engine.max_candidates;
    settings.gui.time_control = document.querySelector("#settingTimeControl").value;
    settings.gui.clock_initial_ms = timeControl.initial_ms || Number(document.querySelector("#settingClockInitial").value) * 60000 || settings.gui.clock_initial_ms;
    settings.gui.clock_increment_ms = timeControl.increment_ms || Number(document.querySelector("#settingClockIncrement").value) * 1000 || 0;
    settings.llm.provider = document.querySelector("#settingProvider").value;
    settings.llm.endpoint = document.querySelector("#settingEndpoint").value;
    settings.llm.model = document.querySelector("#settingModel").value;
    settings.llm.timeout_ms = Number(document.querySelector("#settingTimeout").value) || settings.llm.timeout_ms;
    settings.llm.api_key = document.querySelector("#settingKey").value;
    settings.privacy.cloud_provider_warning_acknowledged = document.querySelector("#settingCloudAck").checked;
    if (settings.llm.provider === "openai_compatible" && !settings.privacy.cloud_provider_warning_acknowledged) {
      document.querySelector("#settingsOutput").textContent = "Acknowledge cloud provider data sharing before saving.";
      return;
    }
    settings.verifier.enabled = document.querySelector("#settingVerifier").checked;
    settings.verifier.path = document.querySelector("#settingVerifierPath").value;
    settings.verifier.movetime_ms = Number(document.querySelector("#settingVerifierMoveTime").value) || settings.verifier.movetime_ms;
    settings.verifier.max_centipawn_loss = Number(document.querySelector("#settingVerifierMaxLoss").value) || settings.verifier.max_centipawn_loss;
    settings.engine.trace_enabled = document.querySelector("#settingTraceEnabled").checked;
    settings.logging.output_dir = document.querySelector("#settingLogDir").value || settings.logging.output_dir;
    settings.privacy.log_raw_prompts = document.querySelector("#settingRaw").checked;
    settings.privacy.log_raw_llm_responses = document.querySelector("#settingRawResponses").checked;
    await call("SaveSettings", settings);
    document.querySelector("#settingsOutput").textContent = "Settings saved.";
  } catch (err) {
    showError(err, "#settingsOutput");
  }
}

function syncProviderDisclosure() {
  const warning = document.querySelector("#cloudProviderWarning");
  const isCloud = document.querySelector("#settingProvider").value === "openai_compatible";
  warning.classList.toggle("hidden", !isCloud);
}

function syncTimeControlInputsFromPreset(overwrite) {
  const preset = document.querySelector("#settingTimeControl").value;
  if (preset === "custom") return;
  const tc = timeControlPresets[preset] || timeControlPresets.untimed;
  if (overwrite || preset === "untimed") {
    document.querySelector("#settingClockInitial").value = Math.max(1, Math.round((tc.initial_ms || 300000) / 60000));
    document.querySelector("#settingClockIncrement").value = Math.round((tc.increment_ms || 0) / 1000);
  }
}

function timeControlForNewGame() {
  const preset = document.querySelector("#settingTimeControl")?.value || settings?.gui?.time_control || "untimed";
  if (preset !== "custom") return timeControlPresets[preset] || timeControlPresets.untimed;
  return {
    initial_ms: numericInputMS("#settingClockInitial", 60000, settings?.gui?.clock_initial_ms || 0),
    increment_ms: numericInputMS("#settingClockIncrement", 1000, settings?.gui?.clock_increment_ms || 0)
  };
}

function numericInputMS(selector, multiplier, fallbackMS) {
  const raw = document.querySelector(selector)?.value;
  const value = Number(raw);
  if (Number.isFinite(value) && value >= 0) return Math.round(value * multiplier);
  return Math.max(0, Number(fallbackMS) || 0);
}

function formatSavedAt(value) {
  if (!value) return "Unknown time";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function gameRecordFields(record) {
  const gameID = record.game_id || record.GameID || "";
  const savedAt = record.saved_at || record.SavedAt || "";
  const stateValue = record.state || record.State || {};
  const snapshot = stateValue.snapshot || stateValue.Snapshot || {};
  return { gameID, savedAt, snapshot };
}

function renderRecentGames(records) {
  const list = document.querySelector("#recentList");
  list.innerHTML = "";
  if (!records?.length) {
    list.textContent = "No recent games.";
    return;
  }
  for (const record of records) {
    const { gameID, savedAt, snapshot } = gameRecordFields(record);
    const row = document.createElement("div");
    row.className = "recent-game";
    const detail = document.createElement("div");
    const title = document.createElement("strong");
    title.textContent = snapshot.ply ? `${snapshot.ply} plies · ${snapshot.side_to_move || "unknown"} to move` : "New game";
    const meta = document.createElement("small");
    meta.textContent = `${formatSavedAt(savedAt)} · ${snapshot.outcome?.status || "ongoing"}`;
    detail.append(title, meta);
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = "Load";
    button.addEventListener("click", async () => {
      try {
        state = await call("LoadRecentGame", gameID);
        selected = null;
        render();
        document.querySelector("#recentDialog").close();
      } catch (err) {
        showError(err, "#recentOutput");
      }
    });
    row.append(detail, button);
    list.appendChild(row);
  }
}

async function openRecentGames() {
  try {
    document.querySelector("#recentOutput").textContent = "";
    document.querySelector("#recentList").textContent = "Loading...";
    document.querySelector("#recentDialog").showModal();
    renderRecentGames(await call("RecentGames", 10));
  } catch (err) {
    showError(err, "#recentOutput");
  }
}

document.querySelectorAll(".tabs button").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".tabs button").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    activeTab = btn.dataset.tab;
    renderDecision();
  });
});

document.querySelector("#newGameBtn").addEventListener("click", async () => {
  try {
    let side = document.querySelector("#settingSide")?.value || playerSide;
    if (side === "random") side = Math.random() < 0.5 ? "white" : "black";
    playerSide = side;
    state = await call("NewGame", {
      side,
      mode: document.querySelector("#settingMode")?.value || "blunderguard",
      personality: document.querySelector("#settingPersonality")?.value || "balanced",
      time_control: timeControlForNewGame()
    });
    render();
    if (autoReply && side === "black") await askEngine();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#settingTimeControl").addEventListener("change", () => syncTimeControlInputsFromPreset(true));
document.querySelector("#settingProvider").addEventListener("change", syncProviderDisclosure);
document.querySelector("#recentBtn").addEventListener("click", openRecentGames);
document.querySelector("#engineBtn").addEventListener("click", askEngine);
document.querySelector("#stopBtn").addEventListener("click", async () => {
  try {
    await call("StopEngine");
    document.querySelector("#thinkingStage").textContent = "Stop requested";
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#resignBtn").addEventListener("click", resignGame);
document.querySelector("#undoBtn").addEventListener("click", async () => {
  try {
    state = await call("Undo", 1);
    render();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#flipBtn").addEventListener("click", () => { flipped = !flipped; renderBoard(); });
document.querySelector("#moveBtn").addEventListener("click", () => makeMove(document.querySelector("#moveInput").value.trim()));
document.querySelector("#settingsBtn").addEventListener("click", async () => {
  try {
    await loadSettings();
    document.querySelector("#settingsDialog").showModal();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#importBtn").addEventListener("click", () => {
  document.querySelector("#importOutput").textContent = "";
  document.querySelector("#importDialog").showModal();
});
document.querySelector("#runImportBtn").addEventListener("click", async () => {
  const type = document.querySelector("#importType").value;
  const text = document.querySelector("#importText").value;
  try {
    state = type === "fen" ? await call("ImportFEN", text) : await call("ImportPGN", text);
    selected = null;
    render();
    document.querySelector("#importDialog").close();
  } catch (err) {
    document.querySelector("#importOutput").textContent = String(err);
  }
});
document.querySelectorAll("#promotionDialog [data-promotion]").forEach((btn) => {
  btn.addEventListener("click", () => finishPromotion(btn.dataset.promotion));
});
document.querySelector("#promotionDialog").addEventListener("close", () => {
  if (!pendingPromotion) return;
  const { resolve } = pendingPromotion;
  pendingPromotion = null;
  resolve(null);
});
document.querySelector("#saveSettingsBtn").addEventListener("click", saveSettings);
document.querySelector("#healthBtn").addEventListener("click", async () => {
  try {
    document.querySelector("#settingsOutput").textContent = JSON.stringify(await call("HealthCheckProvider"), null, 2);
  } catch (err) {
    showError(err, "#settingsOutput");
  }
});
document.querySelector("#benchBtn").addEventListener("click", async () => {
  try {
    document.querySelector("#settingsOutput").textContent = "Running benchmark...";
    document.querySelector("#settingsOutput").textContent = JSON.stringify(await call("RunRandomBenchmark", 100, 64), null, 2);
  } catch (err) {
    showError(err, "#settingsOutput");
  }
});
async function refreshExport() {
  const type = document.querySelector("#exportType").value;
  document.querySelector("#exportText").value = type === "fen" ? await call("ExportFEN") : await call("ExportPGN");
}

document.querySelector("#refreshExportBtn").addEventListener("click", async () => {
  try {
    await refreshExport();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#exportType").addEventListener("change", async () => {
  try {
    await refreshExport();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#exportBtn").addEventListener("click", async () => {
  try {
    await refreshExport();
    document.querySelector("#exportDialog").showModal();
  } catch (err) {
    showError(err);
  }
});

window.addEventListener("keydown", (event) => {
  if (event.target.matches("input, textarea, select")) return;
  if (event.key === "n" || event.key === "N") document.querySelector("#newGameBtn").click();
  if (event.key === " ") { event.preventDefault(); askEngine(); }
  if (event.key === "r" || event.key === "R") document.querySelector("#resignBtn").click();
  if (event.key === "u" || event.key === "U") document.querySelector("#undoBtn").click();
  if (event.key === "f" || event.key === "F") document.querySelector("#flipBtn").click();
  if (event.key === ",") document.querySelector("#settingsBtn").click();
});

async function init() {
  try {
    await loadSettings();
  } catch (err) {
    settings = null;
  }
  await refresh();
}

init();
