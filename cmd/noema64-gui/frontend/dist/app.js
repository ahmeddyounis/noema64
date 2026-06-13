const allFiles = Array.from({ length: 26 }, (_, index) => String.fromCharCode(97 + index));
const pieces = {
  K: "♔", Q: "♕", R: "♖", B: "♗", N: "♘", P: "♙",
  k: "♚", q: "♛", r: "♜", b: "♝", n: "♞", p: "♟"
};

let state = null;
let selected = null;
let focusedSquare = "e2";
let boardKeyboardMode = false;
let flipped = false;
let activeTab = "summary";
let settings = null;
let playerSide = "white";
let autoReply = true;
let gameVariant = "standard";
let chess960Seed = 0;
let pendingPromotion = null;
let applyingProviderProfile = false;
let lastStageEvent = null;
let promptPack = null;
const busyControls = new Set();

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
  const message = appErrorMessage(err);
  if ("value" in target && (target.tagName === "TEXTAREA" || target.tagName === "INPUT")) {
    target.value = message;
  } else {
    target.textContent = message;
  }
  target.title = message;
}

async function call(name, ...args) {
  const svc = api();
  if (!svc || !svc[name]) throw new Error("Wails bindings are not available");
  const result = await svc[name](...args);
  if (isAppErrorValue(result)) throw new Error(result.message);
  return result;
}

function controlFrom(controlOrSelector) {
  return typeof controlOrSelector === "string" ? document.querySelector(controlOrSelector) : controlOrSelector;
}

function busyKey(controlOrSelector, control) {
  return typeof controlOrSelector === "string" ? controlOrSelector : control;
}

function setControlBusy(controlOrSelector, busy) {
  const control = controlFrom(controlOrSelector);
  if (!control) return;
  const key = busyKey(controlOrSelector, control);
  if (busy) {
    busyControls.add(key);
    control.disabled = true;
    control.setAttribute("aria-busy", "true");
  } else {
    busyControls.delete(key);
    control.disabled = false;
    control.removeAttribute("aria-busy");
  }
}

async function withBusyControl(controlOrSelector, action) {
  const control = controlFrom(controlOrSelector);
  const key = busyKey(controlOrSelector, control);
  if (!control || busyControls.has(key)) return undefined;
  setControlBusy(controlOrSelector, true);
  try {
    return await action();
  } finally {
    setControlBusy(controlOrSelector, false);
  }
}

function bindBusyButton(selector, action) {
  document.querySelector(selector).addEventListener("click", (event) => {
    event.preventDefault();
    withBusyControl(selector, () => action(event));
  });
}

function resetBoardEntry() {
  selected = null;
  document.querySelector("#moveInput").value = "";
}

function parseJSONField(selector, label) {
  const field = document.querySelector(selector);
  try {
    return JSON.parse(field.value);
  } catch (err) {
    field.focus();
    field.select?.();
    throw new Error(`${label} must be valid JSON. ${appErrorMessage(err)}`);
  }
}

function requireField(selector, message) {
  const field = document.querySelector(selector);
  const value = String(field?.value || "").trim();
  if (value) return value;
  field?.focus();
  throw new Error(message);
}

async function refresh() {
  try {
    state = await call("GetGame");
    render();
  } catch (err) {
    renderUnavailableState(appErrorMessage(err));
  }
}

function render() {
  if (!state?.snapshot) {
    renderUnavailableState("No game state is available.");
    return;
  }
  renderBoard();
  renderStatus();
  renderMoves();
  renderStrategy();
  renderDecision();
}

function renderUnavailableState(message) {
  selected = null;
  renderBoardEmpty("Board unavailable", "Open Noema64 through the desktop app to connect the game service.");
  const status = document.querySelector("#statusText");
  status.textContent = message || "Game service unavailable.";
  status.title = status.textContent;
  document.querySelector("#clockText").textContent = "Untimed";
  document.querySelector("#modeText").textContent = settings?.engine?.default_mode || "offline";
  document.querySelector("#thinkingStage").textContent = "Unavailable";
  const tab = document.querySelector("#tabContent");
  tab.textContent = "Decision traces will appear after the game service is connected and the engine makes a decision.";
  tab.classList.add("empty-copy");
  const candidates = document.querySelector("#candidates");
  candidates.textContent = "Candidate moves are unavailable until a game is loaded.";
  candidates.classList.add("empty-copy");
  document.querySelector("#confidence").textContent = "0.00";
  renderStrategyRows([
    ["Status", "Unavailable"],
    ["Plan", "Connect the desktop game service to load strategy memory."]
  ]);
  renderMoveListEmpty("No moves loaded.");
}

function renderBoardEmpty(title, detail) {
  const board = document.querySelector("#board");
  board.innerHTML = "";
  board.classList.add("board-empty-state");
  board.style.setProperty("--board-files", 8);
  board.style.setProperty("--board-ranks", 8);
  board.style.aspectRatio = "1 / 1";
  board.setAttribute("aria-rowcount", "8");
  board.setAttribute("aria-colcount", "8");
  const empty = document.createElement("div");
  empty.className = "board-empty";
  empty.textContent = detail ? `${title}. ${detail}` : title;
  board.appendChild(empty);
}

function renderStatus() {
  const s = state.snapshot;
  const variant = state.variant?.variant || "standard";
  const status = document.querySelector("#statusText");
  status.textContent = statusSummary(s, variant);
  status.title = statusDetail(s, variant);
  document.querySelector("#clockText").textContent = formatClock(state.clock);
  document.querySelector("#modeText").textContent = state.last_decision?.mode || settings?.engine?.default_mode || "blunderguard";
}

function statusSummary(snapshot, variant) {
  const parts = [
    `${snapshot.side_to_move || "unknown"} to move`,
    snapshot.outcome?.status || "unknown",
    variant || "standard",
    `ply ${snapshot.ply || 0}`
  ];
  return parts.join(" · ");
}

function statusDetail(snapshot, variant) {
  return [
    `${snapshot.side_to_move || "unknown"} to move`,
    snapshot.outcome?.status || "unknown",
    variant || "standard",
    snapshot.fen || "No FEN recorded"
  ].join(" · ");
}

function compactFen(fen) {
  const text = String(fen || "").trim();
  if (!text) return "";
  const core = text.split(/\s+/).slice(0, 6).join(" ");
  return core.length > 72 ? `${core.slice(0, 69)}...` : core;
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

function applyTheme(theme) {
  const value = theme && theme !== "system" ? theme : "";
  document.body.dataset.theme = value;
}

function boardDimensions() {
  const def = state?.variant?.board_definition || {};
  const width = clampInt(def.board_width, 1, 26, 8);
  const height = clampInt(def.board_height, 1, 99, 8);
  return {
    width,
    height,
    files: allFiles.slice(0, width),
    ranks: Array.from({ length: height }, (_, index) => String(index + 1))
  };
}

function clampInt(value, min, max, fallback) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.max(min, Math.min(max, Math.trunc(parsed)));
}

function squareOrder() {
  const dims = boardDimensions();
  const fs = flipped ? [...dims.files].reverse() : dims.files;
  const rs = flipped ? dims.ranks : [...dims.ranks].reverse();
  const out = [];
  for (const r of rs) for (const f of fs) out.push(f + r);
  return out;
}

function renderBoard() {
  const board = document.querySelector("#board");
  board.innerHTML = "";
  board.classList.remove("board-empty-state");
  const dims = boardDimensions();
  const order = squareOrder();
  if (!order.includes(focusedSquare)) focusedSquare = order[0] || "a1";
  board.style.setProperty("--board-files", dims.width);
  board.style.setProperty("--board-ranks", dims.height);
  board.style.aspectRatio = `${dims.width} / ${dims.height}`;
  board.setAttribute("aria-rowcount", String(dims.height));
  board.setAttribute("aria-colcount", String(dims.width));
  const legalTargets = selected
    ? state.snapshot.legal_moves.filter((m) => m.from === selected).map((m) => m.to)
    : [];
  const last = state.snapshot.move_history.at(-1);
  const lastSquares = splitUCIMoveSquares(last?.uci);
  for (const [index, sq] of order.entries()) {
    const parsed = parseBoardSquare(sq, dims);
    const file = dims.files.indexOf(parsed?.file);
    const rank = dims.ranks.indexOf(parsed?.rank);
    const div = document.createElement("button");
    div.className = `square ${(file + rank) % 2 === 0 ? "dark" : "light"}`;
    div.setAttribute("role", "gridcell");
    div.setAttribute("aria-rowindex", String(Math.floor(index / dims.width) + 1));
    div.setAttribute("aria-colindex", String((index % dims.width) + 1));
    div.setAttribute("aria-label", `${sq} ${state.snapshot.board[sq] || "empty"}`);
    div.dataset.square = sq;
    div.tabIndex = sq === focusedSquare ? 0 : -1;
    if (selected === sq) div.classList.add("selected");
    if (boardKeyboardMode && focusedSquare === sq) div.classList.add("keyboard-focus");
    if (legalTargets.includes(sq)) div.classList.add("target");
    if (lastSquares && (lastSquares.from === sq || lastSquares.to === sq)) div.classList.add("last-move");
    div.draggable = !!state.snapshot.board[sq] && state.snapshot.outcome?.status === "ongoing";
    div.textContent = pieceGlyph(state.snapshot.board[sq]);
    const coord = document.createElement("span");
    coord.className = "coord";
    coord.textContent = sq;
    div.appendChild(coord);
    div.addEventListener("click", () => squareClicked(sq, false));
    div.addEventListener("mousedown", () => { boardKeyboardMode = false; });
    div.addEventListener("focus", () => { focusedSquare = sq; });
    div.addEventListener("dragstart", (event) => dragStarted(event, sq));
    div.addEventListener("dragover", (event) => dragOver(event, sq));
    div.addEventListener("drop", (event) => dropOnSquare(event, sq));
    board.appendChild(div);
  }
  renderBoardOverlay(board);
}

