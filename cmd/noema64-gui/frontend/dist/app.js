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
let activeWorkspaceView = "play";
let settings = null;
let playerSide = "white";
let autoReply = true;
let gameVariant = "standard";
let chess960Seed = 0;
let pendingPromotion = null;
let applyingProviderProfile = false;
let lastStageEvent = null;
let promptPack = null;
let reopenSettingsAfterProfiles = false;
let settingsScrollBeforeProfiles = 0;
const busyControls = new Set();
const busyDisabledState = new WeakMap();
let activeOperationCount = 0;
let activeThinkingOperationCount = 0;
let appActivitySequence = 0;
let activityEvents = [];
const maxActivityEvents = 30;
const workspaceViews = ["play", "study", "lab"];
const workflowCommandTargets = {
  analyze: "#analyzeBtn",
  engine: "#engineBtn",
  export: "#exportBtn",
  experiments: "#experimentsBtn",
  import: "#importBtn",
  lab: "#labBtn",
  newGame: "#newGameBtn",
  prompts: "#promptEditorBtn",
  recent: "#recentBtn",
  review: "#reviewBtn",
  settings: "#settingsBtn",
  study: "#studyBtn"
};
const primaryActionRequirements = {
  "#analyzeBtn": "game",
  "#engineBtn": "ongoing",
  "#experimentsBtn": "service",
  "#exportBtn": "game",
  "#flipBtn": "game",
  "#importBtn": "service",
  "#labBtn": "service",
  "#moveBtn": "ongoing",
  "#newGameBtn": "service",
  "#promptEditorBtn": "service",
  "#recentBtn": "service",
  "#resignBtn": "ongoing",
  "#reviewBtn": "game",
  "#settingsBtn": "service",
  "#stopBtn": "thinking",
  "#studyBtn": "game",
  "#undoBtn": "moves",
  "#whyBtn": "game"
};
const operationLabels = {
  "#accessibilityAuditBtn": "Accessibility audit",
  "#analyzeBtn": "Analysis",
  "#benchBtn": "Benchmark",
  "#createBackupBtn": "Backup",
  "#customBoardBtn": "Custom board",
  "#enablePolicyPriorBtn": "Policy prior",
  "#engineBtn": "Engine move",
  "#exportBtn": "Export",
  "#exportProfilesBtn": "Provider profile export",
  "#fineTuneBtn": "Fine-tune export",
  "#healthBtn": "Provider health",
  "#importBookBtn": "Opening book import",
  "#importProfilesBtn": "Provider profile import",
  "#keychainBtn": "Keychain save",
  "#labTournamentBtn": "Tournament",
  "#modeBenchBtn": "Mode benchmark",
  "#modeCompareBtn": "Mode comparison",
  "#multiAgentBtn": "Agent review",
  "#newChess960Btn": "Chess960 game",
  "#newGameBtn": "New game",
  "#personalityBuilderBtn": "Personality profile",
  "#positionSuiteBtn": "Position suite",
  "#profilesBtn": "Provider profiles",
  "#promptCompareBtn": "Prompt comparison",
  "#promptEditorBtn": "Prompt editor",
  "#providerComparisonBtn": "Provider comparison",
  "#providerDashboardBtn": "Provider dashboard",
  "#refreshExportBtn": "Export refresh",
  "#refreshReviewBtn": "Review refresh",
  "#refreshStudyBtn": "Study refresh",
  "#reloadPromptBtn": "Prompt reload",
  "#restoreBackupBtn": "Restore backup",
  "#reviewBtn": "Review",
  "#runImportBtn": "Import",
  "#runPromptPlaygroundBtn": "Prompt playground",
  "#saveMemoryBtn": "Strategy memory save",
  "#savePromptBtn": "Prompt save",
  "#saveSettingsBtn": "Settings save",
  "#settingsBtn": "Settings",
  "#studyBtn": "Study tools",
  "#trainPolicyPriorBtn": "Policy training",
  "#validatePromptBtn": "Prompt validation"
};
const dialogReturnFocusTargets = {
  activityDialog: "#activityHistoryBtn",
  exportDialog: "#exportBtn",
  experimentsDialog: "#experimentsBtn",
  importDialog: "#importBtn",
  labDialog: "#labBtn",
  profilesDialog: "#profilesBtn",
  promptDialog: "#promptEditorBtn",
  promotionDialog: "#moveInput",
  recentDialog: "#recentBtn",
  reviewDialog: "#reviewBtn",
  settingsDialog: "#settingsBtn",
  studyDialog: "#studyBtn"
};
const knownHumanizedTokens = {
  api: "API",
  fen: "FEN",
  json: "JSON",
  jsonl: "JSONL",
  llm: "LLM",
  openai: "OpenAI",
  pgn: "PGN",
  uci: "UCI"
};

const timeControlPresets = {
  untimed: { initial_ms: 0, increment_ms: 0 },
  bullet: { initial_ms: 60000, increment_ms: 0 },
  blitz: { initial_ms: 300000, increment_ms: 0 },
  rapid: { initial_ms: 600000, increment_ms: 0 },
  classical: { initial_ms: 1800000, increment_ms: 0 }
};

function asArray(value) {
  return Array.isArray(value) ? value : [];
}

function lastItem(value) {
  const items = asArray(value);
  return items.length ? items[items.length - 1] : undefined;
}

function asObject(value) {
  return value && typeof value === "object" ? value : {};
}

function asText(value, fallback = "") {
  if (typeof value === "string") return value;
  if (value === null || value === undefined) return fallback;
  return String(value);
}

function textAreaValue(value) {
  if (typeof value === "string") return value;
  if (value === null || value === undefined) return "";
  try {
    return JSON.stringify(value, null, 2);
  } catch (err) {
    return String(value);
  }
}

function normalizePromptPack(value) {
  const pack = asObject(value);
  return {
    schema_version: asText(pack.schema_version, "prompt-template-pack.v1"),
    source: asText(pack.source, "default"),
    manifest: asObject(pack.manifest),
    system: asText(pack.system),
    user: asText(pack.user),
    schema: asText(pack.schema)
  };
}

function gameStateFromResult(value) {
  if (value?.state && typeof value.state === "object") return value.state;
  return value && typeof value === "object" ? value : null;
}

function applyGameStateResult(value) {
  state = gameStateFromResult(value);
  resetBoardEntry();
  render();
  return state;
}

function defaultSettings() {
  return {
    engine: {
      default_mode: "blunderguard",
      personality: "balanced",
      custom_personality_id: "",
      custom_personalities: [],
      max_candidates: 5,
      trace_enabled: true
    },
    llm: {
      provider: "mock",
      endpoint: "",
      model: "mock-balanced",
      temperature: 0.2,
      max_tokens: 1600,
      timeout_ms: 12000,
      retries: 1,
      profile_id: "custom",
      profiles: []
    },
    verifier: {
      enabled: false,
      path: "",
      movetime_ms: 100,
      max_centipawn_loss: 180,
      tablebase_enabled: false,
      tablebase_path: "",
      tablebase_timeout_ms: 1000
    },
    gui: {
      theme: "system",
      time_control: "untimed",
      clock_initial_ms: 0,
      clock_increment_ms: 0
    },
    privacy: {
      cloud_provider_warning_acknowledged: false,
      log_raw_prompts: false,
      log_raw_llm_responses: false
    },
    logging: {
      output_dir: "logs"
    }
  };
}

function normalizeSettingsShape(value) {
  const defaults = defaultSettings();
  const source = value && typeof value === "object" ? value : {};
  const normalized = {
    ...source,
    engine: { ...defaults.engine, ...(source.engine && typeof source.engine === "object" ? source.engine : {}) },
    llm: { ...defaults.llm, ...(source.llm && typeof source.llm === "object" ? source.llm : {}) },
    verifier: { ...defaults.verifier, ...(source.verifier && typeof source.verifier === "object" ? source.verifier : {}) },
    gui: { ...defaults.gui, ...(source.gui && typeof source.gui === "object" ? source.gui : {}) },
    privacy: { ...defaults.privacy, ...(source.privacy && typeof source.privacy === "object" ? source.privacy : {}) },
    logging: { ...defaults.logging, ...(source.logging && typeof source.logging === "object" ? source.logging : {}) }
  };
  normalized.engine.custom_personalities = asArray(normalized.engine.custom_personalities);
  normalized.llm.profiles = asArray(normalized.llm.profiles);
  return normalized;
}

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
  setAppActivity("Needs attention", message, "error");
}

function showSuccess(message, selector = null) {
  if (selector) {
    const target = document.querySelector(selector);
    if (target) {
      if ("value" in target && (target.tagName === "TEXTAREA" || target.tagName === "INPUT")) {
        target.value = message;
      } else {
        target.textContent = message;
      }
      target.title = message;
    }
  }
  setAppActivity("Done", message, "success");
}

function setAppActivity(label, message, tone = "ready", record = true) {
  const activity = document.querySelector("#appActivity");
  if (!activity) return appActivitySequence;
  appActivitySequence += 1;
  activity.dataset.tone = tone;
  setText("#appActivityLabel", label);
  setText("#appActivityMessage", message);
  if (record) recordActivity(label, message, tone);
  return appActivitySequence;
}

function recordActivity(label, message, tone) {
  activityEvents.unshift({
    label: asText(label, "Status"),
    message: asText(message),
    tone: asText(tone, "ready"),
    at: new Date().toISOString()
  });
  activityEvents = activityEvents.slice(0, maxActivityEvents);
  if (document.querySelector("#activityDialog")?.open) renderActivityLog();
}

function renderActivityLog() {
  const log = document.querySelector("#activityLog");
  const clearButton = document.querySelector("#clearActivityBtn");
  if (clearButton) clearButton.disabled = activityEvents.length === 0;
  if (!log) return;
  log.innerHTML = "";
  if (!activityEvents.length) {
    const empty = document.createElement("div");
    empty.className = "activity-empty";
    empty.setAttribute("role", "listitem");
    empty.textContent = "No recent activity.";
    log.appendChild(empty);
    return;
  }
  for (const event of activityEvents) {
    const row = document.createElement("article");
    row.className = "activity-entry";
    row.dataset.tone = event.tone;
    row.setAttribute("role", "listitem");
    const formattedTime = formatActivityTime(event.at);
    const messageText = event.message || "No details.";
    row.setAttribute("aria-label", `${formattedTime} ${event.label}: ${messageText}`);
    const time = document.createElement("time");
    time.setAttribute("datetime", event.at);
    time.textContent = formattedTime;
    const body = document.createElement("div");
    const title = document.createElement("strong");
    title.textContent = event.label;
    const message = document.createElement("span");
    message.textContent = messageText;
    body.append(title, message);
    row.append(time, body);
    log.appendChild(row);
  }
}

function formatActivityTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown";
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function openActivityHistory() {
  renderActivityLog();
  document.querySelector("#activityDialog").showModal();
  focusDialogInitialControl("#clearActivityBtn:not(:disabled)", "#activityDialog button[value='cancel']");
}

function clearActivityHistory() {
  activityEvents = [];
  renderActivityLog();
  setAppActivity("Ready", "Activity history cleared.", "ready", false);
  focusDialogInitialControl("#activityDialog button[value='cancel']");
}

function beginAppOperation(label) {
  activeOperationCount += 1;
  document.body.dataset.appBusy = "true";
  return setAppActivity("Working", `${label}...`, "busy");
}

function finishAppOperation(marker) {
  activeOperationCount = Math.max(0, activeOperationCount - 1);
  if (activeOperationCount === 0) {
    delete document.body.dataset.appBusy;
  }
  if (marker === appActivitySequence) {
    setAppActivity("Ready", "Noema64 is ready.", "ready");
  }
}

function activityLabelFor(controlOrSelector, control) {
  if (typeof controlOrSelector === "string" && operationLabels[controlOrSelector]) return operationLabels[controlOrSelector];
  const element = control || controlFrom(controlOrSelector);
  const text = element?.getAttribute?.("aria-label") || element?.textContent || element?.title || "Action";
  return humanizeToken(text.replace(/\s+/g, " ").trim());
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
    if (!busyDisabledState.has(control)) {
      busyDisabledState.set(control, control.disabled);
    }
    control.disabled = true;
    control.setAttribute("aria-busy", "true");
  } else {
    busyControls.delete(key);
    const wasDisabled = busyDisabledState.get(control) || false;
    busyDisabledState.delete(control);
    control.disabled = wasDisabled;
    control.removeAttribute("aria-busy");
  }
  renderWorkflowPanel();
}

async function withBusyControl(controlOrSelector, action) {
  const control = controlFrom(controlOrSelector);
  const key = busyKey(controlOrSelector, control);
  if (!control || busyControls.has(key)) return undefined;
  const hadFocus = document.activeElement === control;
  const label = activityLabelFor(controlOrSelector, control);
  const marker = beginAppOperation(label);
  setControlBusy(controlOrSelector, true);
  try {
    return await action();
  } finally {
    setControlBusy(controlOrSelector, false);
    finishAppOperation(marker);
    if (hadFocus && document.activeElement === document.body) {
      restoreBusyControlFocus(control);
    }
  }
}

async function withThinkingControl(controlOrSelector, action) {
  const control = controlFrom(controlOrSelector);
  const key = busyKey(controlOrSelector, control);
  if (!control || busyControls.has(key)) return undefined;
  activeThinkingOperationCount += 1;
  setPrimaryActionAvailability(state?.snapshot);
  try {
    return await withBusyControl(controlOrSelector, action);
  } finally {
    activeThinkingOperationCount = Math.max(0, activeThinkingOperationCount - 1);
    setPrimaryActionAvailability(state?.snapshot);
  }
}

function restoreBusyControlFocus(control) {
  if (focusVisibleElement(control, true)) return;
  const dialog = control?.closest?.("dialog[open]");
  const scopedFallback = dialog?.querySelector([
    "input:not(:disabled)",
    "textarea:not(:disabled)",
    "select:not(:disabled)",
    "button:not(:disabled)",
    "a[href]",
    "[role='button']:not([aria-disabled='true'])"
  ].join(", "));
  if (focusVisibleElement(scopedFallback, true)) return;
  focusVisibleElement(
    document.querySelector("#newGameBtn:not(:disabled)") ||
    document.querySelector("[role='tab'][aria-selected='true']"),
    true
  );
}

function bindBusyButton(selector, action) {
  document.querySelector(selector).addEventListener("click", (event) => {
    event.preventDefault();
    withBusyControl(selector, () => action(event));
  });
}

function bindDialogCloseButtons() {
  document.querySelectorAll("dialog").forEach((dialog) => {
    dialog.addEventListener("close", () => restoreDialogFocus(dialog));
  });
  document.querySelectorAll("dialog button[value='cancel']").forEach((button) => {
    button.type = "button";
    button.addEventListener("click", () => {
      const dialog = button.closest("dialog");
      const restoreSettings = dialog?.id === "profilesDialog" && reopenSettingsAfterProfiles;
      closeDialogAndRestoreFocus(dialog, "cancel");
      if (restoreSettings) restoreSettingsAfterProfiles();
    });
  });
}

function closeDialogAndRestoreFocus(dialog, returnValue = undefined) {
  if (!dialog?.open) return;
  dialog.close(returnValue);
  restoreDialogFocus(dialog);
}

function restoreDialogFocus(dialog) {
  const restore = () => {
    const active = document.activeElement;
    const needsRestore = !active || active === document.body || !!active.closest?.("dialog:not([open])");
    if (!needsRestore) return;
    const preferred = document.querySelector(dialogReturnFocusTargets[dialog?.id]);
    const fallback = document.querySelector("[role='tab'][aria-selected='true']");
    const target = [preferred, fallback].find((element) => element && !element.disabled && isElementVisible(element));
    focusVisibleElement(target, true);
  };
  [0, 80, 250, 750, 1250].forEach((delay) => window.setTimeout(restore, delay));
}

function focusVisibleElement(element, preventScroll = false) {
  if (!element || element.disabled || !isElementVisible(element)) return false;
  try {
    if (preventScroll) {
      element.focus({ preventScroll: true });
    } else {
      element.focus();
    }
  } catch {
    element.focus();
  }
  return true;
}

function focusDialogInitialControl(selector, fallbackSelector = null) {
  window.setTimeout(() => {
    focusVisibleElement(document.querySelector(selector)) ||
      focusVisibleElement(document.querySelector(fallbackSelector));
  }, 0);
}

function setWorkspaceView(view, focus = false) {
  const next = workspaceViews.includes(view) ? view : "play";
  activeWorkspaceView = next;
  document.body.dataset.workspaceView = next;
  document.querySelectorAll("#workspaceNav button[data-workspace-target]").forEach((button) => {
    const selectedView = button.dataset.workspaceTarget === next;
    button.classList.toggle("active", selectedView);
    button.setAttribute("aria-selected", selectedView ? "true" : "false");
    button.tabIndex = selectedView ? 0 : -1;
    if (selectedView && focus) button.focus();
  });
  const selected = document.querySelector(`#workspaceNav button[data-workspace-target="${next}"]`);
  if (selected) document.querySelector("#mainWorkspace")?.setAttribute("aria-labelledby", selected.id);
  if (next !== "lab" && ["prompt", "raw"].includes(activeTab)) {
    activateTraceTab(document.querySelector("#summaryTab"));
  }
  renderWorkflowPanel();
}

function moveWorkspaceViewFocus(current, delta) {
  const buttons = [...document.querySelectorAll("#workspaceNav button[data-workspace-target]")];
  const index = buttons.indexOf(current);
  if (index < 0) return;
  const next = buttons[(index + delta + buttons.length) % buttons.length];
  setWorkspaceView(next.dataset.workspaceTarget, true);
}

function bindWorkspaceNavigation() {
  document.querySelectorAll("#workspaceNav button[data-workspace-target]").forEach((button) => {
    button.addEventListener("click", () => setWorkspaceView(button.dataset.workspaceTarget));
    button.addEventListener("keydown", (event) => {
      if (event.key === "ArrowRight" || event.key === "ArrowDown") {
        event.preventDefault();
        moveWorkspaceViewFocus(button, 1);
      }
      if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
        event.preventDefault();
        moveWorkspaceViewFocus(button, -1);
      }
      if (event.key === "Home") {
        event.preventDefault();
        setWorkspaceView(workspaceViews[0], true);
      }
      if (event.key === "End") {
        event.preventDefault();
        setWorkspaceView(workspaceViews[workspaceViews.length - 1], true);
      }
    });
  });
}

function bindWorkflowCommands() {
  document.querySelectorAll("#workflowPanel [data-command]").forEach((button) => {
    button.addEventListener("click", (event) => {
      event.preventDefault();
      runWorkflowCommand(button.dataset.command);
    });
  });
}

function runWorkflowCommand(command) {
  const target = document.querySelector(workflowCommandTargets[command]);
  if (!target || target.disabled) return;
  target.click();
}

function setText(selector, text) {
  const target = document.querySelector(selector);
  if (!target) return;
  target.textContent = text;
  target.title = text;
}

function renderWorkflowPanel() {
  const panel = document.querySelector("#workflowPanel");
  if (!panel) return;
  const view = workspaceViews.includes(activeWorkspaceView) ? activeWorkspaceView : "play";
  const snapshot = state?.snapshot || null;
  const decision = state?.last_decision || null;
  setText("#workflowEyebrow", workflowLabel(view));
  setText("#workflowTitle", workflowTitleText(view, snapshot, decision));
  setText("#workflowDetail", workflowDetailText(view, snapshot, decision));
  setText("#playFlowMeta", gameFlowMeta(snapshot));
  setText("#setupFlowMeta", setupFlowMeta());
  setText("#recordFlowMeta", recordFlowMeta(snapshot));
  setText("#reviewFlowMeta", decisionFlowMeta(decision));
  setText("#studyFlowMeta", studyFlowMeta(snapshot, decision));
  setText("#memoryFlowMeta", memoryFlowMeta());
  setText("#providerFlowMeta", providerFlowMeta());
  setText("#promptFlowMeta", promptFlowMeta());
  setText("#assetFlowMeta", assetFlowMeta(snapshot));
  setPrimaryActionAvailability(snapshot);
  setWorkflowActionAvailability(snapshot, decision);
}

function workflowLabel(view) {
  return view === "study" ? "Study" : view === "lab" ? "Lab" : "Play";
}

function workflowTitleText(view, snapshot, decision) {
  if (view === "study") return decision ? "Review Ready" : "Study Workspace";
  if (view === "lab") return "Developer Lab";
  if (!snapshot) return "Current Game";
  const outcome = snapshot.outcome?.status || "unknown";
  if (outcome !== "ongoing") return "Game Complete";
  return `${humanizeToken(snapshot.side_to_move || "unknown")} to move`;
}

function workflowDetailText(view, snapshot, decision) {
  if (view === "lab") return providerFlowMeta();
  if (view === "study") return decisionFlowMeta(decision);
  if (!snapshot) return "No game loaded";
  const fen = compactFen(snapshot.fen);
  return fen ? `${recordFlowMeta(snapshot)} · ${fen}` : gameFlowMeta(snapshot);
}

function gameFlowMeta(snapshot) {
  if (!snapshot) return "No game loaded";
  return statusSummary(snapshot, state?.variant?.variant || "standard");
}

function setupFlowMeta() {
  const mode = document.querySelector("#settingMode")?.value || settings?.engine?.default_mode || "blunderguard";
  const personality = document.querySelector("#settingPersonality")?.value || settings?.engine?.personality || "balanced";
  return `${humanizeToken(mode)} · ${humanizeToken(personality)} · ${timeControlMeta()}`;
}