function renderBoardOverlay(board) {
  const candidates = state?.last_decision?.candidate_moves || [];
  if (!candidates.length) return;
  const overlay = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  overlay.setAttribute("class", "board-overlay");
  overlay.setAttribute("viewBox", "0 0 100 100");
  overlay.setAttribute("aria-hidden", "true");
  for (const [index, candidate] of candidates.slice(0, 4).entries()) {
    const move = candidate.legal_move || candidate;
    const parsed = splitUCIMoveSquares(candidate.uci || move.uci);
    const from = move.from || parsed?.from;
    const to = move.to || parsed?.to;
    const start = squareCenter(from);
    const end = squareCenter(to);
    if (!start || !end) continue;
    const line = document.createElementNS("http://www.w3.org/2000/svg", "line");
    const head = document.createElementNS("http://www.w3.org/2000/svg", "polygon");
    const geometry = arrowGeometry(start, end);
    if (!geometry) continue;
    line.setAttribute("x1", geometry.lineStart.x);
    line.setAttribute("y1", geometry.lineStart.y);
    line.setAttribute("x2", geometry.lineEnd.x);
    line.setAttribute("y2", geometry.lineEnd.y);
    line.setAttribute("class", `candidate-arrow${index ? " alt" : ""}`);
    head.setAttribute("points", geometry.headPoints);
    head.setAttribute("class", `candidate-arrow-head${index ? " alt" : ""}`);
    overlay.appendChild(line);
    overlay.appendChild(head);
  }
  board.appendChild(overlay);
}

function arrowGeometry(start, end) {
  const dx = end.x - start.x;
  const dy = end.y - start.y;
  const length = Math.hypot(dx, dy);
  if (length < 2) return null;
  const ux = dx / length;
  const uy = dy / length;
  const headLength = Math.min(4.6, length * 0.32);
  const headWidth = Math.min(2.6, length * 0.18);
  const startPad = Math.min(3.6, length * 0.18);
  const lineEnd = {
    x: end.x - ux * headLength * 0.72,
    y: end.y - uy * headLength * 0.72
  };
  const base = {
    x: end.x - ux * headLength,
    y: end.y - uy * headLength
  };
  const px = -uy;
  const py = ux;
  return {
    lineStart: {
      x: start.x + ux * startPad,
      y: start.y + uy * startPad
    },
    lineEnd,
    headPoints: [
      `${end.x.toFixed(2)},${end.y.toFixed(2)}`,
      `${(base.x + px * headWidth).toFixed(2)},${(base.y + py * headWidth).toFixed(2)}`,
      `${(base.x - px * headWidth).toFixed(2)},${(base.y - py * headWidth).toFixed(2)}`
    ].join(" ")
  };
}

function squareCenter(square) {
  if (!square) return null;
  const dims = boardDimensions();
  const order = squareOrder();
  const index = order.indexOf(square);
  if (index < 0) return null;
  const col = index % dims.width;
  const row = Math.floor(index / dims.width);
  return { x: (col + 0.5) * (100 / dims.width), y: (row + 0.5) * (100 / dims.height) };
}

async function squareClicked(sq, fromKeyboard = false) {
  if (!state?.snapshot) return;
  boardKeyboardMode = !!fromKeyboard;
  if (!selected) {
    if (state.snapshot.board[sq]) selected = sq;
    renderBoard();
    return;
  }
  await playFromTo(selected, sq);
}

async function playFromTo(from, to) {
  const matches = state.snapshot.legal_moves.filter((m) => m.from === from && m.to === to);
  if (!matches.length) {
    selected = state.snapshot.board[to] ? to : null;
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

function dragStarted(event, sq) {
  if (!state?.snapshot?.board?.[sq] || state.snapshot.outcome?.status !== "ongoing") {
    event.preventDefault();
    return;
  }
  selected = sq;
  event.dataTransfer.setData("text/plain", sq);
  event.dataTransfer.effectAllowed = "move";
}

function dragOver(event, sq) {
  if (!selected) return;
  if (state.snapshot.legal_moves.some((m) => m.from === selected && m.to === sq)) {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }
}

async function dropOnSquare(event, sq) {
  event.preventDefault();
  const from = event.dataTransfer.getData("text/plain") || selected;
  if (!from) return;
  await playFromTo(from, sq);
}

function handleBoardKeyboard(event) {
  const active = document.activeElement;
  if (!active?.closest?.("#board")) return false;
  boardKeyboardMode = true;
  const delta = {
    ArrowLeft: [-1, 0],
    ArrowRight: [1, 0],
    ArrowUp: [0, 1],
    ArrowDown: [0, -1]
  }[event.key];
  if (delta) {
    event.preventDefault();
    moveBoardFocus(delta[0], flipped ? -delta[1] : delta[1]);
    return true;
  }
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    squareClicked(focusedSquare, true);
    return true;
  }
  if (event.key === "Escape") {
    selected = null;
    renderBoard();
    return true;
  }
  return false;
}

function moveBoardFocus(df, dr) {
  boardKeyboardMode = true;
  const dims = boardDimensions();
  const current = parseBoardSquare(focusedSquare, dims) || { file: dims.files[0], rank: dims.ranks[0] };
  const file = Math.max(0, Math.min(dims.width - 1, dims.files.indexOf(current.file) + (flipped ? -df : df)));
  const rank = Math.max(0, Math.min(dims.height - 1, dims.ranks.indexOf(current.rank) + dr));
  focusedSquare = dims.files[file] + dims.ranks[rank];
  renderBoard();
  document.querySelector(`[data-square="${focusedSquare}"]`)?.focus();
}

function parseBoardSquare(square, dims = boardDimensions()) {
  if (!square || typeof square !== "string" || square.length < 2) return null;
  const file = square[0];
  const rank = square.slice(1);
  if (!dims.files.includes(file) || !dims.ranks.includes(rank)) return null;
  return { file, rank, square };
}

function splitUCIMoveSquares(uci) {
  if (!uci || typeof uci !== "string") return null;
  const dims = boardDimensions();
  const from = readUCISquare(uci, 0, dims);
  if (!from) return null;
  const to = readUCISquare(uci, from.next, dims);
  if (!to) return null;
  return { from: from.square, to: to.square };
}

function readUCISquare(text, start, dims) {
  const file = text[start];
  if (!dims.files.includes(file)) return null;
  let index = start + 1;
  while (index < text.length && /[0-9]/.test(text[index])) index += 1;
  const rank = text.slice(start + 1, index);
  if (!dims.ranks.includes(rank)) return null;
  return { square: file + rank, next: index };
}

function pieceGlyph(piece) {
  if (!piece) return "";
  return pieces[piece] || piece;
}

function choosePromotion(moves) {
  const dialog = document.querySelector("#promotionDialog");
  renderPromotionChoices(moves);
  return new Promise((resolve) => {
    pendingPromotion = { moves, resolve };
    dialog.showModal();
  });
}

function renderPromotionChoices(moves) {
  const grid = document.querySelector("#promotionGrid");
  grid.innerHTML = "";
  const seen = new Set();
  for (const move of moves) {
    const promotion = move.promotion;
    if (!promotion || seen.has(promotion)) continue;
    seen.add(promotion);
    const button = document.createElement("button");
    button.type = "button";
    button.dataset.promotion = promotion;
    button.title = promotionTitle(promotion);
    button.textContent = promotionGlyph(promotion);
    button.addEventListener("click", () => finishPromotion(promotion));
    grid.appendChild(button);
  }
}

function promotionGlyph(promotion) {
  const symbol = String(promotion || "").trim();
  if (!symbol) return "";
  return pieceGlyph(symbol.toUpperCase()) || symbol.toUpperCase();
}

function promotionTitle(promotion) {
  switch (String(promotion || "").toLowerCase()) {
    case "q":
      return "Queen";
    case "r":
      return "Rook";
    case "b":
      return "Bishop";
    case "n":
      return "Knight";
    default:
      return `Promote to ${String(promotion || "").toUpperCase()}`;
  }
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
  list.classList.remove("move-list-empty");
  if (!state.snapshot.move_history.length) {
    renderMoveListEmpty("No moves yet.");
    return;
  }
  for (const move of state.snapshot.move_history) {
    const item = document.createElement("li");
    item.textContent = `${move.san} (${move.uci})`;
    list.appendChild(item);
  }
}

function renderMoveListEmpty(message) {
  const list = document.querySelector("#moveList");
  list.innerHTML = "";
  list.classList.add("move-list-empty");
  const item = document.createElement("li");
  item.textContent = message;
  list.appendChild(item);
}

function renderStrategy() {
  const mem = state.strategy_memory || {};
  const metrics = state.strategy_metrics || {};
  document.querySelector("#confidence").textContent = Number(mem.plan?.confidence || 0).toFixed(2);
  const rows = [
    ["Plan", mem.plan?.summary],
    ["Status", mem.plan?.status],
    ["Phase", mem.phase],
    ["Quality", formatMetric(metrics.quality)],
    ["Drift", `${formatMetric(metrics.drift)} · ${metrics.alert_level || "none"}`],
    ["Alerts", formatStrategyAlerts(metrics.alerts)],
    ["Targets", [...(mem.targets?.squares || []), ...(mem.targets?.pieces || []), ...(mem.targets?.pawns || [])].join(", ")],
    ["Opponent", mem.opponent_model?.likely_plan],
    ["Warnings", (mem.tactical_warnings || []).join("; ")],
    ["Commitments", (mem.commitments || []).join("; ")],
    ["Triggers", (mem.refutation_triggers || []).map((t) => t.condition || t).join("; ")],
    ["Last", mem.last_update?.summary]
  ];
  renderStrategyRows(rows);
}

function renderStrategyRows(rows) {
  const dl = document.querySelector("#strategyMemory");
  dl.innerHTML = "";
  for (const [label, value] of rows) {
    const dt = document.createElement("dt");
    dt.textContent = label;
    const dd = document.createElement("dd");
    dd.textContent = value || "None";
    dl.append(dt, dd);
  }
}

function formatMetric(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n.toFixed(2) : "0.00";
}

function formatStrategyAlerts(alerts) {
  if (!alerts?.length) return "None";
  return alerts.map(formatStrategyAlert).filter(Boolean).join("; ") || "None";
}

function formatStrategyAlert(alert) {
  if (typeof alert === "string") return alert;
  if (!alert || typeof alert !== "object") return "";
  const severity = alert.severity || "info";
  const message = alert.message || humanizeToken(alert.code) || "Strategy alert";
  return `${severity}: ${message}`;
}

function humanizeToken(value) {
  return String(value || "")
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

function renderDecision() {
  const dec = state?.last_decision;
  document.querySelector("#thinkingStage").textContent = dec ? stageStatusText(lastDecisionStage(dec), "Decision finished") : "Idle";
  const tab = document.querySelector("#tabContent");
  tab.textContent = tabText(dec);
  tab.classList.toggle("empty-copy", !dec);
  const box = document.querySelector("#candidates");
  box.innerHTML = "";
  if (!dec?.candidate_moves?.length) {
    box.textContent = "Candidate moves will appear here while the engine is thinking.";
    box.classList.add("empty-copy");
    return;
  }
  box.classList.remove("empty-copy");
  for (const c of dec.candidate_moves) {
    const div = document.createElement("div");
    div.className = c.rank === 1 ? "candidate top-candidate" : "candidate";
    const move = document.createElement("strong");
    move.textContent = c.san || c.uci;
    const detail = document.createElement("span");
    detail.append(document.createTextNode(candidatePurpose(c)));
    detail.append(document.createElement("br"));
    const meta = document.createElement("small");
    meta.textContent = `conf ${formatScore(c.confidence)} · plan ${formatScore(c.plan_alignment_score)} · style ${formatScore(c.personality_score)} · search ${formatScore(c.search_score)} · verifier ${c.verifier_score?.status || "not_checked"}`;
    detail.append(meta);
    if (c.risk) {
      detail.append(document.createElement("br"));
      const risk = document.createElement("small");
      risk.textContent = c.risk;
      detail.append(risk);
    }
    const score = document.createElement("small");
    score.className = "candidate-score";
    score.textContent = `#${c.rank || "-"} ${formatScore(c.final_score)}`;
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
    case "prompt": return promptInspectorText(dec);
    case "raw": return JSON.stringify(dec, null, 2);
    default:
      return decisionSummaryText(dec);
  }
}

function decisionSummaryText(dec) {
  const move = dec.selected_move?.san || dec.selected_move?.uci || "Selected move";
  const explanation = dec.explanation || candidatePurpose((dec.candidate_moves || [])[0], "Explanation unavailable.");
  const position = dec.position_summary || "Position summary unavailable.";
  const fallback = dec.fallback_used ? `Yes${dec.fallback_reason ? `, ${dec.fallback_reason}` : ""}` : "No";
  return `${move}: ${explanation}\n\n${position}\n\nFallback used: ${fallback}\n\n${stageSummary(dec)}`;
}

function candidatePurpose(candidate, fallback = "No candidate rationale recorded.") {
  return candidate?.purpose || candidate?.rationale || candidate?.explanation || candidate?.expected_reply || fallback;
}

function promptInspectorText(dec) {
  const provider = dec.provider || {};
  const prompt = provider.raw_prompt || {};
  const lines = [
    `Prompt ID: ${provider.prompt_id || "unknown"}`,
    `Prompt version: ${provider.prompt_version || "unknown"}`,
    `Prompt schema: ${provider.prompt_schema_version || "unknown"}`,
    `Decision schema: ${provider.decision_schema_version || dec.schema_version || "unknown"}`,
    `Provider: ${provider.name || "unknown"}`,
    `Model: ${provider.model || "unknown"}`,
    `Parse status: ${provider.parse_status || "unknown"}`,
    "",
    "SYSTEM PROMPT",
    prompt.system || "Raw prompt logging is disabled.",
    "",
    "USER PROMPT",
    prompt.user || "Raw prompt logging is disabled.",
    "",
    "RAW LLM RESPONSE",
    provider.raw_response || "Raw LLM response logging is disabled.",
    "",
    "PARSED PROVIDER JSON",
    JSON.stringify(provider.parsed_decision || {}, null, 2),
    "",
    "STRATEGY MEMORY AFTER",
    JSON.stringify(dec.strategy_after || {}, null, 2)
  ];
  return lines.join("\n");
}

function lastDecisionStage(dec) {
  return dec?.stages?.length ? dec.stages[dec.stages.length - 1] : null;
}

function stageSummary(dec) {
  if (!dec?.stages?.length) return "Stages: not recorded";
  return `Stages:\n${dec.stages.map((stage) => `${stageLabel(stage.name)} · ${stage.status || "unknown"} · ${stage.duration_ms || 0} ms`).join("\n")}`;
}

function stageStatusText(stage, fallback = "Idle") {
  if (!stage) return fallback;
  return `${stageLabel(stage.stage || stage.name)}: ${stage.status || "started"}`;
}

function stageLabel(value) {
  return String(value || "")
    .split("_")
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ") || "Stage";
}

async function makeMove(move) {
  const normalizedMove = String(move || "").trim();
  if (!normalizedMove) {
    showError("Enter a UCI move before playing.");
    document.querySelector("#moveInput").focus();
    return;
  }
  return withBusyControl("#moveBtn", async () => {
    try {
      document.querySelector("#thinkingStage").textContent = "Applying user move";
      state = await call("MakeUserMove", normalizedMove);
      resetBoardEntry();
      render();
      if (autoReply && state?.snapshot?.outcome?.status === "ongoing" && state.snapshot.side_to_move !== playerSide) {
        await askEngine();
      }
    } catch (err) {
      showError(err);
      document.querySelector("#thinkingStage").textContent = "Move failed";
    }
  });
}

async function askEngine() {
  return withBusyControl("#engineBtn", async () => {
    try {
      lastStageEvent = null;
      document.querySelector("#thinkingStage").textContent = "Thinking: provider, repair, verifier, scoring";
      const result = await call("RequestEngineMove");
      state = result.state;
      resetBoardEntry();
      render();
    } catch (err) {
      showError(err);
      document.querySelector("#thinkingStage").textContent = "Engine stopped";
    }
  });
}

async function analyzeCurrentPosition() {
  return withBusyControl("#analyzeBtn", async () => {
    try {
      lastStageEvent = null;
      document.querySelector("#thinkingStage").textContent = "Analyzing current position";
      const decision = await call("AnalyzeCurrentPosition");
      if (state) {
        state.last_decision = decision;
      }
      renderStatus();
      renderDecision();
    } catch (err) {
      showError(err);
      document.querySelector("#thinkingStage").textContent = "Analysis failed";
    }
  });
}

async function whyNotMove() {
  const move = document.querySelector("#moveInput").value.trim();
  if (!move) {
    document.querySelector("#tabContent").textContent = "Enter a move to compare.";
    return;
  }
  return withBusyControl("#whyBtn", async () => {
    try {
      const comparison = await call("WhyNotMove", move);
      document.querySelector("#tabContent").textContent = whyNotText(comparison);
    } catch (err) {
      showError(err);
    }
  });
}

function whyNotText(comparison) {
  return [
    comparison.summary || "No comparison available.",
    "",
    `Requested: ${comparison.requested_move || "unknown"}`,
    `Selected: ${comparison.selected_move || "unknown"}`,
    "",
    "REQUESTED CANDIDATE",
    JSON.stringify(comparison.requested || {}, null, 2),
    "",
    "SELECTED CANDIDATE",
    JSON.stringify(comparison.selected || {}, null, 2)
  ].join("\n");
}

function subscribeDecisionStageEvents() {
  if (!window.runtime?.EventsOn) return;
  window.runtime.EventsOn("decision.stage", (event) => {
    lastStageEvent = event;
    document.querySelector("#thinkingStage").textContent = stageStatusText(event, "Thinking");
  });
}

async function resignGame() {
  if (!state?.snapshot || state.snapshot.outcome?.status !== "ongoing") return;
  const side = playerSide || state.snapshot.side_to_move || "white";
  if (!window.confirm(`Resign as ${side}?`)) return;
  return withBusyControl("#resignBtn", async () => {
    try {
      state = await call("Resign", side);
      resetBoardEntry();
      render();
    } catch (err) {
      showError(err);
    }
  });
}

async function loadSettings() {
  settings = await call("GetSettings");
  populateProviderProfiles(settings.llm?.profiles || []);
  populateCustomPersonalities(settings.engine?.custom_personalities || []);
  document.querySelector("#settingMode").value = settings.engine.default_mode;
  document.querySelector("#settingPersonality").value = settings.engine.personality;
  document.querySelector("#settingCustomPersonality").value = settings.engine.custom_personality_id || "";
  document.querySelector("#settingSide").value = playerSide;
  document.querySelector("#settingVariant").value = gameVariant;
  document.querySelector("#settingVariantSeed").value = chess960Seed;
  document.querySelector("#settingTheme").value = settings.gui?.theme || "system";
  applyTheme(settings.gui?.theme || "system");
  document.querySelector("#settingTimeControl").value = settings.gui?.time_control || "untimed";
  const clockInitialMS = Number(settings.gui?.clock_initial_ms);
  const clockIncrementMS = Number(settings.gui?.clock_increment_ms);
  document.querySelector("#settingClockInitial").value = Math.max(0, Math.round((Number.isFinite(clockInitialMS) ? clockInitialMS : 300000) / 60000));
  document.querySelector("#settingClockIncrement").value = Math.max(0, Math.round((Number.isFinite(clockIncrementMS) ? clockIncrementMS : 0) / 1000));
  document.querySelector("#settingAutoReply").checked = autoReply;
  document.querySelector("#settingMaxCandidates").value = settings.engine.max_candidates || 5;
  document.querySelector("#settingProfile").value = providerProfileValue(settings.llm?.profile_id);
  document.querySelector("#settingProvider").value = settings.llm.provider;
  document.querySelector("#settingEndpoint").value = settings.llm.endpoint || "";
  document.querySelector("#settingModel").value = settings.llm.model || "";
  document.querySelector("#settingTemperature").value = settings.llm.temperature ?? 0.2;
  document.querySelector("#settingMaxTokens").value = settings.llm.max_tokens || 1600;
  document.querySelector("#settingTimeout").value = settings.llm.timeout_ms || 12000;
  document.querySelector("#settingRetries").value = settings.llm.retries ?? 1;
  document.querySelector("#settingKey").value = settings.llm.api_key || "";
  document.querySelector("#settingKeyRef").value = settings.llm.api_key_ref || "";
  document.querySelector("#settingCloudAck").checked = !!settings.privacy.cloud_provider_warning_acknowledged;
  document.querySelector("#settingVerifier").checked = !!settings.verifier.enabled;
  document.querySelector("#settingVerifierPath").value = settings.verifier.path || "";
  document.querySelector("#settingVerifierMoveTime").value = settings.verifier.movetime_ms || 100;
  document.querySelector("#settingVerifierMaxLoss").value = settings.verifier.max_centipawn_loss || 180;
  document.querySelector("#settingTablebase").checked = !!settings.verifier.tablebase_enabled;
  document.querySelector("#settingTablebasePath").value = settings.verifier.tablebase_path || "";
  document.querySelector("#settingTablebaseTimeout").value = settings.verifier.tablebase_timeout_ms || 1000;
  document.querySelector("#settingTraceEnabled").checked = !!settings.engine.trace_enabled;
  document.querySelector("#settingLogDir").value = settings.logging.output_dir || "logs";
  document.querySelector("#settingRaw").checked = !!settings.privacy.log_raw_prompts;
  document.querySelector("#settingRawResponses").checked = !!settings.privacy.log_raw_llm_responses;
  syncTimeControlInputsFromPreset(false);
  syncProviderDisclosure();
}

function populateProviderProfiles(profiles) {
  const select = document.querySelector("#settingProfile");
  select.innerHTML = "";
  const custom = document.createElement("option");
  custom.value = "custom";
  custom.textContent = "Custom";
  select.appendChild(custom);
  for (const profile of profiles || []) {
    if (!profile?.id) continue;
    const option = document.createElement("option");
    option.value = profile.id;
    option.textContent = profile.id;
    select.appendChild(option);
  }
}

function populateCustomPersonalities(profiles) {
  const select = document.querySelector("#settingCustomPersonality");
  select.innerHTML = "";
  const none = document.createElement("option");
  none.value = "";
  none.textContent = "None";
  select.appendChild(none);
  for (const profile of profiles || []) {
    if (!profile?.id) continue;
    const option = document.createElement("option");
    option.value = profile.id;
    option.textContent = profile.name ? `${profile.name} (${profile.id})` : profile.id;
    select.appendChild(option);
  }
}

function providerProfileValue(profileID) {
  const id = profileID || "custom";
  const exists = [...document.querySelector("#settingProfile").options].some((option) => option.value === id);
  return exists ? id : "custom";
}

function selectedProviderProfile() {
  const profileID = document.querySelector("#settingProfile")?.value;
  return (settings?.llm?.profiles || []).find((profile) => profile.id === profileID) || null;
}

function applySelectedProviderProfile() {
  const profile = selectedProviderProfile();
  if (!profile) return;
  applyingProviderProfile = true;
  document.querySelector("#settingProvider").value = profile.provider || "mock";
  document.querySelector("#settingEndpoint").value = profile.endpoint || profile.base_url || "";
  document.querySelector("#settingModel").value = profile.model || "";
  document.querySelector("#settingTemperature").value = profile.temperature ?? settings?.llm?.temperature ?? 0.2;
  document.querySelector("#settingMaxTokens").value = profile.max_tokens || settings?.llm?.max_tokens || 1600;
  document.querySelector("#settingTimeout").value = profile.timeout_ms || settings?.llm?.timeout_ms || 12000;
  document.querySelector("#settingRetries").value = profile.retries ?? settings?.llm?.retries ?? 1;
  document.querySelector("#settingKeyRef").value = profile.api_key_ref || "";
  applyingProviderProfile = false;
  syncProviderDisclosure();
}

function markProviderProfileCustom() {
  if (applyingProviderProfile) return;
  document.querySelector("#settingProfile").value = "custom";
}

async function saveSettings() {
  return withBusyControl("#saveSettingsBtn", async () => {
    try {
      playerSide = document.querySelector("#settingSide").value;
      if (playerSide === "random") playerSide = Math.random() < 0.5 ? "white" : "black";
      autoReply = document.querySelector("#settingAutoReply").checked;
      gameVariant = document.querySelector("#settingVariant").value || "standard";
      chess960Seed = Number(document.querySelector("#settingVariantSeed").value) || 0;
      const timeControl = timeControlForNewGame();
      settings.engine.default_mode = document.querySelector("#settingMode").value;
      settings.engine.personality = document.querySelector("#settingPersonality").value;
      settings.engine.custom_personality_id = document.querySelector("#settingCustomPersonality").value;
      settings.engine.max_candidates = Number(document.querySelector("#settingMaxCandidates").value) || settings.engine.max_candidates;
      settings.gui.theme = document.querySelector("#settingTheme").value || "system";
      settings.gui.time_control = document.querySelector("#settingTimeControl").value;
      settings.gui.clock_initial_ms = timeControl.initial_ms;
      settings.gui.clock_increment_ms = timeControl.increment_ms;
      settings.llm.profile_id = document.querySelector("#settingProfile").value || "custom";
      settings.llm.provider = document.querySelector("#settingProvider").value;
      settings.llm.endpoint = document.querySelector("#settingEndpoint").value;
      settings.llm.model = document.querySelector("#settingModel").value;
      settings.llm.temperature = Number(document.querySelector("#settingTemperature").value);
      settings.llm.max_tokens = Number(document.querySelector("#settingMaxTokens").value) || settings.llm.max_tokens;
      settings.llm.timeout_ms = Number(document.querySelector("#settingTimeout").value) || settings.llm.timeout_ms;
      settings.llm.retries = Number(document.querySelector("#settingRetries").value) || 0;
      settings.llm.api_key = document.querySelector("#settingKey").value;
      settings.llm.api_key_ref = document.querySelector("#settingKeyRef").value;
      settings.privacy.cloud_provider_warning_acknowledged = document.querySelector("#settingCloudAck").checked;
      if (providerRequiresAck(settings.llm.provider) && !settings.privacy.cloud_provider_warning_acknowledged) {
        document.querySelector("#settingsOutput").textContent = "Acknowledge provider endpoint data sharing before saving.";
        return;
      }
      settings.verifier.enabled = document.querySelector("#settingVerifier").checked;
      settings.verifier.path = document.querySelector("#settingVerifierPath").value;
      settings.verifier.movetime_ms = Number(document.querySelector("#settingVerifierMoveTime").value) || settings.verifier.movetime_ms;
      settings.verifier.max_centipawn_loss = Number(document.querySelector("#settingVerifierMaxLoss").value) || settings.verifier.max_centipawn_loss;
      settings.verifier.tablebase_enabled = document.querySelector("#settingTablebase").checked;
      settings.verifier.tablebase_path = document.querySelector("#settingTablebasePath").value;
      settings.verifier.tablebase_timeout_ms = Number(document.querySelector("#settingTablebaseTimeout").value) || settings.verifier.tablebase_timeout_ms || 1000;
      settings.engine.trace_enabled = document.querySelector("#settingTraceEnabled").checked;
      settings.logging.output_dir = document.querySelector("#settingLogDir").value || settings.logging.output_dir;
      settings.privacy.log_raw_prompts = document.querySelector("#settingRaw").checked;
      settings.privacy.log_raw_llm_responses = document.querySelector("#settingRawResponses").checked;
      await call("SaveSettings", settings);
      applyTheme(settings.gui.theme);
      document.querySelector("#settingsOutput").textContent = "Settings saved.";
    } catch (err) {
      showError(err, "#settingsOutput");
    }
  });
}

async function saveProviderKeyToKeychain() {
  const apiKey = document.querySelector("#settingKey").value.trim();
  if (!apiKey || apiKey === "[REDACTED]") {
    document.querySelector("#settingsOutput").textContent = "Enter an API key before saving it to the keychain.";
    return;
  }
  try {
    settings = await call("SaveProviderAPIKeyToKeychain", document.querySelector("#settingProfile").value, apiKey);
    await loadSettings();
    document.querySelector("#settingsOutput").textContent = "API key saved to keychain reference.";
  } catch (err) {
    showError(err, "#settingsOutput");
  }
}

function syncProviderDisclosure() {
  const warning = document.querySelector("#cloudProviderWarning");
  const isCloud = providerRequiresAck(document.querySelector("#settingProvider").value);
  warning.classList.toggle("hidden", !isCloud);
}

function providerRequiresAck(provider) {
  return ["openai_compatible", "anthropic", "gemini", "ollama"].includes(provider);
}

function syncTimeControlInputsFromPreset(overwrite) {
  const preset = document.querySelector("#settingTimeControl").value;
  const initialInput = document.querySelector("#settingClockInitial");
  const incrementInput = document.querySelector("#settingClockIncrement");
  const custom = preset === "custom";
  initialInput.disabled = !custom;
  incrementInput.disabled = !custom;
  initialInput.title = custom ? "" : "Choose Custom to edit the clock values.";
  incrementInput.title = custom ? "" : "Choose Custom to edit the clock values.";
  if (custom) return;
  const tc = timeControlPresets[preset] || timeControlPresets.untimed;
  initialInput.value = Math.max(0, Math.round((tc.initial_ms || 0) / 60000));
  incrementInput.value = Math.round((tc.increment_ms || 0) / 1000);
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

function renderBenchmarkSummary(summary) {
  if (!summary) return "No benchmark result.";
  if (summary.results?.length && summary.results[0]?.summary) {
    const rows = summary.results.map((result) => {
      const s = result.summary || {};
      return `${result.mode}: ${s.games_completed || 0}/${s.games_requested || summary.games_per_mode || 0} games · ${s.total_plies || 0} plies · ${s.fallbacks_used || 0} fallbacks · ${s.engine_errors || 0} errors`;
    });
    return `Mode benchmark · ${summary.games_per_mode || 0} games/mode · seed ${summary.seed || 64}\n${rows.join("\n")}`;
  }
  return `Random benchmark · ${summary.games_completed || 0}/${summary.games_requested || 0} games · ${summary.total_plies || 0} plies · ${summary.fallbacks_used || 0} fallbacks · ${summary.engine_errors || 0} errors`;
}

function renderPositionSuiteSummary(summary) {
  if (!summary) return "No position suite result.";
  const rows = (summary.results || []).map((result) => {
    const move = result.selected_san ? `${result.selected_san} (${result.selected_move})` : result.selected_move || "none";
    const status = result.engine_error ? `error: ${result.engine_error}` : `${move} · ${result.candidate_count || 0} candidates · ${result.duration_ms || 0} ms`;
    return `${result.index}. ${result.name || "Position"} · ${result.side_to_move || "unknown"} · ${status}`;
  });
  return [
    `Position suite · ${summary.positions_analyzed || 0}/${summary.positions_requested || 0} analyzed · ${summary.fallbacks_used || 0} fallbacks · ${summary.engine_errors || 0} errors`,
    ...rows
  ].join("\n");
}

function renderProviderComparison(summary) {
  if (!summary) return "No provider comparison result.";
  const rows = (summary.results || []).map((result) => {
    const suite = result.summary || {};
    return `${result.profile_id || "profile"} · ${result.status || "unknown"} · ${suite.positions_analyzed || 0}/${suite.positions_requested || 0} positions · ${suite.fallbacks_used || 0} fallbacks · ${result.error || ""}`.trim();
  });
  return [
    `Provider comparison · ${summary.profiles_compared || 0} profiles completed · ${summary.positions?.length || 0} positions`,
    ...rows
  ].join("\n");
}

function renderProviderDashboard(dashboard) {
  if (!dashboard) return "No provider dashboard result.";
  const rows = (dashboard.profiles || []).map((profile) => {
    const endpoint = profile.endpoint ? ` · ${profile.endpoint}` : "";
    const error = profile.error ? ` · ${profile.error}` : "";
    return `${profile.id || "profile"} · ${profile.provider || "provider"} · ${profile.model || "model"} · ${profile.status || "unknown"}${endpoint}${error}`;
  });
  return [
    `Provider dashboard · active ${dashboard.active_profile || "custom"} · ${dashboard.active_provider || "unknown"} · ${dashboard.active_model || "model"}`,
    ...rows
  ].join("\n");
}

function renderPromptPlayground(result) {
  if (!result) return "No prompt playground result.";
  const renderSide = (label, side) => {
    const candidates = (side?.candidates || []).map((c) => `${c.san || c.uci} · ${c.purpose || ""}`).join("\n");
    return [
      label,
      `${side?.provider || "provider"} · ${side?.model || "model"} · valid ${!!side?.valid} · parse ${side?.parse_status || "none"}`,
      side?.error ? `Error: ${side.error}` : "",
      candidates || "No legal candidates parsed."
    ].filter(Boolean).join("\n");
  };
  return [
    `Prompt playground · ${result.game_id || "unknown"} · ply ${result.ply || 0}`,
    `Changed: ${(result.comparison?.changed_files || []).join(", ") || "none"}`,
    "",
    renderSide("LEFT", result.left),
    "",
    renderSide("RIGHT", result.right)
  ].join("\n");
}

function renderStudyDashboard(dashboard) {
  if (!dashboard) return "No study dashboard result.";
  const memory = dashboard.memory || {};
  const coherence = dashboard.coherence || {};
  const diversity = dashboard.candidate_diversity || {};
  const lesson = dashboard.lesson || {};
  const multi = dashboard.multi_agent || {};
  const agents = (multi.reviews || []).map((review) => `${review.role}: ${review.summary} (${formatMetric(review.confidence)})`).join("\n");
  const heat = (dashboard.heatmap || []).map((item) => `${item.move} -> ${item.square} · ${formatScore(item.weight)} · ${item.label || "candidate"}`).join("\n");
  return [
    `Study · ${dashboard.game_id || "unknown"} · ply ${dashboard.ply || 0} · ${dashboard.variant?.variant || "standard"}`,
    `Memory: ${memory.plan_status || "unknown"} · confidence ${formatMetric(memory.plan_confidence)} · retained ${memory.retained_items || 0} · dropped ${memory.dropped_items || 0}`,
    `Coherence: ${coherence.status || "unknown"} · ${formatMetric(coherence.score)}`,
    `Diversity: ${diversity.status || "unknown"} · ${formatMetric(diversity.score)} · ${diversity.candidate_count || 0} candidates`,
    "",
    "LESSON",
    `${lesson.title || "Study"} · ${lesson.focus || ""}`,
    ...(lesson.steps || []),
    "",
    "AGENTS",
    agents || multi.arbiter || "No agent review.",
    "",
    "HEATMAP",
    heat || "No candidate heatmap.",
    "",
    "PUZZLE",
    `${dashboard.puzzle?.goal || "No puzzle."}\nSolution: ${dashboard.puzzle?.solution || "pending"}`
  ].join("\n");
}

function renderTournament(summary) {
  if (!summary) return "No tournament result.";
  const ratings = (summary.ratings || []).map((rating) => `${rating.id} · ${formatMetric(rating.elo)} · ${rating.wins}-${rating.draws}-${rating.losses}`).join("\n");
  const games = (summary.results || []).slice(0, 12).map((game) => `${game.game_index}. ${game.white_id}-${game.black_id} · ${game.outcome || "unknown"} · ${game.winner_id || "draw"} · ${game.plies || 0} plies`).join("\n");
  return [
    `Tournament · ${summary.games_played || 0} games · seed ${summary.seed || 64}`,
    "",
    "RATINGS",
    ratings || "No ratings.",
    "",
    "GAMES",
    games || "No games."
  ].join("\n");
}

function renderBackupManifest(manifest) {
  if (!manifest) return "No backup manifest.";
  const files = (manifest.files || []).map((file) => `${file.path} · ${file.bytes || 0} bytes`).join("\n");
  return [
    `Archive: ${manifest.archive_path || "unknown"}`,
    `SHA-256: ${manifest.sha256 || "pending"}`,
    `Files: ${(manifest.files || []).length} · bytes ${manifest.bytes || 0}`,
    "",
    files
  ].join("\n");
}

function renderFineTuneWorkflow(workflow) {
  if (!workflow) return "No fine-tune workflow.";
  const spec = workflow.workflow || {};
  return [
    `Fine tune · ${spec.example_count || 0} examples`,
    spec.intended_use || "",
    "",
    "SAFETY",
    ...(spec.safety_notes || []),
    "",
    "JSONL",
    workflow.dataset_jsonl || ""
  ].join("\n");
}

function renderReview(review) {
  if (!review) return "No review available.";
  const metrics = review.strategy_metrics || {};
  const recommendations = review.recommendations?.length ? review.recommendations.join("\n") : "No recommendations.";
  return [
    review.summary || "No review summary.",
    "",
    `Game: ${review.game_id || "unknown"} · ply ${review.ply || 0} · ${review.outcome_status || "unknown"}`,
    `Move: ${review.selected_move || "none"}`,
    `Provider: ${review.provider || "none"} · mode ${review.mode || "none"}`,
    `Plan: ${review.plan || "none"}`,
    `Strategy quality: ${formatMetric(metrics.quality)} · completeness ${formatMetric(metrics.completeness)} · consistency ${formatMetric(metrics.consistency)} · drift ${formatMetric(metrics.drift)} · alerts ${metrics.alert_level || "none"}`,
    "",
    "POSITION SUMMARY",
    review.position_summary || "No position summary recorded.",
    "",
    "RECOMMENDATIONS",
    recommendations
  ].join("\n");
}

async function openReview() {
  document.querySelector("#reviewOutput").textContent = "Loading review...";
  document.querySelector("#reviewDialog").showModal();
  await refreshReview();
}

async function refreshReview() {
  try {
    document.querySelector("#reviewOutput").textContent = renderReview(await call("PostGameReview"));
  } catch (err) {
    showError(err, "#reviewOutput");
  }
}

async function openStudy() {
  document.querySelector("#studyOutput").textContent = "Loading study tools...";
  document.querySelector("#studyDialog").showModal();
  await refreshStudy();
}

async function refreshStudy() {
  try {
    const dashboard = await call("StudyDashboard");
    const current = await call("GetGame");
    document.querySelector("#studyMemoryText").value = JSON.stringify(current.strategy_memory || {}, null, 2);
    document.querySelector("#studyOutput").textContent = renderStudyDashboard(dashboard);
  } catch (err) {
    showError(err, "#studyOutput");
  }
}

async function refreshMultiAgent() {
  try {
    document.querySelector("#studyOutput").textContent = JSON.stringify(await call("MultiAgentAnalysis"), null, 2);
  } catch (err) {
    showError(err, "#studyOutput");
  }
}

async function saveStudyMemory() {
  try {
    const memory = parseJSONField("#studyMemoryText", "Strategy memory");
    state = await call("UpdateStrategyMemory", memory);
    render();
    await refreshStudy();
  } catch (err) {
    showError(err, "#studyOutput");
  }
}

function openExperiments() {
  document.querySelector("#experimentsOutput").textContent = "";
  document.querySelector("#experimentsDialog").showModal();
}

function openLab() {
  document.querySelector("#labOutput").textContent = "";
  document.querySelector("#tournamentGames").value = document.querySelector("#tournamentGames").value || "1";
  if (!document.querySelector("#customBoardDefinition").value.trim()) {
    document.querySelector("#customBoardDefinition").value = JSON.stringify(defaultCustomBoardDefinition(), null, 2);
  }
  document.querySelector("#labDialog").showModal();
}

function defaultCustomBoardDefinition() {
  return {
    schema_version: "custom-board-definition.v1",
    id: "archbishop-lab",
    name: "Archbishop Lab",
    initial_fen: "4k3/8/8/8/3A4/8/8/4K3 w - - 0 1",
    rule_set: "custom-piece-lab",
    board_width: 8,
    board_height: 8,
    piece_rules: [
      {
        symbol: "A",
        name: "Archbishop",
        move: "bishop+knight"
      }
    ]
  };
}

async function newChess960Game() {
  try {
    const seed = Math.floor(Date.now() % 960);
    chess960Seed = seed;
    gameVariant = "chess960";
    state = await call("NewGame", {
      side: playerSide || "white",
      variant: "chess960",
      seed,
      mode: document.querySelector("#settingMode")?.value || "blunderguard",
      personality: document.querySelector("#settingPersonality")?.value || "balanced",
      time_control: timeControlForNewGame()
    });
    resetBoardEntry();
    render();
    document.querySelector("#labOutput").textContent = JSON.stringify(state.variant || {}, null, 2);
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function startCustomBoardFromLab() {
  try {
    const definition = parseJSONField("#customBoardDefinition", "Custom board definition");
    let side = document.querySelector("#settingSide")?.value || playerSide || "white";
    if (side === "random") side = Math.random() < 0.5 ? "white" : "black";
    playerSide = side;
    gameVariant = "custom";
    state = await call("NewGame", {
      side,
      variant: "custom",
      board_definition: definition,
      mode: document.querySelector("#settingMode")?.value || "blunderguard",
      personality: document.querySelector("#settingPersonality")?.value || "balanced",
      time_control: timeControlForNewGame()
    });
    resetBoardEntry();
    render();
    document.querySelector("#labOutput").textContent = JSON.stringify(state.variant || {}, null, 2);
    if (autoReply && side === "black") await askEngine();
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function createBackup() {
  try {
    const manifest = await call("CreateBackup", document.querySelector("#backupDir").value.trim());
    document.querySelector("#restoreArchive").value = manifest.archive_path || "";
    document.querySelector("#labOutput").textContent = renderBackupManifest(manifest);
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function restoreBackup() {
  try {
    const archive = requireField("#restoreArchive", "Choose a backup archive before restoring.");
    const manifest = await call("RestoreBackup", archive, document.querySelector("#restoreTarget").value.trim());
    document.querySelector("#labOutput").textContent = renderBackupManifest(manifest);
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function exportFineTuneWorkflow() {
  try {
    document.querySelector("#labOutput").textContent = renderFineTuneWorkflow(await call("ExportFineTuneDataset"));
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function runTournamentFromLab() {
  try {
    const games = Number(document.querySelector("#tournamentGames").value) || 1;
    document.querySelector("#labOutput").textContent = "Running tournament...";
    document.querySelector("#labOutput").textContent = renderTournament(await call("RunTournament", games, 64));
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function compareAnalysisModes() {
  try {
    document.querySelector("#labOutput").textContent = "Comparing modes...";
    const comparison = await call("ComparePureHybridAnalysis");
    document.querySelector("#labOutput").textContent = [
      comparison.summary || "No comparison summary.",
      "",
      "PURE",
      `${comparison.pure?.selected_move?.san || comparison.pure?.selected_move?.uci || "none"} · ${comparison.pure?.explanation || ""}`,
      "",
      "HYBRID",
      `${comparison.hybrid?.selected_move?.san || comparison.hybrid?.selected_move?.uci || "none"} · ${comparison.hybrid?.explanation || ""}`
    ].join("\n");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function comparePromptPlayground() {
  try {
    const base = await call("PromptTemplatePack");
    const variant = { ...base, source: "playground", user: `${base.user || ""}\n\nPrefer concise contrast between the top two candidates.\n` };
    document.querySelector("#labOutput").textContent = renderPromptPlayground(await call("RunPromptPlayground", base, variant));
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function runPromptPlaygroundFromEditor() {
  try {
    const base = await call("PromptTemplatePack");
    const edited = promptPackFromInputs();
    document.querySelector("#promptOutput").textContent = renderPromptPlayground(await call("RunPromptPlayground", base, edited));
  } catch (err) {
    showError(err, "#promptOutput");
  }
}

async function buildPersonalityFromLab() {
  try {
    const profile = await call("BuildCustomPersonalityProfile", "lab-balanced", "Lab Balanced", 0.55, ["development", "king safety"], ["Prefer clear plans with tactical checks."]);
    settings = await call("SaveCustomPersonalityProfile", profile, true);
    await loadSettings();
    document.querySelector("#labOutput").textContent = JSON.stringify({ saved: profile, selected: settings.engine?.custom_personality_id }, null, 2);
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function trainPolicyPriorFromLab() {
  try {
    const workflow = await call("ExportFineTuneDataset");
    const path = document.querySelector("#policyModelPath").value.trim() || "logs/policy-prior-model.json";
    const result = await call("TrainLocalPolicyPrior", workflow.dataset_jsonl || "", path);
    document.querySelector("#policyModelPath").value = result.model_path || path;
    document.querySelector("#labOutput").textContent = JSON.stringify(result, null, 2);
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function enablePolicyPriorFromLab() {
  try {
    const path = requireField("#policyModelPath", "Enter a policy model path before enabling the prior.");
    settings = await call("EnablePolicyPriorModel", path);
    await loadSettings();
    document.querySelector("#labOutput").textContent = `Policy prior enabled: ${settings.llm?.model || ""}`;
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function importOpeningBookFromLab() {
  try {
    const path = requireField("#openingBookPath", "Enter an opening book path before importing.");
    const book = await call("ImportOpeningBook", path);
    const suggestions = await call("OpeningBook");
    document.querySelector("#labOutput").textContent = JSON.stringify({ imported: book, current_suggestions: suggestions }, null, 2);
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function runAccessibilityAudit() {
  try {
    document.querySelector("#settingsOutput").textContent = JSON.stringify(await call("AccessibilityAudit"), null, 2);
  } catch (err) {
    showError(err, "#settingsOutput");
  }
}

async function runExperiment(action, label, renderResult) {
  try {
    document.querySelector("#experimentsOutput").textContent = `${label}...`;
    document.querySelector("#experimentsOutput").textContent = renderResult(await action());
  } catch (err) {
    showError(err, "#experimentsOutput");
  }
}

async function loadPromptEditor() {
  try {
    promptPack = await call("PromptTemplatePack");
    document.querySelector("#promptSource").value = promptPack.source || "default";
    document.querySelector("#promptManifest").value = JSON.stringify(promptPack.manifest || {}, null, 2);
    document.querySelector("#promptSystem").value = promptPack.system || "";
    document.querySelector("#promptUser").value = promptPack.user || "";
    document.querySelector("#promptSchema").value = promptPack.schema || "";
    document.querySelector("#promptOutput").textContent = "";
  } catch (err) {
    showError(err, "#promptOutput");
  }
}

async function openPromptEditor() {
  document.querySelector("#promptDialog").showModal();
  await loadPromptEditor();
}

function promptPackFromInputs() {
  const manifest = parseJSONField("#promptManifest", "Prompt manifest");
  return {
    schema_version: "prompt-template-pack.v1",
    source: document.querySelector("#promptSource").value || "editor",
    manifest,
    system: document.querySelector("#promptSystem").value,
    user: document.querySelector("#promptUser").value,
    schema: document.querySelector("#promptSchema").value
  };
}

async function validatePromptEditor() {
  try {
    const validation = await call("ValidatePromptTemplatePack", promptPackFromInputs());
    document.querySelector("#promptOutput").textContent = JSON.stringify(validation, null, 2);
    return validation;
  } catch (err) {
    showError(err, "#promptOutput");
    return null;
  }
}

async function savePromptEditor() {
  try {
    const dir = requireField("#promptSaveDir", "Enter a save directory before saving the prompt pack.");
    const validation = await call("SavePromptTemplatePack", dir, promptPackFromInputs());
    document.querySelector("#promptOutput").textContent = JSON.stringify(validation, null, 2);
  } catch (err) {
    showError(err, "#promptOutput");
  }
}

async function openProfilesEditor() {
  document.querySelector("#profilesDialog").showModal();
  await exportProfiles();
}

async function exportProfiles() {
  try {
    document.querySelector("#profilesText").value = await call("ExportProviderProfiles");
    document.querySelector("#profilesOutput").textContent = "Profiles exported.";
  } catch (err) {
    showError(err, "#profilesOutput");
  }
}

async function importProfiles() {
  try {
    const text = requireField("#profilesText", "Paste provider profiles before importing.");
    settings = await call("ImportProviderProfiles", text);
    populateProviderProfiles(settings.llm?.profiles || []);
    document.querySelector("#profilesOutput").textContent = "Profiles imported.";
  } catch (err) {
    showError(err, "#profilesOutput");
  }
}

function renderRecentGames(records) {
  const list = document.querySelector("#recentList");
  list.innerHTML = "";
  list.setAttribute("role", "list");
  if (!records?.length) {
    list.textContent = "No recent games.";
    return;
  }
  for (const record of records) {
    const { gameID, savedAt, snapshot } = gameRecordFields(record);
    const row = document.createElement("div");
    row.className = "recent-game";
    row.setAttribute("role", "listitem");
    const detail = document.createElement("div");
    const title = document.createElement("strong");
    title.textContent = snapshot.ply ? `${snapshot.ply} plies · ${snapshot.side_to_move || "unknown"} to move` : "New game";
    const meta = document.createElement("small");
    const outcomeStatus = snapshot.outcome?.status || "ongoing";
    meta.textContent = `${formatSavedAt(savedAt)} · ${outcomeStatus}`;
    detail.append(title, meta);
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = "Load";
    button.setAttribute("aria-label", `Load ${title.textContent} from ${formatSavedAt(savedAt)}, ${outcomeStatus}`);
    button.addEventListener("click", () => withBusyControl(button, async () => {
      try {
        state = await call("LoadRecentGame", gameID);
        resetBoardEntry();
        render();
        document.querySelector("#recentDialog").close();
      } catch (err) {
        showError(err, "#recentOutput");
      }
    }));
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
    document.querySelector("#recentList").textContent = "Recent games could not be loaded.";
    showError(err, "#recentOutput");
  }
}

function activateTraceTab(btn, focus = false) {
  document.querySelectorAll(".tabs button").forEach((b) => {
    const selectedTab = b === btn;
    b.classList.toggle("active", selectedTab);
    b.setAttribute("aria-selected", selectedTab ? "true" : "false");
    b.tabIndex = selectedTab ? 0 : -1;
  });
  activeTab = btn.dataset.tab;
  renderDecision();
  if (focus) btn.focus();
}

function moveTraceTabFocus(current, delta) {
  const tabs = [...document.querySelectorAll(".tabs button")];
  const index = tabs.indexOf(current);
  if (index < 0) return;
  const next = tabs[(index + delta + tabs.length) % tabs.length];
  activateTraceTab(next, true);
}

document.querySelectorAll(".tabs button").forEach((btn) => {
  btn.addEventListener("click", () => activateTraceTab(btn));
  btn.addEventListener("keydown", (event) => {
    if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      event.preventDefault();
      moveTraceTabFocus(btn, 1);
    }
    if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      event.preventDefault();
      moveTraceTabFocus(btn, -1);
    }
    if (event.key === "Home") {
      event.preventDefault();
      activateTraceTab(document.querySelector(".tabs button"), true);
    }
    if (event.key === "End") {
      event.preventDefault();
      activateTraceTab([...document.querySelectorAll(".tabs button")].at(-1), true);
    }
  });
});

bindBusyButton("#newGameBtn", async () => {
  try {
    let side = document.querySelector("#settingSide")?.value || playerSide;
    if (side === "random") side = Math.random() < 0.5 ? "white" : "black";
    playerSide = side;
    state = await call("NewGame", {
      side,
      variant: document.querySelector("#settingVariant")?.value || gameVariant,
      seed: Number(document.querySelector("#settingVariantSeed")?.value) || chess960Seed,
      mode: document.querySelector("#settingMode")?.value || "blunderguard",
      personality: document.querySelector("#settingPersonality")?.value || "balanced",
      time_control: timeControlForNewGame()
    });
    resetBoardEntry();
    render();
    if (autoReply && side === "black") await askEngine();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#settingTimeControl").addEventListener("change", () => syncTimeControlInputsFromPreset(true));
document.querySelector("#settingTheme").addEventListener("change", () => applyTheme(document.querySelector("#settingTheme").value));
document.querySelector("#settingProfile").addEventListener("change", applySelectedProviderProfile);
document.querySelector("#settingProvider").addEventListener("change", () => {
  markProviderProfileCustom();
  syncProviderDisclosure();
});
["#settingEndpoint", "#settingModel", "#settingTemperature", "#settingMaxTokens", "#settingTimeout", "#settingRetries", "#settingKey", "#settingKeyRef"].forEach((selector) => {
  document.querySelector(selector).addEventListener("input", markProviderProfileCustom);
});
bindBusyButton("#recentBtn", openRecentGames);
document.querySelector("#engineBtn").addEventListener("click", askEngine);
document.querySelector("#analyzeBtn").addEventListener("click", analyzeCurrentPosition);
bindBusyButton("#stopBtn", async () => {
  try {
    await call("StopEngine");
    document.querySelector("#thinkingStage").textContent = "Stop requested";
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#resignBtn").addEventListener("click", resignGame);
bindBusyButton("#undoBtn", async () => {
  try {
    state = await call("Undo", 1);
    resetBoardEntry();
    render();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#flipBtn").addEventListener("click", () => { flipped = !flipped; renderBoard(); });
bindBusyButton("#reviewBtn", openReview);
bindBusyButton("#studyBtn", openStudy);
document.querySelector("#experimentsBtn").addEventListener("click", openExperiments);
document.querySelector("#labBtn").addEventListener("click", openLab);
bindBusyButton("#promptEditorBtn", openPromptEditor);
document.querySelector("#moveBtn").addEventListener("click", () => makeMove(document.querySelector("#moveInput").value.trim()));
document.querySelector("#moveInput").addEventListener("keydown", (event) => {
  if (event.key !== "Enter") return;
  event.preventDefault();
  makeMove(document.querySelector("#moveInput").value.trim());
});
document.querySelector("#whyBtn").addEventListener("click", whyNotMove);
bindBusyButton("#settingsBtn", async () => {
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
document.querySelector("#importText").addEventListener("keydown", (event) => {
  if (event.key !== "Enter" || (!event.ctrlKey && !event.metaKey)) return;
  event.preventDefault();
  document.querySelector("#runImportBtn").click();
});
bindBusyButton("#runImportBtn", async () => {
  const type = document.querySelector("#importType").value;
  const text = document.querySelector("#importText").value;
  if (!text.trim()) {
    showError("Paste a FEN or PGN before importing.", "#importOutput");
    document.querySelector("#importText").focus();
    return;
  }
  try {
    state = type === "fen" ? await call("ImportFEN", text) : await call("ImportPGN", text);
    resetBoardEntry();
    render();
    document.querySelector("#importDialog").close();
  } catch (err) {
    showError(err, "#importOutput");
  }
});
document.querySelector("#promotionDialog").addEventListener("close", () => {
  if (!pendingPromotion) return;
  const { resolve } = pendingPromotion;
  pendingPromotion = null;
  resolve(null);
});
document.querySelector("#saveSettingsBtn").addEventListener("click", saveSettings);
bindBusyButton("#profilesBtn", openProfilesEditor);
bindBusyButton("#keychainBtn", saveProviderKeyToKeychain);
bindBusyButton("#healthBtn", async () => {
  try {
    document.querySelector("#settingsOutput").textContent = JSON.stringify(await call("HealthCheckProvider"), null, 2);
  } catch (err) {
    showError(err, "#settingsOutput");
  }
});
bindBusyButton("#benchBtn", async () => {
  try {
    document.querySelector("#settingsOutput").textContent = "Running benchmark...";
    document.querySelector("#settingsOutput").textContent = renderBenchmarkSummary(await call("RunRandomBenchmark", 100, 64));
  } catch (err) {
    showError(err, "#settingsOutput");
  }
});
bindBusyButton("#modeBenchBtn", async () => {
  try {
    document.querySelector("#settingsOutput").textContent = "Running mode benchmark...";
    document.querySelector("#settingsOutput").textContent = renderBenchmarkSummary(await call("RunModeBenchmark", 10, 64));
  } catch (err) {
    showError(err, "#settingsOutput");
  }
});
bindBusyButton("#accessibilityAuditBtn", runAccessibilityAudit);
bindBusyButton("#refreshReviewBtn", refreshReview);
bindBusyButton("#refreshStudyBtn", refreshStudy);
bindBusyButton("#multiAgentBtn", refreshMultiAgent);
bindBusyButton("#saveMemoryBtn", saveStudyMemory);
bindBusyButton("#newChess960Btn", newChess960Game);
bindBusyButton("#customBoardBtn", startCustomBoardFromLab);
bindBusyButton("#createBackupBtn", createBackup);
bindBusyButton("#restoreBackupBtn", restoreBackup);
bindBusyButton("#fineTuneBtn", exportFineTuneWorkflow);
bindBusyButton("#trainPolicyPriorBtn", trainPolicyPriorFromLab);
bindBusyButton("#enablePolicyPriorBtn", enablePolicyPriorFromLab);
bindBusyButton("#importBookBtn", importOpeningBookFromLab);
bindBusyButton("#labTournamentBtn", runTournamentFromLab);
bindBusyButton("#modeCompareBtn", compareAnalysisModes);
bindBusyButton("#promptCompareBtn", comparePromptPlayground);
bindBusyButton("#personalityBuilderBtn", buildPersonalityFromLab);
bindBusyButton("#providerDashboardBtn", () => runExperiment(
  () => call("ProviderDashboard"),
  "Checking providers",
  renderProviderDashboard
));
bindBusyButton("#positionSuiteBtn", () => runExperiment(
  () => call("RunPositionSuite", []),
  "Running position suite",
  renderPositionSuiteSummary
));
bindBusyButton("#providerComparisonBtn", () => runExperiment(
  () => call("RunProviderComparison"),
  "Comparing providers",
  renderProviderComparison
));
bindBusyButton("#tournamentBtn", () => runExperiment(
  () => call("RunTournament", 1, 64),
  "Running tournament",
  renderTournament
));
bindBusyButton("#experimentBenchBtn", () => runExperiment(
  () => call("RunRandomBenchmark", 100, 64),
  "Running random benchmark",
  renderBenchmarkSummary
));
bindBusyButton("#experimentModeBtn", () => runExperiment(
  () => call("RunModeBenchmark", 10, 64),
  "Running mode benchmark",
  renderBenchmarkSummary
));
bindBusyButton("#reloadPromptBtn", loadPromptEditor);
bindBusyButton("#validatePromptBtn", validatePromptEditor);
bindBusyButton("#runPromptPlaygroundBtn", runPromptPlaygroundFromEditor);
bindBusyButton("#savePromptBtn", savePromptEditor);
bindBusyButton("#exportProfilesBtn", exportProfiles);
bindBusyButton("#importProfilesBtn", importProfiles);
async function refreshExport() {
  const type = document.querySelector("#exportType").value;
  if (type === "fen") {
    document.querySelector("#exportText").value = await call("ExportFEN");
    return true;
  }
  if (type === "trace") {
    document.querySelector("#exportText").value = await call("ExportTrace");
    return true;
  }
  if (type === "debug_trace") {
    if (!confirmDebugTraceExport()) return false;
    document.querySelector("#exportText").value = await call("ExportDebugTrace");
    return true;
  }
  if (type === "fine_tune") {
    const workflow = await call("ExportFineTuneDataset");
    document.querySelector("#exportText").value = workflow.dataset_jsonl || "";
    return true;
  }
  document.querySelector("#exportText").value = await call("ExportPGN");
  return true;
}

function confirmDebugTraceExport() {
  return window.confirm("Debug trace export may include raw prompts or raw LLM responses when raw logging is enabled. Continue?");
}

bindBusyButton("#refreshExportBtn", async () => {
  try {
    await refreshExport();
  } catch (err) {
    showError(err, "#exportText");
  }
});
document.querySelector("#exportType").addEventListener("change", async () => {
  await withBusyControl("#exportType", async () => {
    try {
      await refreshExport();
    } catch (err) {
      showError(err, "#exportText");
    }
  });
});
bindBusyButton("#exportBtn", async () => {
  const exportDialog = document.querySelector("#exportDialog");
  document.querySelector("#exportText").value = "Preparing export...";
  if (!exportDialog.open) exportDialog.showModal();
  try {
    if (!await refreshExport()) {
      document.querySelector("#exportText").value = "Export canceled.";
    }
  } catch (err) {
    showError(err, "#exportText");
  }
});

window.addEventListener("keydown", (event) => {
  if (handleBoardKeyboard(event)) return;
  if (shouldIgnoreGlobalShortcut(event)) return;
  if (event.key === "n" || event.key === "N") document.querySelector("#newGameBtn").click();
  if (event.key === "a" || event.key === "A") document.querySelector("#analyzeBtn").click();
  if (event.key === " ") { event.preventDefault(); askEngine(); }
  if (event.key === "r" || event.key === "R") document.querySelector("#resignBtn").click();
  if (event.key === "u" || event.key === "U") document.querySelector("#undoBtn").click();
  if (event.key === "f" || event.key === "F") document.querySelector("#flipBtn").click();
  if (event.key === ",") document.querySelector("#settingsBtn").click();
});

function shouldIgnoreGlobalShortcut(event) {
  const target = event.target;
  if (!target?.matches) return false;
  if (target.closest("dialog[open]")) return true;
  if (target.isContentEditable) return true;
  return target.matches("input, textarea, select, button, a, [role='button']");
}

async function init() {
  subscribeDecisionStageEvents();
  try {
    await loadSettings();
  } catch (err) {
    settings = null;
  }
  await refresh();
}

init();