function timeControlMeta() {
  const preset = document.querySelector("#settingTimeControl")?.value || settings?.gui?.time_control || "untimed";
  if (preset !== "custom") return humanizeToken(preset);
  const tc = timeControlForNewGame();
  return `${Math.round((tc.initial_ms || 0) / 60000)}+${Math.round((tc.increment_ms || 0) / 1000)}`;
}

function recordFlowMeta(snapshot) {
  if (!snapshot) return "No moves recorded";
  const plies = asArray(snapshot.move_history).length;
  return `${plies} ${plies === 1 ? "ply" : "plies"} · ${snapshot.outcome?.status || "unknown"}`;
}

function decisionFlowMeta(decision) {
  if (!decision) return "No decision recorded";
  const move = decision.selected_move?.san || decision.selected_move?.uci || "Decision recorded";
  const mode = decision.mode || settings?.engine?.default_mode || "mode";
  const fallback = decision.fallback_used ? "fallback" : "selected";
  return `${move} · ${humanizeToken(mode)} · ${fallback}`;
}

function studyFlowMeta(snapshot, decision) {
  if (!snapshot) return "No game loaded";
  return decision ? decisionFlowMeta(decision) : gameFlowMeta(snapshot);
}

function memoryFlowMeta() {
  const memory = state?.strategy_memory || {};
  const metrics = state?.strategy_metrics || {};
  const status = memory.plan?.status || "No plan";
  return `${humanizeToken(status)} · confidence ${formatMetric(memory.plan?.confidence)} · quality ${formatMetric(metrics.quality)}`;
}

function providerFlowMeta() {
  const provider = document.querySelector("#settingProvider")?.value || settings?.llm?.provider || "mock";
  const profile = document.querySelector("#settingProfile")?.value || settings?.llm?.profile_id || "custom";
  const model = document.querySelector("#settingModel")?.value || settings?.llm?.model || "mock-balanced";
  return `${humanizeToken(provider)} · ${profile || "custom"} · ${model}`;
}

function promptFlowMeta() {
  return promptPack?.source || "Default prompt pack";
}

function assetFlowMeta(snapshot) {
  const outputDir = document.querySelector("#settingLogDir")?.value || settings?.logging?.output_dir || "logs";
  return `${outputDir} · ${snapshot ? recordFlowMeta(snapshot) : "No game loaded"}`;
}

function setWorkflowActionAvailability(snapshot, decision) {
  document.querySelectorAll("#workflowPanel [data-requires]").forEach((button) => {
    const requirement = button.dataset.requires;
    const busy = workflowCommandBusy(button.dataset.command);
    const unavailable = workflowCommandUnavailable(button.dataset.command);
    button.disabled = busy || unavailable || (requirement === "game" && !snapshot) || (requirement === "decision" && !decision);
    button.toggleAttribute("aria-busy", busy);
  });
  document.querySelectorAll("#workflowPanel [data-command]:not([data-requires])").forEach((button) => {
    const busy = workflowCommandBusy(button.dataset.command);
    button.disabled = busy || workflowCommandUnavailable(button.dataset.command);
    button.toggleAttribute("aria-busy", busy);
  });
}

function setPrimaryActionAvailability(snapshot) {
  const hasService = !!api();
  const hasGame = !!snapshot;
  const ongoing = hasGame && snapshot.outcome?.status === "ongoing";
  const hasMoves = hasGame && asArray(snapshot.move_history).length > 0;
  for (const [selector, requirement] of Object.entries(primaryActionRequirements)) {
    const control = document.querySelector(selector);
    if (!control || busyControls.has(selector)) continue;
    control.disabled =
      (requirement === "service" && !hasService) ||
      (requirement === "game" && (!hasService || !hasGame)) ||
      (requirement === "ongoing" && (!hasService || !ongoing)) ||
      (requirement === "moves" && (!hasService || !hasMoves)) ||
      (requirement === "thinking" && (!hasService || activeThinkingOperationCount === 0));
  }
  const moveInput = document.querySelector("#moveInput");
  if (moveInput) moveInput.disabled = !hasService || !ongoing;
}

function workflowCommandBusy(command) {
  const target = workflowCommandTargets[command];
  return !!target && busyControls.has(target);
}

function workflowCommandUnavailable(command) {
  const target = workflowCommandTargets[command];
  const control = target ? document.querySelector(target) : null;
  return !!control?.disabled;
}

function resetBoardEntry() {
  selected = null;
  const moveInput = document.querySelector("#moveInput");
  moveInput.value = "";
  clearFieldInvalid(moveInput);
}

function focusMoveInput(select = false) {
  const input = document.querySelector("#moveInput");
  if (!focusVisibleElement(input)) return;
  if (select) input.select?.();
}

function parseJSONField(selector, label) {
  const field = document.querySelector(selector);
  try {
    const value = JSON.parse(field.value);
    clearFieldInvalid(field);
    return value;
  } catch (err) {
    markFieldInvalid(field);
    field.focus();
    field.select?.();
    throw new Error(`${label} must be valid JSON. ${appErrorMessage(err)}`);
  }
}

function requireField(selector, message) {
  const field = document.querySelector(selector);
  const value = String(field?.value || "").trim();
  if (value) {
    clearFieldInvalid(field);
    return value;
  }
  markFieldInvalid(field);
  field?.focus();
  throw new Error(message);
}

function markFieldInvalid(field) {
  if (!field) return;
  field.setAttribute("aria-invalid", "true");
  const describedBy = statusDescriberForField(field);
  if (describedBy) {
    addDescribedBy(field, describedBy);
    field.dataset.invalidDescribedBy = describedBy;
  }
  const clear = () => clearFieldInvalid(field);
  field.addEventListener("input", clear, { once: true });
  field.addEventListener("change", clear, { once: true });
}

function clearFieldInvalid(field) {
  if (!field) return;
  field.removeAttribute("aria-invalid");
  const describedBy = field.dataset.invalidDescribedBy;
  if (describedBy) removeDescribedBy(field, describedBy);
  delete field.dataset.invalidDescribedBy;
}

function statusDescriberForField(field) {
  return field?.closest?.("dialog")?.querySelector?.("[role='status'][id]")?.id ||
    field?.closest?.("section")?.querySelector?.("[role='status'][id]")?.id ||
    "";
}

function addDescribedBy(field, id) {
  const ids = new Set(String(field.getAttribute("aria-describedby") || "").split(/\s+/).filter(Boolean));
  ids.add(id);
  field.setAttribute("aria-describedby", [...ids].join(" "));
}

function removeDescribedBy(field, id) {
  const ids = String(field.getAttribute("aria-describedby") || "").split(/\s+/).filter((value) => value && value !== id);
  if (ids.length) {
    field.setAttribute("aria-describedby", ids.join(" "));
  } else {
    field.removeAttribute("aria-describedby");
  }
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
  normalizeRenderableState();
  renderBoard();
  renderStatus();
  renderMoves();
  renderStrategy();
  renderDecision();
  renderWorkflowPanel();
}

function normalizeRenderableState() {
  const snapshot = state.snapshot;
  snapshot.board = snapshot.board && typeof snapshot.board === "object" ? snapshot.board : {};
  snapshot.legal_moves = asArray(snapshot.legal_moves);
  snapshot.move_history = asArray(snapshot.move_history);
  snapshot.outcome = snapshot.outcome && typeof snapshot.outcome === "object" ? snapshot.outcome : { status: "unknown" };
  state.clock = state.clock && typeof state.clock === "object" ? state.clock : {};
  state.variant = state.variant && typeof state.variant === "object" ? state.variant : {};
}

function renderUnavailableState(message) {
  state = null;
  selected = null;
  renderBoardEmpty("Board unavailable", "Open Noema64 through the desktop app to connect the game service.");
  const status = document.querySelector("#statusText");
  status.textContent = message || "Game service unavailable.";
  status.title = status.textContent;
  setAppActivity("Unavailable", status.textContent, "error");
  document.querySelector("#clockText").textContent = "Untimed";
  document.querySelector("#modeText").textContent = settings?.engine?.default_mode || "offline";
  document.querySelector("#thinkingStage").textContent = "Unavailable";
  const tab = document.querySelector("#tabContent");
  tab.textContent = "Decision traces will appear after the game service is connected and the engine makes a decision.";
  tab.classList.add("empty-copy");
  renderCandidatesEmpty("Candidate moves are unavailable until a game is loaded.");
  document.querySelector("#confidence").textContent = "0.00";
  renderStrategyRows([
    ["Status", "Unavailable"],
    ["Plan", "Desktop game service unavailable."]
  ]);
  renderMoveListEmpty("No moves recorded.");
  renderWorkflowPanel();
}

function renderBoardEmpty(title, detail) {
  const board = document.querySelector("#board");
  const message = detail ? `${title}. ${detail}` : title;
  board.innerHTML = "";
  board.classList.add("board-empty-state");
  board.style.setProperty("--board-files", 8);
  board.style.setProperty("--board-ranks", 8);
  board.style.aspectRatio = "1 / 1";
  board.setAttribute("aria-rowcount", "8");
  board.setAttribute("aria-colcount", "8");
  board.setAttribute("aria-label", message);
  const row = document.createElement("div");
  row.className = "board-empty-row";
  row.setAttribute("role", "row");
  row.setAttribute("aria-rowindex", "1");
  const empty = document.createElement("div");
  empty.className = "board-empty";
  empty.setAttribute("role", "gridcell");
  empty.setAttribute("aria-colindex", "1");
  empty.setAttribute("aria-colspan", "8");
  empty.textContent = message;
  row.appendChild(empty);
  board.appendChild(row);
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
  if (!state?.snapshot) {
    renderBoardEmpty("Board unavailable", "Open Noema64 through the desktop app to connect the game service.");
    return;
  }
  const board = document.querySelector("#board");
  board.innerHTML = "";
  board.classList.remove("board-empty-state");
  board.setAttribute("aria-label", "Chess board");
  const dims = boardDimensions();
  const order = squareOrder();
  if (!order.includes(focusedSquare)) focusedSquare = order[0] || "a1";
  board.style.setProperty("--board-files", dims.width);
  board.style.setProperty("--board-ranks", dims.height);
  board.style.aspectRatio = `${dims.width} / ${dims.height}`;
  board.setAttribute("aria-rowcount", String(dims.height));
  board.setAttribute("aria-colcount", String(dims.width));
  const legalMoves = asArray(state.snapshot.legal_moves);
  const legalTargets = selected
    ? legalMoves.filter((m) => m?.from === selected).map((m) => m?.to)
    : [];
  const last = lastItem(state.snapshot.move_history);
  const lastSquares = splitUCIMoveSquares(last?.uci);
  let row = null;
  for (const [index, sq] of order.entries()) {
    if (index % dims.width === 0) {
      row = document.createElement("div");
      row.className = "board-row";
      row.setAttribute("role", "row");
      row.setAttribute("aria-rowindex", String(Math.floor(index / dims.width) + 1));
      board.appendChild(row);
    }
    const parsed = parseBoardSquare(sq, dims);
    const file = dims.files.indexOf(parsed?.file);
    const rank = dims.ranks.indexOf(parsed?.rank);
    const div = document.createElement("button");
    div.className = `square ${(file + rank) % 2 === 0 ? "dark" : "light"}`;
    div.setAttribute("role", "gridcell");
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
    row.appendChild(div);
  }
  renderBoardOverlay(board);
}

function renderBoardOverlay(board) {
  const candidates = asArray(state?.last_decision?.candidate_moves);
  if (!candidates.length) return;
  const overlay = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  overlay.setAttribute("class", "board-overlay");
  overlay.setAttribute("viewBox", "0 0 100 100");
  overlay.setAttribute("aria-hidden", "true");
  for (const [index, candidate] of candidates.slice(0, 4).entries()) {
    const move = candidate?.legal_move || candidate || {};
    const parsed = splitUCIMoveSquares(candidate?.uci || move?.uci);
    const from = move?.from || parsed?.from;
    const to = move?.to || parsed?.to;
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

function focusBoardSquare(square = focusedSquare) {
  const target = document.querySelector(`[data-square="${square}"]`);
  if (target) target.focus();
}

async function squareClicked(sq, fromKeyboard = false) {
  if (!state?.snapshot) return;
  boardKeyboardMode = !!fromKeyboard;
  if (!selected) {
    if (state.snapshot.board[sq]) selected = sq;
    renderBoard();
    focusBoardSquare(sq);
    return;
  }
  await playFromTo(selected, sq);
}

async function playFromTo(from, to) {
  const matches = asArray(state?.snapshot?.legal_moves).filter((m) => m?.from === from && m?.to === to);
  if (!matches.length) {
    selected = state?.snapshot?.board?.[to] ? to : null;
    renderBoard();
    focusBoardSquare(to);
    return;
  }
  let legal = matches[0];
  if (matches.length > 1) {
    legal = await choosePromotion(matches);
    if (!legal) {
      renderBoard();
      focusBoardSquare(from);
      return;
    }
  }
  selected = null;
  await makeMove(legal.uci);
  focusBoardSquare(to);
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
  if (asArray(state?.snapshot?.legal_moves).some((m) => m?.from === selected && m?.to === sq)) {
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
    focusBoardSquare();
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
  focusBoardSquare();
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
    focusDialogInitialControl("#promotionGrid button");
  });
}

function renderPromotionChoices(moves) {
  const grid = document.querySelector("#promotionGrid");
  grid.innerHTML = "";
  const seen = new Set();
  for (const move of asArray(moves)) {
    const promotion = move?.promotion;
    if (!promotion || seen.has(promotion)) continue;
    seen.add(promotion);
    const button = document.createElement("button");
    button.type = "button";
    button.dataset.promotion = promotion;
    const label = promotionTitle(promotion);
    button.title = label;
    button.setAttribute("aria-label", label);
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
  closeDialogAndRestoreFocus(document.querySelector("#promotionDialog"));
  resolve(asArray(moves).find((m) => m?.promotion === promotion) || null);
}

function renderMoves() {
  const list = document.querySelector("#moveList");
  list.innerHTML = "";
  list.classList.remove("move-list-empty");
  if (!state.snapshot.move_history.length) {
    renderMoveListEmpty("No moves recorded.");
    return;
  }
  for (const move of asArray(state.snapshot.move_history)) {
    const item = document.createElement("li");
    const san = move?.san || (typeof move === "string" ? move : "move");
    const uci = move?.uci || "";
    item.textContent = uci ? `${san} (${uci})` : san;
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
  const targets = mem.targets || {};
  document.querySelector("#confidence").textContent = Number(mem.plan?.confidence || 0).toFixed(2);
  const rows = [
    ["Plan", mem.plan?.summary],
    ["Status", mem.plan?.status],
    ["Phase", mem.phase],
    ["Quality", formatMetric(metrics.quality)],
    ["Drift", `${formatMetric(metrics.drift)} · ${metrics.alert_level || "none"}`],
    ["Alerts", formatStrategyAlerts(metrics.alerts)],
    ["Targets", [...asArray(targets.squares), ...asArray(targets.pieces), ...asArray(targets.pawns)].join(", ")],
    ["Opponent", mem.opponent_model?.likely_plan],
    ["Warnings", asArray(mem.tactical_warnings).join("; ")],
    ["Commitments", asArray(mem.commitments).join("; ")],
    ["Triggers", asArray(mem.refutation_triggers).map((t) => t?.condition || t).join("; ")],
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
  const items = asArray(alerts);
  if (!items.length) return "None";
  return items.map(formatStrategyAlert).filter(Boolean).join("; ") || "None";
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
    .map((word) => knownHumanizedTokens[word.toLowerCase()] || word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

function renderDecision() {
  const dec = state?.last_decision;
  const candidates = asArray(dec?.candidate_moves);
  document.querySelector("#thinkingStage").textContent = dec ? stageStatusText(lastDecisionStage(dec), "Decision finished") : "Idle";
  const tab = document.querySelector("#tabContent");
  tab.textContent = tabText(dec);
  tab.classList.toggle("empty-copy", !dec);
  const box = document.querySelector("#candidates");
  box.innerHTML = "";
  box.setAttribute("role", "list");
  box.setAttribute("aria-label", "Candidate moves");
  if (!candidates.length) {
    renderCandidatesEmpty(dec ? "No candidate moves recorded." : "No candidates recorded.");
    return;
  }
  box.classList.remove("empty-copy");
  for (const c of candidates) {
    const div = document.createElement("div");
    div.className = c?.rank === 1 ? "candidate top-candidate" : "candidate";
    div.setAttribute("role", "listitem");
    const move = document.createElement("strong");
    move.textContent = c?.san || c?.uci || "candidate";
    const detail = document.createElement("span");
    detail.append(document.createTextNode(candidatePurpose(c)));
    detail.append(document.createElement("br"));
    const meta = document.createElement("small");
    meta.textContent = `conf ${formatScore(c?.confidence)} · plan ${formatScore(c?.plan_alignment_score)} · style ${formatScore(c?.personality_score)} · search ${formatScore(c?.search_score)} · verifier ${c?.verifier_score?.status || "not_checked"}`;
    detail.append(meta);
    if (c?.risk) {
      detail.append(document.createElement("br"));
      const risk = document.createElement("small");
      risk.textContent = c.risk;
      detail.append(risk);
    }
    const score = document.createElement("small");
    score.className = "candidate-score";
    score.textContent = `#${c?.rank || "-"} ${formatScore(c?.final_score)}`;
    div.append(move, detail, score);
    box.appendChild(div);
  }
}

function renderCandidatesEmpty(message) {
  const box = document.querySelector("#candidates");
  box.innerHTML = "";
  box.classList.add("empty-copy");
  const empty = document.createElement("div");
  empty.setAttribute("role", "listitem");
  empty.textContent = message;
  box.appendChild(empty);
}

function formatScore(value) {
  const score = Number(value);
  return Number.isFinite(score) ? score.toFixed(2) : "0.00";
}

function tabText(dec) {
  if (!dec) return "No decision recorded.";
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
  const explanation = dec.explanation || candidatePurpose(asArray(dec.candidate_moves)[0], "Explanation unavailable.");
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
  const stages = asArray(dec?.stages);
  return stages.length ? stages[stages.length - 1] : null;
}

function stageSummary(dec) {
  const stages = asArray(dec?.stages);
  if (!stages.length) return "Stages: not recorded";
  return `Stages:\n${stages.map((stage) => `${stageLabel(stage?.name)} · ${stage?.status || "unknown"} · ${stage?.duration_ms || 0} ms`).join("\n")}`;
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
  const moveInput = document.querySelector("#moveInput");
  const normalizedMove = String(move || "").trim();
  if (!normalizedMove) {
    markFieldInvalid(moveInput);
    showError("Enter a UCI move before playing.");
    focusMoveInput();
    return;
  }
  return withBusyControl("#moveBtn", async () => {
    try {
      clearFieldInvalid(moveInput);
      document.querySelector("#thinkingStage").textContent = "Applying user move";
      applyGameStateResult(await call("MakeUserMove", normalizedMove));
      showSuccess("Move applied.");
      if (autoReply && state?.snapshot?.outcome?.status === "ongoing" && state.snapshot.side_to_move !== playerSide) {
        await askEngine();
      }
      focusMoveInput();
    } catch (err) {
      markFieldInvalid(moveInput);
      showError(err);
      document.querySelector("#thinkingStage").textContent = "Move failed";
      focusMoveInput(true);
    }
  });
}

async function askEngine() {
  return withThinkingControl("#engineBtn", async () => {
    try {
      lastStageEvent = null;
      document.querySelector("#thinkingStage").textContent = "Thinking: provider, repair, verifier, scoring";
      applyGameStateResult(await call("RequestEngineMove"));
      showSuccess("Engine move applied.");
    } catch (err) {
      showError(err);
      document.querySelector("#thinkingStage").textContent = "Engine stopped";
    }
  });
}

async function analyzeCurrentPosition() {
  return withThinkingControl("#analyzeBtn", async () => {
    try {
      lastStageEvent = null;
      document.querySelector("#thinkingStage").textContent = "Analyzing current position";
      const decision = await call("AnalyzeCurrentPosition");
      if (!state || typeof state !== "object") {
        state = { last_decision: decision };
      } else {
        state.last_decision = decision;
      }
      if (state?.snapshot) {
        renderStatus();
      } else {
        const status = document.querySelector("#statusText");
        status.textContent = "Analysis complete. No game state is loaded.";
        status.title = status.textContent;
      }
      renderDecision();
      renderWorkflowPanel();
      showSuccess("Analysis complete.");
    } catch (err) {
      showError(err);
      document.querySelector("#thinkingStage").textContent = "Analysis failed";
    }
  });
}

async function whyNotMove() {
  const moveInput = document.querySelector("#moveInput");
  const move = moveInput.value.trim();
  if (!move) {
    activateTraceTab(document.querySelector("#summaryTab"));
    markFieldInvalid(moveInput);
    showError("Enter a move to compare.");
    document.querySelector("#tabContent").textContent = "Enter a move to compare.";
    document.querySelector("#tabContent").classList.remove("empty-copy");
    moveInput.focus();
    return;
  }
  return withThinkingControl("#whyBtn", async () => {
    try {
      clearFieldInvalid(moveInput);
      const comparison = await call("WhyNotMove", move);
      activateTraceTab(document.querySelector("#summaryTab"));
      document.querySelector("#tabContent").textContent = whyNotText(comparison);
      document.querySelector("#tabContent").classList.remove("empty-copy");
      showSuccess("Move comparison ready.");
    } catch (err) {
      markFieldInvalid(moveInput);
      showError(err);
      focusMoveInput(true);
    }
  });
}

function whyNotText(comparison) {
  const data = asObject(comparison);
  return [
    data.summary || "No comparison available.",
    "",
    `Requested: ${data.requested_move || "unknown"}`,
    `Selected: ${data.selected_move || "unknown"}`,
    "",
    "REQUESTED CANDIDATE",
    JSON.stringify(asObject(data.requested), null, 2),
    "",
    "SELECTED CANDIDATE",
    JSON.stringify(asObject(data.selected), null, 2)
  ].join("\n");
}

function subscribeDecisionStageEvents() {
  if (!window.runtime?.EventsOn) return;
  window.runtime.EventsOn("decision.stage", (event) => {
    lastStageEvent = event;
    const text = stageStatusText(event, "Thinking");
    document.querySelector("#thinkingStage").textContent = text;
    setAppActivity("Thinking", text, "busy");
  });
}

async function resignGame() {
  if (!state?.snapshot || state.snapshot.outcome?.status !== "ongoing") return;
  const side = playerSide || state.snapshot.side_to_move || "white";
  if (!window.confirm(`Resign as ${side}?`)) return;
  return withBusyControl("#resignBtn", async () => {
    try {
      applyGameStateResult(await call("Resign", side));
      showSuccess("Game resigned.");
    } catch (err) {
      showError(err);
    }
  });
}

async function loadSettings() {
  settings = normalizeSettingsShape(await call("GetSettings"));
  document.querySelector("#settingsOutput").textContent = "";
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
  clearFieldInvalid(document.querySelector("#settingKey"));
  clearFieldInvalid(document.querySelector("#settingCloudAck"));
  syncTimeControlInputsFromPreset(false);
  syncProviderDisclosure();
  renderWorkflowPanel();
}

function populateProviderProfiles(profiles) {
  const select = document.querySelector("#settingProfile");
  select.innerHTML = "";
  const custom = document.createElement("option");
  custom.value = "custom";
  custom.textContent = "Custom";
  select.appendChild(custom);
  for (const profile of asArray(profiles)) {
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
  for (const profile of asArray(profiles)) {
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
  return asArray(settings?.llm?.profiles).find((profile) => profile?.id === profileID) || null;
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
  renderWorkflowPanel();
}

function markProviderProfileCustom() {
  if (applyingProviderProfile) return;
  document.querySelector("#settingProfile").value = "custom";
}

const settingsSaveErrorFields = [
  [/engine\.max_candidates|max candidates/i, "#settingMaxCandidates"],
  [/llm\.temperature|temperature/i, "#settingTemperature"],
  [/llm\.timeout_ms|llm timeout/i, "#settingTimeout"],
  [/verifier\.movetime_ms|verifier movetime|movetime/i, "#settingVerifierMoveTime"],
  [/verifier\.max_centipawn_loss|max centipawn loss/i, "#settingVerifierMaxLoss"],
  [/tablebase_path|tablebase path/i, "#settingTablebasePath"],
  [/tablebase_timeout|tablebase timeout/i, "#settingTablebaseTimeout"],
  [/verifier\.path|verifier path/i, "#settingVerifierPath"],
  [/logging\.output_dir|log output directory|output_dir/i, "#settingLogDir"]
];

function clearSettingsSaveErrorFields() {
  settingsSaveErrorFields.forEach(([, selector]) => clearFieldInvalid(document.querySelector(selector)));
}

function focusSettingsSaveError(err) {
  const message = appErrorMessage(err);
  const match = settingsSaveErrorFields.find(([pattern]) => pattern.test(message));
  const field = match ? document.querySelector(match[1]) : null;
  if (!field) {
    focusDialogInitialControl("#saveSettingsBtn");
    return;
  }
  markFieldInvalid(field);
  field.focus();
  field.select?.();
}

async function saveSettings() {
  return withBusyControl("#saveSettingsBtn", async () => {
    try {
      settings = normalizeSettingsShape(settings);
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
      const cloudAck = document.querySelector("#settingCloudAck");
      settings.privacy.cloud_provider_warning_acknowledged = cloudAck.checked;
      if (providerRequiresAck(settings.llm.provider) && !settings.privacy.cloud_provider_warning_acknowledged) {
        markFieldInvalid(cloudAck);
        showError("Acknowledge provider endpoint data sharing before saving.", "#settingsOutput");
        cloudAck.focus();
        return;
      }
      clearFieldInvalid(cloudAck);
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
      clearSettingsSaveErrorFields();
      showSuccess("Settings saved.", "#settingsOutput");
      renderWorkflowPanel();
      focusDialogInitialControl("#saveSettingsBtn");
    } catch (err) {
      showError(err, "#settingsOutput");
      focusSettingsSaveError(err);
    }
  });
}

async function saveProviderKeyToKeychain() {
  const keyField = document.querySelector("#settingKey");
  const apiKey = keyField.value.trim();
  if (!apiKey || apiKey === "[REDACTED]") {
    markFieldInvalid(keyField);
    showError("Enter an API key before saving it to the keychain.", "#settingsOutput");
    keyField.focus();
    return;
  }
  try {
    clearFieldInvalid(keyField);
    settings = await call("SaveProviderAPIKeyToKeychain", document.querySelector("#settingProfile").value, apiKey);
    await loadSettings();
    showSuccess("API key saved to keychain reference.", "#settingsOutput");
  } catch (err) {
    showError(err, "#settingsOutput");
  }
}

function syncProviderDisclosure() {
  const warning = document.querySelector("#cloudProviderWarning");
  const isCloud = providerRequiresAck(document.querySelector("#settingProvider").value);
  warning.classList.toggle("hidden", !isCloud);
  if (!isCloud) clearFieldInvalid(document.querySelector("#settingCloudAck"));
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
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString();
}

function gameRecordFields(record) {
  const source = record && typeof record === "object" ? record : {};
  const gameID = asText(source.game_id || source.GameID);
  const savedAt = source.saved_at || source.SavedAt || "";
  const stateValue = source.state || source.State || {};
  const gameState = stateValue && typeof stateValue === "object" ? stateValue : {};
  const snapshotValue = gameState.snapshot || gameState.Snapshot || {};
  const snapshot = snapshotValue && typeof snapshotValue === "object" ? snapshotValue : {};
  return { gameID, savedAt, snapshot };
}

function renderBenchmarkSummary(summary) {
  if (!summary) return "No benchmark result.";
  const results = asArray(summary.results);
  if (results.length && results[0]?.summary) {
    const rows = results.map((result) => {
      const s = result?.summary || {};
      return `${result?.mode || "mode"}: ${s.games_completed || 0}/${s.games_requested || summary.games_per_mode || 0} games · ${s.total_plies || 0} plies · ${s.fallbacks_used || 0} fallbacks · ${s.engine_errors || 0} errors`;
    });
    return `Mode benchmark · ${summary.games_per_mode || 0} games/mode · seed ${summary.seed || 64}\n${rows.join("\n")}`;
  }
  return `Random benchmark · ${summary.games_completed || 0}/${summary.games_requested || 0} games · ${summary.total_plies || 0} plies · ${summary.fallbacks_used || 0} fallbacks · ${summary.engine_errors || 0} errors`;
}

function renderPositionSuiteSummary(summary) {
  if (!summary) return "No position suite result.";
  const rows = asArray(summary.results).map((result) => {
    const move = result?.selected_san ? `${result.selected_san} (${result.selected_move})` : result?.selected_move || "none";
    const status = result?.engine_error ? `error: ${result.engine_error}` : `${move} · ${result?.candidate_count || 0} candidates · ${result?.duration_ms || 0} ms`;
    return `${result?.index || 0}. ${result?.name || "Position"} · ${result?.side_to_move || "unknown"} · ${status}`;
  });
  return [
    `Position suite · ${summary.positions_analyzed || 0}/${summary.positions_requested || 0} analyzed · ${summary.fallbacks_used || 0} fallbacks · ${summary.engine_errors || 0} errors`,
    ...rows
  ].join("\n");
}

function renderProviderComparison(summary) {
  if (!summary) return "No provider comparison result.";
  const rows = asArray(summary.results).map((result) => {
    const suite = result?.summary || {};
    return `${result?.profile_id || "profile"} · ${result?.status || "unknown"} · ${suite.positions_analyzed || 0}/${suite.positions_requested || 0} positions · ${suite.fallbacks_used || 0} fallbacks · ${result?.error || ""}`.trim();
  });
  return [
    `Provider comparison · ${summary.profiles_compared || 0} profiles completed · ${asArray(summary.positions).length} positions`,
    ...rows
  ].join("\n");
}

function renderProviderDashboard(dashboard) {
  if (!dashboard) return "No provider dashboard result.";
  const rows = asArray(dashboard.profiles).map((profile) => {
    const endpoint = profile?.endpoint ? ` · ${profile.endpoint}` : "";
    const error = profile?.error ? ` · ${profile.error}` : "";
    return `${profile?.id || "profile"} · ${humanizeToken(profile?.provider || "provider")} · ${profile?.model || "model"} · ${profile?.status || "unknown"}${endpoint}${error}`;
  });
  return [
    `Provider dashboard · active ${dashboard.active_profile || "custom"} · ${humanizeToken(dashboard.active_provider || "unknown")} · ${dashboard.active_model || "model"}`,
    ...rows
  ].join("\n");
}

function renderPromptPlayground(result) {
  if (!result) return "No prompt playground result.";
  const renderSide = (label, side) => {
    const candidates = asArray(side?.candidates).map((c) => `${c?.san || c?.uci || "candidate"} · ${c?.purpose || ""}`).join("\n");
    return [
      label,
      `${humanizeToken(side?.provider || "provider")} · ${side?.model || "model"} · valid ${!!side?.valid} · parse ${side?.parse_status || "none"}`,
      side?.error ? `Error: ${side.error}` : "",
      candidates || "No legal candidates parsed."
    ].filter(Boolean).join("\n");
  };
  return [
    `Prompt playground · ${result.game_id || "unknown"} · ply ${result.ply || 0}`,
    `Changed: ${asArray(result.comparison?.changed_files).join(", ") || "none"}`,
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
  const agents = asArray(multi.reviews).map((review) => `${review?.role || "agent"}: ${review?.summary || "No summary"} (${formatMetric(review?.confidence)})`).join("\n");
  const heat = asArray(dashboard.heatmap).map((item) => `${item?.move || "move"} -> ${item?.square || "square"} · ${formatScore(item?.weight)} · ${item?.label || "candidate"}`).join("\n");
  return [
    `Study · ${dashboard.game_id || "unknown"} · ply ${dashboard.ply || 0} · ${dashboard.variant?.variant || "standard"}`,
    `Memory: ${memory.plan_status || "unknown"} · confidence ${formatMetric(memory.plan_confidence)} · retained ${memory.retained_items || 0} · dropped ${memory.dropped_items || 0}`,
    `Coherence: ${coherence.status || "unknown"} · ${formatMetric(coherence.score)}`,
    `Diversity: ${diversity.status || "unknown"} · ${formatMetric(diversity.score)} · ${diversity.candidate_count || 0} candidates`,
    "",
    "LESSON",
    `${lesson.title || "Study"} · ${lesson.focus || ""}`,
    ...asArray(lesson.steps),
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
  const ratings = asArray(summary.ratings).map((rating) => `${rating?.id || "profile"} · ${formatMetric(rating?.elo)} · ${rating?.wins || 0}-${rating?.draws || 0}-${rating?.losses || 0}`).join("\n");
  const games = asArray(summary.results).slice(0, 12).map((game) => `${game?.game_index || 0}. ${game?.white_id || "white"}-${game?.black_id || "black"} · ${game?.outcome || "unknown"} · ${game?.winner_id || "draw"} · ${game?.plies || 0} plies`).join("\n");
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
  const filesList = asArray(manifest.files);
  const files = filesList.map((file) => `${file?.path || "file"} · ${file?.bytes || 0} bytes`).join("\n");
  return [
    `Archive: ${manifest.archive_path || "unknown"}`,
    `SHA-256: ${manifest.sha256 || "pending"}`,
    `Files: ${filesList.length} · bytes ${manifest.bytes || 0}`,
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
    ...asArray(spec.safety_notes),
    "",
    "JSONL",
    workflow.dataset_jsonl || ""
  ].join("\n");
}

function renderReview(review) {
  if (!review) return "No review available.";
  const metrics = review.strategy_metrics || {};
  const recommendations = asArray(review.recommendations);
  return [
    review.summary || "No review summary.",
    "",
    `Game: ${review.game_id || "unknown"} · ply ${review.ply || 0} · ${review.outcome_status || "unknown"}`,
    `Move: ${review.selected_move || "none"}`,
    `Provider: ${humanizeToken(review.provider || "none")} · mode ${review.mode || "none"}`,
    `Plan: ${review.plan || "none"}`,
    `Strategy quality: ${formatMetric(metrics.quality)} · completeness ${formatMetric(metrics.completeness)} · consistency ${formatMetric(metrics.consistency)} · drift ${formatMetric(metrics.drift)} · alerts ${metrics.alert_level || "none"}`,
    "",
    "POSITION SUMMARY",
    review.position_summary || "No position summary recorded.",
    "",
    "RECOMMENDATIONS",
    recommendations.length ? recommendations.join("\n") : "No recommendations."
  ].join("\n");
}

async function openReview() {
  document.querySelector("#reviewOutput").textContent = "Loading review...";
  document.querySelector("#reviewDialog").showModal();
  focusDialogInitialControl("#refreshReviewBtn");
  await refreshReview();
}

async function refreshReview() {
  try {
    document.querySelector("#reviewOutput").textContent = renderReview(await call("PostGameReview"));
    showSuccess("Review ready.");
  } catch (err) {
    showError(err, "#reviewOutput");
  }
}

async function openStudy() {
  document.querySelector("#studyOutput").textContent = "Loading study tools...";
  document.querySelector("#studyDialog").showModal();
  await refreshStudy();
  focusDialogInitialControl("#studyMemoryText");
}

async function refreshStudy() {
  try {
    const dashboard = await call("StudyDashboard");
    const current = await call("GetGame");
    const memoryField = document.querySelector("#studyMemoryText");
    memoryField.value = JSON.stringify(current?.strategy_memory || {}, null, 2);
    clearFieldInvalid(memoryField);
    document.querySelector("#studyOutput").textContent = renderStudyDashboard(dashboard);
    showSuccess("Study dashboard ready.");
  } catch (err) {
    showError(err, "#studyOutput");
  }
}

async function refreshMultiAgent() {
  try {
    document.querySelector("#studyOutput").textContent = JSON.stringify(await call("MultiAgentAnalysis"), null, 2);
    showSuccess("Agent review ready.");
  } catch (err) {
    showError(err, "#studyOutput");
  }
}

async function saveStudyMemory() {
  const memoryField = document.querySelector("#studyMemoryText");
  try {
    const memory = parseJSONField("#studyMemoryText", "Strategy memory");
    applyGameStateResult(await call("UpdateStrategyMemory", memory));
    clearFieldInvalid(memoryField);
    await refreshStudy();
    showSuccess("Strategy memory saved.");
  } catch (err) {
    markFieldInvalid(memoryField);
    showError(err, "#studyOutput");
    memoryField?.focus();
    memoryField?.select?.();
  }
}

function openExperiments() {
  document.querySelector("#experimentsOutput").textContent = "";
  document.querySelector("#experimentsDialog").showModal();
  focusDialogInitialControl("#providerDashboardBtn");
}

function openLab() {
  document.querySelector("#labOutput").textContent = "";
  document.querySelector("#tournamentGames").value = document.querySelector("#tournamentGames").value || "1";
  const customBoardDefinition = document.querySelector("#customBoardDefinition");
  if (!customBoardDefinition.value.trim()) {
    customBoardDefinition.value = JSON.stringify(defaultCustomBoardDefinition(), null, 2);
    clearFieldInvalid(customBoardDefinition);
  }
  document.querySelector("#labDialog").showModal();
  focusDialogInitialControl("#backupDir");
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
    applyGameStateResult(await call("NewGame", {
      side: playerSide || "white",
      variant: "chess960",
      seed,
      mode: document.querySelector("#settingMode")?.value || "blunderguard",
      personality: document.querySelector("#settingPersonality")?.value || "balanced",
      time_control: timeControlForNewGame()
    }));
    document.querySelector("#labOutput").textContent = JSON.stringify(state?.variant || {}, null, 2);
    showSuccess("Chess960 game ready.");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function startCustomBoardFromLab() {
  const customBoardDefinition = document.querySelector("#customBoardDefinition");
  let boardLoaded = false;
  try {
    const definition = parseJSONField("#customBoardDefinition", "Custom board definition");
    let side = document.querySelector("#settingSide")?.value || playerSide || "white";
    if (side === "random") side = Math.random() < 0.5 ? "white" : "black";
    playerSide = side;
    gameVariant = "custom";
    applyGameStateResult(await call("NewGame", {
      side,
      variant: "custom",
      board_definition: definition,
      mode: document.querySelector("#settingMode")?.value || "blunderguard",
      personality: document.querySelector("#settingPersonality")?.value || "balanced",
      time_control: timeControlForNewGame()
    }));
    boardLoaded = true;
    clearFieldInvalid(customBoardDefinition);
    document.querySelector("#labOutput").textContent = JSON.stringify(state?.variant || {}, null, 2);
    showSuccess("Custom board ready.");
    if (autoReply && side === "black") await askEngine();
  } catch (err) {
    if (!boardLoaded) {
      markFieldInvalid(customBoardDefinition);
      customBoardDefinition?.focus();
      customBoardDefinition?.select?.();
    }
    showError(err, "#labOutput");
  }
}

async function createBackup() {
  const backupDir = document.querySelector("#backupDir");
  try {
    const manifest = await call("CreateBackup", backupDir.value.trim());
    const restoreArchive = document.querySelector("#restoreArchive");
    restoreArchive.value = manifest?.archive_path || "";
    clearFieldInvalid(backupDir);
    if (restoreArchive.value) clearFieldInvalid(restoreArchive);
    document.querySelector("#labOutput").textContent = renderBackupManifest(manifest);
    showSuccess("Backup created.");
  } catch (err) {
    markFieldInvalid(backupDir);
    showError(err, "#labOutput");
    backupDir?.focus();
    backupDir?.select?.();
  }
}

async function restoreBackup() {
  const restoreArchive = document.querySelector("#restoreArchive");
  const restoreTarget = document.querySelector("#restoreTarget");
  try {
    const archive = requireField("#restoreArchive", "Choose a backup archive before restoring.");
    const target = requireField("#restoreTarget", "Choose a restore target before restoring.");
    const manifest = await call("RestoreBackup", archive, target);
    clearFieldInvalid(restoreArchive);
    clearFieldInvalid(restoreTarget);
    document.querySelector("#labOutput").textContent = renderBackupManifest(manifest);
    showSuccess("Backup restored.");
  } catch (err) {
    const message = appErrorMessage(err);
    const targetValue = String(restoreTarget?.value || "").trim();
    if (/restore target|target directory|restore directory/i.test(message) || (targetValue && message.includes(targetValue))) {
      markFieldInvalid(restoreTarget);
      showError(err, "#labOutput");
      restoreTarget?.focus();
      restoreTarget?.select?.();
      return;
    }
    markFieldInvalid(restoreArchive);
    showError(err, "#labOutput");
    restoreArchive?.focus();
    restoreArchive?.select?.();
  }
}

async function exportFineTuneWorkflow() {
  try {
    document.querySelector("#labOutput").textContent = renderFineTuneWorkflow(await call("ExportFineTuneDataset"));
    showSuccess("Fine-tune dataset ready.");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function runTournamentFromLab() {
  try {
    const games = Number(document.querySelector("#tournamentGames").value) || 1;
    document.querySelector("#labOutput").textContent = "Running tournament...";
    document.querySelector("#labOutput").textContent = renderTournament(await call("RunTournament", games, 64));
    showSuccess("Tournament complete.");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function compareAnalysisModes() {
  try {
    document.querySelector("#labOutput").textContent = "Comparing modes...";
    const comparison = await call("ComparePureHybridAnalysis");
    document.querySelector("#labOutput").textContent = [
      comparison?.summary || "No comparison summary.",
      "",
      "PURE",
      `${comparison?.pure?.selected_move?.san || comparison?.pure?.selected_move?.uci || "none"} · ${comparison?.pure?.explanation || ""}`,
      "",
      "HYBRID",
      `${comparison?.hybrid?.selected_move?.san || comparison?.hybrid?.selected_move?.uci || "none"} · ${comparison?.hybrid?.explanation || ""}`
    ].join("\n");
    showSuccess("Mode comparison ready.");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function comparePromptPlayground() {
  try {
    const base = normalizePromptPack(await call("PromptTemplatePack"));
    const variant = { ...base, source: "playground", user: `${base.user || ""}\n\nPrefer concise contrast between the top two candidates.\n` };
    document.querySelector("#labOutput").textContent = renderPromptPlayground(await call("RunPromptPlayground", base, variant));
    showSuccess("Prompt comparison ready.");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function runPromptPlaygroundFromEditor() {
  try {
    const base = normalizePromptPack(await call("PromptTemplatePack"));
    const edited = promptPackFromInputs();
    document.querySelector("#promptOutput").textContent = renderPromptPlayground(await call("RunPromptPlayground", base, edited));
    showSuccess("Prompt playground ready.");
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
    showSuccess("Personality profile saved.");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function trainPolicyPriorFromLab() {
  try {
    const workflow = await call("ExportFineTuneDataset");
    const policyModelPath = document.querySelector("#policyModelPath");
    const path = policyModelPath.value.trim() || "logs/policy-prior-model.json";
    const result = await call("TrainLocalPolicyPrior", workflow?.dataset_jsonl || "", path);
    policyModelPath.value = result?.model_path || path;
    clearFieldInvalid(policyModelPath);
    document.querySelector("#labOutput").textContent = JSON.stringify(result, null, 2);
    showSuccess("Policy prior trained.");
  } catch (err) {
    showError(err, "#labOutput");
  }
}

async function enablePolicyPriorFromLab() {
  const policyModelPath = document.querySelector("#policyModelPath");
  try {
    const path = requireField("#policyModelPath", "Enter a policy model path before enabling the prior.");
    settings = await call("EnablePolicyPriorModel", path);
    clearFieldInvalid(policyModelPath);
    await loadSettings();
    document.querySelector("#labOutput").textContent = `Policy prior enabled: ${settings.llm?.model || ""}`;
    showSuccess("Policy prior enabled.");
  } catch (err) {
    markFieldInvalid(policyModelPath);
    showError(err, "#labOutput");
    policyModelPath?.focus();
    policyModelPath?.select?.();
  }
}

async function importOpeningBookFromLab() {
  const openingBookPath = document.querySelector("#openingBookPath");
  try {
    const path = requireField("#openingBookPath", "Enter an opening book path before importing.");
    const book = await call("ImportOpeningBook", path);
    const suggestions = await call("OpeningBook");
    clearFieldInvalid(openingBookPath);
    document.querySelector("#labOutput").textContent = JSON.stringify({ imported: book, current_suggestions: suggestions }, null, 2);
    showSuccess("Opening book imported.");
  } catch (err) {
    markFieldInvalid(openingBookPath);
    showError(err, "#labOutput");
    openingBookPath?.focus();
    openingBookPath?.select?.();
  }
}

async function runAccessibilityAudit() {
  try {
    document.querySelector("#settingsOutput").textContent = JSON.stringify(await call("AccessibilityAudit"), null, 2);
    showSuccess("Accessibility audit complete.");
  } catch (err) {
    showError(err, "#settingsOutput");
  }
}

async function runExperiment(action, label, renderResult) {
  try {
    document.querySelector("#experimentsOutput").textContent = `${label}...`;
    document.querySelector("#experimentsOutput").textContent = renderResult(await action());
    showSuccess(`${label} complete.`);
  } catch (err) {
    showError(err, "#experimentsOutput");
  }
}

async function loadPromptEditor() {
  try {
    promptPack = normalizePromptPack(await call("PromptTemplatePack"));
    document.querySelector("#promptSource").value = promptPack.source || "default";
    document.querySelector("#promptManifest").value = JSON.stringify(promptPack.manifest || {}, null, 2);
    document.querySelector("#promptSystem").value = promptPack.system || "";
    document.querySelector("#promptUser").value = promptPack.user || "";
    document.querySelector("#promptSchema").value = promptPack.schema || "";
    document.querySelector("#promptOutput").textContent = "";
    clearFieldInvalid(document.querySelector("#promptManifest"));
    clearFieldInvalid(document.querySelector("#promptSystem"));
    clearFieldInvalid(document.querySelector("#promptUser"));
    clearFieldInvalid(document.querySelector("#promptSchema"));
    clearFieldInvalid(document.querySelector("#promptSaveDir"));
    renderWorkflowPanel();
    showSuccess("Prompt pack loaded.");
  } catch (err) {
    showError(err, "#promptOutput");
  }
}

async function openPromptEditor() {
  document.querySelector("#promptDialog").showModal();
  await loadPromptEditor();
  focusDialogInitialControl("#promptSaveDir");
}

function promptPackFromInputs() {
  const manifest = parseJSONField("#promptManifest", "Prompt manifest");
  parseJSONField("#promptSchema", "Prompt output schema");
  return {
    schema_version: "prompt-template-pack.v1",
    source: document.querySelector("#promptSource").value || "editor",
    manifest,
    system: document.querySelector("#promptSystem").value,
    user: document.querySelector("#promptUser").value,
    schema: document.querySelector("#promptSchema").value
  };
}

function promptValidationErrorMessage(validation) {
  const errors = asArray(validation?.errors).map((error) => String(error || "").trim()).filter(Boolean);
  return errors.length ? `Prompt pack validation failed: ${errors.join(" ")}` : "Prompt pack validation failed.";
}

function focusPromptValidationError(validation) {
  const message = asArray(validation?.errors).join(" ").toLowerCase();
  const selector = message.includes("schema")
    ? "#promptSchema"
    : message.includes("manifest")
      ? "#promptManifest"
      : message.includes("system")
        ? "#promptSystem"
        : message.includes("user") || message.includes("template") || message.includes("placeholder")
          ? "#promptUser"
          : null;
  const field = selector ? document.querySelector(selector) : null;
  markFieldInvalid(field);
  field?.focus();
  field?.select?.();
}

function renderPromptValidation(validation, successMessage) {
  document.querySelector("#promptOutput").textContent = JSON.stringify(validation, null, 2);
  if (validation?.valid === false) {
    showError(promptValidationErrorMessage(validation), "#promptOutput");
    focusPromptValidationError(validation);
    return false;
  }
  clearFieldInvalid(document.querySelector("#promptManifest"));
  clearFieldInvalid(document.querySelector("#promptSystem"));
  clearFieldInvalid(document.querySelector("#promptUser"));
  clearFieldInvalid(document.querySelector("#promptSchema"));
  showSuccess(successMessage);
  return true;
}

async function validatePromptEditor() {
  try {
    const validation = await call("ValidatePromptTemplatePack", promptPackFromInputs());
    renderPromptValidation(validation, "Prompt pack validated.");
    return validation;
  } catch (err) {
    showError(err, "#promptOutput");
    return null;
  }
}

async function savePromptEditor() {
  const promptSaveDir = document.querySelector("#promptSaveDir");
  let saveAttempted = false;
  try {
    const dir = requireField("#promptSaveDir", "Enter a save directory before saving the prompt pack.");
    const pack = promptPackFromInputs();
    saveAttempted = true;
    const validation = await call("SavePromptTemplatePack", dir, pack);
    if (renderPromptValidation(validation, "Prompt pack saved.")) {
      clearFieldInvalid(promptSaveDir);
    }
  } catch (err) {
    if (saveAttempted) {
      markFieldInvalid(promptSaveDir);
      promptSaveDir?.focus();
      promptSaveDir?.select?.();
    }
    showError(err, "#promptOutput");
  }
}

async function openProfilesEditor() {
  const dialog = document.querySelector("#profilesDialog");
  const settingsDialog = document.querySelector("#settingsDialog");
  const settingsForm = settingsDialog?.querySelector("form");
  const importButton = document.querySelector("#importProfilesBtn");
  const previousImportDisabled = importButton?.disabled || false;
  reopenSettingsAfterProfiles = !!settingsDialog?.open;
  settingsScrollBeforeProfiles = settingsForm?.scrollTop || 0;
  if (reopenSettingsAfterProfiles) settingsDialog.close("profiles");
  dialog.showModal();
  focusDialogInitialControl("#profilesText");
  if (importButton) importButton.disabled = true;
  try {
    await exportProfiles();
  } finally {
    if (importButton && !busyControls.has("#importProfilesBtn")) {
      importButton.disabled = previousImportDisabled;
    }
  }
}

function restoreSettingsAfterProfiles() {
  if (!reopenSettingsAfterProfiles) return;
  reopenSettingsAfterProfiles = false;
  window.setTimeout(() => {
    const settingsDialog = document.querySelector("#settingsDialog");
    const settingsForm = settingsDialog?.querySelector("form");
    if (settingsDialog && !settingsDialog.open) settingsDialog.showModal();
    if (settingsForm) settingsForm.scrollTop = settingsScrollBeforeProfiles;
    try {
      document.querySelector("#profilesBtn")?.focus({ preventScroll: true });
    } catch {
      document.querySelector("#profilesBtn")?.focus();
    }
  }, 0);
}

async function exportProfiles() {
  try {
    const profilesText = document.querySelector("#profilesText");
    profilesText.value = textAreaValue(await call("ExportProviderProfiles"));
    clearFieldInvalid(profilesText);
    document.querySelector("#profilesOutput").textContent = "Profiles exported.";
    showSuccess("Provider profiles exported.");
  } catch (err) {
    showError(err, "#profilesOutput");
  }
}

async function importProfiles() {
  const profilesText = document.querySelector("#profilesText");
  try {
    const text = requireField("#profilesText", "Paste provider profiles before importing.");
    settings = normalizeSettingsShape(await call("ImportProviderProfiles", text));
    clearFieldInvalid(profilesText);
    populateProviderProfiles(settings.llm?.profiles || []);
    document.querySelector("#profilesOutput").textContent = "Profiles imported.";
    renderWorkflowPanel();
    showSuccess("Provider profiles imported.");
  } catch (err) {
    markFieldInvalid(profilesText);
    showError(err, "#profilesOutput");
    profilesText.focus();
  }
}

function renderRecentGames(records) {
  const list = document.querySelector("#recentList");
  list.innerHTML = "";
  list.setAttribute("role", "list");
  const items = asArray(records);
  if (!items.length) {
    renderRecentGamesEmpty("No recent games.");
    return;
  }
  for (const record of items) {
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
    button.disabled = !gameID;
    button.title = gameID ? "" : "This recent game record is missing an id.";
    button.setAttribute("aria-label", gameID
      ? `Load ${title.textContent} from ${formatSavedAt(savedAt)}, ${outcomeStatus}`
      : `Cannot load ${title.textContent}; missing game id`);
    button.addEventListener("click", () => withBusyControl(button, async () => {
      try {
        applyGameStateResult(await call("LoadRecentGame", gameID));
        closeDialogAndRestoreFocus(document.querySelector("#recentDialog"));
        showSuccess("Recent game loaded.");
      } catch (err) {
        showError(err, "#recentOutput");
      }
    }));
    row.append(detail, button);
    list.appendChild(row);
  }
}

function renderRecentGamesEmpty(message) {
  const list = document.querySelector("#recentList");
  list.innerHTML = "";
  list.setAttribute("role", "list");
  const empty = document.createElement("div");
  empty.className = "recent-empty";
  empty.setAttribute("role", "listitem");
  empty.textContent = message;
  list.appendChild(empty);
}

async function openRecentGames() {
  try {
    document.querySelector("#recentOutput").textContent = "";
    renderRecentGamesEmpty("Loading recent games...");
    document.querySelector("#recentDialog").showModal();
    renderRecentGames(await call("RecentGames", 10));
    focusDialogInitialControl("#recentList button:not(:disabled)", "#recentDialog button[value='cancel']");
    showSuccess("Recent games loaded.");
  } catch (err) {
    renderRecentGamesEmpty("Recent games could not be loaded.");
    showError(err, "#recentOutput");
    focusDialogInitialControl("#recentDialog button[value='cancel']");
  }
}

function activateTraceTab(btn, focus = false) {
  const next = isTraceTabAvailable(btn) ? btn : document.querySelector("#summaryTab");
  document.querySelectorAll(".tabs button").forEach((b) => {
    const selectedTab = b === next;
    b.classList.toggle("active", selectedTab);
    b.setAttribute("aria-selected", selectedTab ? "true" : "false");
    b.tabIndex = selectedTab ? 0 : -1;
  });
  activeTab = next?.dataset.tab || "summary";
  document.querySelector("#tabContent")?.setAttribute("aria-labelledby", next?.id || "summaryTab");
  renderDecision();
  if (focus) next?.focus();
}

function isTraceTabAvailable(button) {
  if (!button) return false;
  return activeWorkspaceView === "lab" || button.dataset.labOnly !== "true";
}

function availableTraceTabs() {
  return [...document.querySelectorAll(".tabs button")].filter(isTraceTabAvailable);
}

function moveTraceTabFocus(current, delta) {
  const tabs = availableTraceTabs();
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
      activateTraceTab(availableTraceTabs()[0], true);
    }
    if (event.key === "End") {
      event.preventDefault();
      activateTraceTab(lastItem(availableTraceTabs()), true);
    }
  });
});

document.querySelector("#activityHistoryBtn").addEventListener("click", openActivityHistory);
document.querySelector("#clearActivityBtn").addEventListener("click", (event) => {
  event.preventDefault();
  clearActivityHistory();
});

bindBusyButton("#newGameBtn", async () => {
  try {
    let side = document.querySelector("#settingSide")?.value || playerSide;
    if (side === "random") side = Math.random() < 0.5 ? "white" : "black";
    playerSide = side;
    applyGameStateResult(await call("NewGame", {
      side,
      variant: document.querySelector("#settingVariant")?.value || gameVariant,
      seed: Number(document.querySelector("#settingVariantSeed")?.value) || chess960Seed,
      mode: document.querySelector("#settingMode")?.value || "blunderguard",
      personality: document.querySelector("#settingPersonality")?.value || "balanced",
      time_control: timeControlForNewGame()
    }));
    showSuccess("New game ready.");
    if (autoReply && side === "black") await askEngine();
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#settingTimeControl").addEventListener("change", () => {
  syncTimeControlInputsFromPreset(true);
  renderWorkflowPanel();
});
document.querySelector("#settingTheme").addEventListener("change", () => applyTheme(document.querySelector("#settingTheme").value));
document.querySelector("#settingProfile").addEventListener("change", applySelectedProviderProfile);
document.querySelector("#settingProvider").addEventListener("change", () => {
  markProviderProfileCustom();
  syncProviderDisclosure();
  renderWorkflowPanel();
});
["#settingEndpoint", "#settingModel", "#settingTemperature", "#settingMaxTokens", "#settingTimeout", "#settingRetries", "#settingKey", "#settingKeyRef"].forEach((selector) => {
  document.querySelector(selector).addEventListener("input", () => {
    markProviderProfileCustom();
    renderWorkflowPanel();
  });
});
["#settingMode", "#settingPersonality", "#settingLogDir", "#settingClockInitial", "#settingClockIncrement"].forEach((selector) => {
  document.querySelector(selector).addEventListener("input", renderWorkflowPanel);
});
bindBusyButton("#recentBtn", openRecentGames);
document.querySelector("#engineBtn").addEventListener("click", askEngine);
document.querySelector("#analyzeBtn").addEventListener("click", analyzeCurrentPosition);
bindBusyButton("#stopBtn", async () => {
  try {
    await call("StopEngine");
    document.querySelector("#thinkingStage").textContent = "Stop requested";
    showSuccess("Stop requested.");
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#resignBtn").addEventListener("click", resignGame);
bindBusyButton("#undoBtn", async () => {
  try {
    applyGameStateResult(await call("Undo", 1));
    showSuccess("Move undone.");
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
    focusDialogInitialControl("#settingMode");
  } catch (err) {
    showError(err);
  }
});
document.querySelector("#importBtn").addEventListener("click", () => {
  document.querySelector("#importOutput").textContent = "";
  clearFieldInvalid(document.querySelector("#importText"));
  document.querySelector("#importDialog").showModal();
  focusDialogInitialControl("#importText");
});
document.querySelector("#importText").addEventListener("keydown", (event) => {
  if (event.key !== "Enter" || (!event.ctrlKey && !event.metaKey)) return;
  event.preventDefault();
  document.querySelector("#runImportBtn").click();
});
bindBusyButton("#runImportBtn", async () => {
  const type = document.querySelector("#importType").value;
  const importText = document.querySelector("#importText");
  const text = importText.value;
  if (!text.trim()) {
    markFieldInvalid(importText);
    showError("Paste a FEN or PGN before importing.", "#importOutput");
    importText.focus();
    return;
  }
  try {
    clearFieldInvalid(importText);
    applyGameStateResult(type === "fen" ? await call("ImportFEN", text) : await call("ImportPGN", text));
    closeDialogAndRestoreFocus(document.querySelector("#importDialog"));
    showSuccess("Position imported.");
  } catch (err) {
    markFieldInvalid(importText);
    showError(err, "#importOutput");
    importText.focus();
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
    showSuccess("Provider health check complete.");
  } catch (err) {
    showError(err, "#settingsOutput");
  }
});
bindBusyButton("#benchBtn", async () => {
  try {
    document.querySelector("#settingsOutput").textContent = "Running benchmark...";
    document.querySelector("#settingsOutput").textContent = renderBenchmarkSummary(await call("RunRandomBenchmark", 100, 64));
    showSuccess("Random benchmark complete.");
  } catch (err) {
    showError(err, "#settingsOutput");
  }
});
bindBusyButton("#modeBenchBtn", async () => {
  try {
    document.querySelector("#settingsOutput").textContent = "Running mode benchmark...";
    document.querySelector("#settingsOutput").textContent = renderBenchmarkSummary(await call("RunModeBenchmark", 10, 64));
    showSuccess("Mode benchmark complete.");
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
document.querySelector("#profilesDialog").addEventListener("close", restoreSettingsAfterProfiles);
bindBusyButton("#exportProfilesBtn", exportProfiles);
bindBusyButton("#importProfilesBtn", importProfiles);
async function refreshExport() {
  const type = document.querySelector("#exportType").value;
  if (type === "fen") {
    finishExport(await call("ExportFEN"), "FEN export ready.");
    return true;
  }
  if (type === "trace") {
    finishExport(await call("ExportTrace"), "Trace export ready.");
    return true;
  }
  if (type === "debug_trace") {
    if (!confirmDebugTraceExport()) return false;
    finishExport(await call("ExportDebugTrace"), "Debug trace export ready.");
    return true;
  }
  if (type === "fine_tune") {
    const workflow = await call("ExportFineTuneDataset");
    finishExport(workflow?.dataset_jsonl, "Fine-tune export ready.");
    return true;
  }
  finishExport(await call("ExportPGN"), "PGN export ready.");
  return true;
}

function setExportText(value) {
  document.querySelector("#exportText").value = textAreaValue(value);
}

function setExportStatus(message) {
  const output = document.querySelector("#exportOutput");
  if (!output) return;
  output.textContent = message;
  output.title = message;
}

function finishExport(value, message) {
  setExportText(value);
  showSuccess(message, "#exportOutput");
}

function showExportError(err) {
  const message = appErrorMessage(err);
  setExportText(message);
  showError(message, "#exportOutput");
}

function confirmDebugTraceExport() {
  return window.confirm("Debug trace export may include raw prompts or raw LLM responses when raw logging is enabled. Continue?");
}

function showExportCanceled() {
  setExportText("Export canceled.");
  setExportStatus("Export canceled.");
  setAppActivity("Canceled", "Export canceled.", "ready");
}

bindBusyButton("#refreshExportBtn", async () => {
  try {
    if (!await refreshExport()) showExportCanceled();
  } catch (err) {
    showExportError(err);
  }
});
document.querySelector("#exportType").addEventListener("change", async () => {
  await withBusyControl("#exportType", async () => {
    try {
      if (!await refreshExport()) showExportCanceled();
    } catch (err) {
      showExportError(err);
    }
  });
});
bindBusyButton("#exportBtn", async () => {
  const exportDialog = document.querySelector("#exportDialog");
  setExportText("Preparing export...");
  setExportStatus("Preparing export...");
  if (!exportDialog.open) exportDialog.showModal();
  focusDialogInitialControl("#exportType");
  try {
    if (!await refreshExport()) {
      showExportCanceled();
    }
  } catch (err) {
    showExportError(err);
  }
});

window.addEventListener("keydown", (event) => {
  if (handleBoardKeyboard(event)) return;
  if (shouldIgnoreGlobalShortcut(event)) return;
  if (event.key === "n" || event.key === "N") triggerShortcut("#newGameBtn");
  if (event.key === "a" || event.key === "A") triggerShortcut("#analyzeBtn");
  if (event.key === " ") {
    event.preventDefault();
    triggerShortcut("#engineBtn");
  }
  if (event.key === "r" || event.key === "R") triggerShortcut("#resignBtn");
  if (event.key === "u" || event.key === "U") triggerShortcut("#undoBtn");
  if (event.key === "f" || event.key === "F") triggerShortcut("#flipBtn");
  if (event.key === ",") triggerShortcut("#settingsBtn");
});

function shouldIgnoreGlobalShortcut(event) {
  const target = event.target;
  if (!target?.matches) return false;
  if (target.closest("dialog[open]")) return true;
  if (target.isContentEditable) return true;
  return target.matches("input, textarea, select, button, a, [role='button']");
}

function triggerShortcut(selector) {
  const control = document.querySelector(selector);
  if (!control || control.disabled || !isElementVisible(control)) return false;
  control.click();
  return true;
}

function isElementVisible(element) {
  return !!(element?.offsetWidth || element?.offsetHeight || element?.getClientRects?.().length);
}

async function init() {
  bindWorkspaceNavigation();
  bindWorkflowCommands();
  setWorkspaceView(document.body.dataset.workspaceView || activeWorkspaceView);
  bindDialogCloseButtons();
  subscribeDecisionStageEvents();
  try {
    await loadSettings();
  } catch (err) {
    settings = null;
  }
  await refresh();
}

init();
