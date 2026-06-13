import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const dist = new URL("./dist/", import.meta.url);
const indexHTML = await readFile(new URL("index.html", dist), "utf8");
const appJS = await readFile(new URL("app.js", dist), "utf8");
const stylesCSS = await readFile(new URL("styles.css", dist), "utf8");

function expectMarkupID(id) {
  assert.match(indexHTML, new RegExp(`id="${id}"`), `missing #${id}`);
}

function expectScriptToken(token) {
  assert.ok(appJS.includes(token), `missing script token ${token}`);
}

test("main GUI screen exposes critical panels", () => {
  for (const id of ["workspaceNav", "viewPlayBtn", "viewStudyBtn", "viewLabBtn", "appActivity", "appActivityLabel", "appActivityMessage", "activityHistoryBtn", "activityDialog", "activityTitle", "clearActivityBtn", "activityLog", "mainWorkspace", "workflowPanel", "workflowEyebrow", "workflowTitle", "workflowDetail", "playFlowMeta", "setupFlowMeta", "recordFlowMeta", "reviewFlowMeta", "studyFlowMeta", "memoryFlowMeta", "providerFlowMeta", "promptFlowMeta", "assetFlowMeta", "board", "statusText", "clockText", "modeText", "moveInput", "moveList", "tabContent", "candidates", "strategyMemory"]) {
    expectMarkupID(id);
  }
  assert.match(indexHTML, /body data-workspace-view="play"/);
  assert.match(indexHTML, /id="workspaceNav" class="workspace-nav" role="tablist" aria-label="Workspace"/);
  assert.match(indexHTML, /id="viewPlayBtn"[\s\S]*data-workspace-target="play"[\s\S]*aria-selected="true"/);
  assert.match(indexHTML, /id="viewStudyBtn"[\s\S]*data-workspace-target="study"/);
  assert.match(indexHTML, /id="viewLabBtn"[\s\S]*data-workspace-target="lab"/);
  assert.match(indexHTML, /id="appActivity" class="app-activity" role="status" aria-live="polite" data-tone="ready"/);
  assert.match(indexHTML, /id="appActivityLabel">Ready</);
  assert.match(indexHTML, /id="appActivityMessage">Noema64 is ready\.</);
  assert.match(indexHTML, /id="activityHistoryBtn" class="activity-history-button" type="button" title="Activity history" aria-label="Activity history"/);
  assert.match(indexHTML, /dialog id="activityDialog" aria-labelledby="activityTitle"/);
  assert.match(indexHTML, /id="activityLog" class="activity-log" role="list" aria-label="Activity log" aria-live="polite"/);
  assert.match(indexHTML, /id="mainWorkspace" class="workspace" role="tabpanel" aria-labelledby="viewPlayBtn"/);
  assert.match(indexHTML, /id="workflowPanel" class="workflow-panel" aria-label="Workspace overview"/);
  assert.match(indexHTML, /data-workspace-card="play"/);
  assert.match(indexHTML, /data-workspace-card="study"/);
  assert.match(indexHTML, /data-workspace-card="lab"/);
  assert.match(indexHTML, /data-command="engine" data-requires="game"/);
  assert.match(indexHTML, /data-command="newGame"/);
  assert.match(indexHTML, /data-command="experiments"/);
  assert.match(indexHTML, /data-command="prompts"/);
  const workflowLabels = [...indexHTML.matchAll(/<button type="button" data-command="[^"]+"[^>]*aria-label="([^"]+)"/g)].map((match) => match[1]);
  assert.equal(workflowLabels.length, 18);
  assert.equal(new Set(workflowLabels).size, workflowLabels.length);
  assert.ok(workflowLabels.includes("Export review trace"));
  assert.ok(workflowLabels.includes("Open local assets in developer lab"));
  assert.match(indexHTML, />Position Setup</);
  assert.match(indexHTML, />Provider Health</);
  assert.match(indexHTML, /aria-label="Chess board workspace"/);
  assert.match(indexHTML, /aria-label="Decision trace"/);
  assert.match(indexHTML, /aria-label="Strategy memory"/);
  assert.match(indexHTML, /id="candidates" class="candidates" role="list" aria-label="Candidate moves" aria-live="polite"/);
  assert.match(indexHTML, /role="status" aria-live="polite"/);
  assert.match(indexHTML, /role="tablist" aria-label="Decision trace views"/);
  assert.match(indexHTML, /id="summaryTab"[\s\S]*tabindex="0"/);
  assert.match(indexHTML, /id="diffTab"[\s\S]*tabindex="-1"/);
  assert.match(indexHTML, /role="tabpanel" aria-labelledby="summaryTab" aria-live="polite"/);
  assert.match(indexHTML, /board-empty-state/);
  assert.match(indexHTML, /Loading board/);
  assert.match(indexHTML, /data-tab="prompt"/);
  assert.match(indexHTML, /id="promptTab"[\s\S]*data-lab-only="true"/);
  assert.match(indexHTML, /id="rawTab"[\s\S]*data-lab-only="true"/);
  assert.match(indexHTML, /Trace JSONL/);
  assert.match(indexHTML, /Debug trace JSONL/);
  assert.match(indexHTML, /Fine-tune JSONL/);
});

test("primary toolbar and dialogs expose expected controls", () => {
  for (const id of [
    "newGameBtn",
    "recentBtn",
    "engineBtn",
    "analyzeBtn",
    "whyBtn",
    "stopBtn",
    "resignBtn",
    "undoBtn",
    "flipBtn",
    "reviewBtn",
    "studyBtn",
    "experimentsBtn",
    "labBtn",
    "promptEditorBtn",
    "importBtn",
    "exportBtn",
    "settingsBtn",
    "settingsDialog",
    "reviewDialog",
    "studyDialog",
    "experimentsDialog",
    "labDialog",
    "promptDialog",
    "profilesDialog",
    "importDialog",
    "exportDialog",
    "recentDialog",
    "promotionDialog",
    "promotionGrid",
  ]) {
    expectMarkupID(id);
  }
  for (const label of [
    "New game",
    "Recent games",
    "Ask engine",
    "Analyze current position",
    "Stop thinking",
    "Flip board",
    "Settings",
    "Why not this move?",
  ]) {
    assert.match(indexHTML, new RegExp(`aria-label="${label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}"`), `missing aria-label for ${label}`);
  }
  assert.match(indexHTML, /aria-label="Close dialog"/);
  assert.match(indexHTML, /id="profilesText" aria-label="Provider profiles YAML or JSON"/);
  assert.match(indexHTML, /id="studyMemoryText" aria-label="Strategy memory JSON"/);
  assert.match(indexHTML, /id="exportText" aria-label="Export output" aria-describedby="exportOutput"/);
  assert.match(indexHTML, /id="exportOutput" role="status" aria-live="polite"/);
  assert.match(indexHTML, /id="importText" aria-label="FEN or PGN input"/);
  assert.match(indexHTML, /role="toolbar" aria-label="Workspace actions"/);
  assert.match(indexHTML, /class="toolbar-group" data-action-scope="play" role="group" aria-label="Play actions"/);
  assert.match(indexHTML, /class="toolbar-group" data-action-scope="study" role="group" aria-label="Study actions"/);
  assert.match(indexHTML, /class="toolbar-group" data-action-scope="lab" role="group" aria-label="Lab actions"/);
  assert.match(indexHTML, /class="toolbar-group toolbar-group-global" data-action-scope="global"/);
  assert.match(indexHTML, /class="toolbar-group toolbar-group-global"[\s\S]*id="recentBtn"[\s\S]*id="importBtn"/);
  assert.match(indexHTML, />New Game</);
  assert.match(indexHTML, />Engine Move</);
  assert.match(indexHTML, />Developer Lab</);
  assert.match(indexHTML, /class="check-label"><input id="settingAutoReply"/);
  assert.match(indexHTML, /id="settingClockInitial" type="number" min="0"/);
  assert.match(indexHTML, /dialog id="settingsDialog" aria-labelledby="settingsTitle"/);
  assert.match(indexHTML, /dialog id="promotionDialog" aria-labelledby="promotionTitle"/);
  assert.match(indexHTML, /id="recentList" class="recent-list" role="list" aria-label="Recent games" aria-live="polite"/);
  assert.match(indexHTML, /aria-keyshortcuts="N"/);
  assert.match(indexHTML, /aria-keyshortcuts="Space"/);
  assert.match(indexHTML, /id="moveBtn"[\s\S]*aria-keyshortcuts="Enter"/);
  assert.match(indexHTML, /id="runImportBtn"[\s\S]*aria-keyshortcuts="Control\+Enter Meta\+Enter"/);
  assert.match(indexHTML, /title="Settings \(\,\)"/);
});

test("settings surface covers MVP and profile controls", () => {
  for (const id of [
    "settingMode",
    "settingPersonality",
    "settingCustomPersonality",
    "settingVariant",
    "settingVariantSeed",
    "settingTheme",
    "settingTimeControl",
    "settingMaxCandidates",
    "settingProfile",
    "settingProvider",
    "settingEndpoint",
    "settingModel",
    "settingTemperature",
    "settingMaxTokens",
    "settingTimeout",
    "settingRetries",
    "settingKey",
    "settingKeyRef",
    "settingVerifier",
    "settingTablebase",
    "settingTablebasePath",
    "settingTablebaseTimeout",
    "settingTraceEnabled",
    "settingRaw",
    "settingRawResponses",
    "healthBtn",
    "benchBtn",
    "modeBenchBtn",
    "profilesBtn",
    "keychainBtn",
    "accessibilityAuditBtn",
    "backupDir",
    "restoreArchive",
    "restoreTarget",
    "tournamentGames",
    "openingBookPath",
    "policyModelPath",
    "customBoardDefinition",
    "customBoardBtn",
    "trainPolicyPriorBtn",
    "enablePolicyPriorBtn",
    "importBookBtn",
    "modeCompareBtn",
    "promptCompareBtn",
    "personalityBuilderBtn",
    "runPromptPlaygroundBtn",
  ]) {
    expectMarkupID(id);
  }
  assert.match(indexHTML, /high_contrast/);
  for (const section of ["Game", "Provider", "Verifier", "Logging"]) {
    assert.match(indexHTML, new RegExp(`<h3 class="settings-section">${section}</h3>`), `missing settings section ${section}`);
  }
});

test("bundle wires core actions and renders trace metadata", () => {
  for (const token of [
    "RequestEngineMove",
    "AnalyzeCurrentPosition",
    "analyzeCurrentPosition",
    "WhyNotMove",
    "whyNotMove",
    "MakeUserMove",
    "NewGame",
    "StopEngine",
    "Resign",
    "Undo",
    "ExportPGN",
    "ExportTrace",
    "ExportDebugTrace",
    "confirmDebugTraceExport",
    "Debug trace export may include raw prompts",
    "ImportPGN",
    "RunRandomBenchmark",
    "RunModeBenchmark",
    "RunPositionSuite",
    "RunProviderComparison",
    "RunTournament",
    "ProviderDashboard",
    "PostGameReview",
    "StudyDashboard",
    "MultiAgentAnalysis",
    "UpdateStrategyMemory",
    "CreateBackup",
    "RestoreBackup",
    "ExportFineTuneDataset",
    "ComparePureHybridAnalysis",
    "RunPromptPlayground",
    "BuildCustomPersonalityProfile",
    "SaveCustomPersonalityProfile",
    "TrainLocalPolicyPrior",
    "EnablePolicyPriorModel",
    "ImportOpeningBook",
    "AccessibilityAudit",
    "PromptTemplatePack",
    "ValidatePromptTemplatePack",
    "SavePromptTemplatePack",
    "ExportProviderProfiles",
    "ImportProviderProfiles",
    "SaveProviderAPIKeyToKeychain",
    "renderBenchmarkSummary",
    "renderPositionSuiteSummary",
    "renderProviderComparison",
    "renderProviderDashboard",
    "renderReview",
    "renderStudyDashboard",
    "renderTournament",
    "renderFineTuneWorkflow",
    "newChess960Game",
    "startCustomBoardFromLab",
    "custom-board-definition.v1",
    "bishop+knight",
    "compareAnalysisModes",
    "comparePromptPlayground",
    "runPromptPlaygroundFromEditor",
    "renderPromptPlayground",
    "buildPersonalityFromLab",
    "trainPolicyPriorFromLab",
    "enablePolicyPriorFromLab",
    "importOpeningBookFromLab",
    "runAccessibilityAudit",
    "promptPackFromInputs",
    "button.setAttribute(\"aria-label\", label)",
    "parseJSONField(\"#promptSchema\", \"Prompt output schema\")",
    "promptValidationErrorMessage",
    "focusPromptValidationError",
    "message.includes(\"system\")",
    "message.includes(\"user\") || message.includes(\"template\") || message.includes(\"placeholder\")",
    "renderPromptValidation",
    "validation?.valid === false",
    "Prompt pack validation failed:",
    "availableTraceTabs",
    "setAttribute(\"aria-labelledby\", next?.id || \"summaryTab\")",
    "activateTraceTab(availableTraceTabs()[0], true)",
    "activateTraceTab(lastItem(availableTraceTabs()), true)",
    "subscribeDecisionStageEvents",
    "stageSummary",
    "statusSummary",
    "statusDetail",
    "compactFen",
    "decisionSummaryText",
    "candidatePurpose",
    "formatStrategyAlert",
    "humanizeToken",
    "knownHumanizedTokens",
    "openai: \"OpenAI\"",
    "humanizeToken(dashboard.active_provider",
    "humanizeToken(profile?.provider",
    "humanizeToken(side?.provider",
    "humanizeToken(review.provider",
    "Explanation unavailable.",
    "Position summary unavailable.",
    "No candidate rationale recorded.",
    "promptInspectorText",
    "Prompt ID",
    "Prompt schema",
    "Decision schema",
    "decision.stage",
    "populateProviderProfiles",
    "raw_prompt",
    "raw_response",
    "parsed_decision",
    "applySelectedProviderProfile",
    "populateCustomPersonalities",
    "providerRequiresAck",
    "boardDimensions",
    "splitUCIMoveSquares",
    "pieceGlyph",
    "renderPromotionChoices",
    "promotionGlyph",
    "handleBoardKeyboard",
    "shouldIgnoreGlobalShortcut",
    "triggerShortcut",
    "isElementVisible",
    "control.disabled || !isElementVisible(control)",
    "triggerShortcut(\"#newGameBtn\")",
    "triggerShortcut(\"#engineBtn\")",
    "triggerShortcut(\"#settingsBtn\")",
    "arrowGeometry",
    "renderBoardOverlay",
    "renderUnavailableState",
    "state = null;",
    "function asArray(value)",
    "function lastItem(value)",
    "function asObject(value)",
    "function asText(value",
    "function textAreaValue(value)",
    "function normalizePromptPack(value)",
    "function gameStateFromResult(value)",
    "function applyGameStateResult(value)",
    "normalizeRenderableState",
    "snapshot.move_history = asArray(snapshot.move_history)",
    "snapshot.legal_moves = asArray(snapshot.legal_moves)",
    "snapshot.board = snapshot.board && typeof snapshot.board === \"object\" ? snapshot.board : {}",
    "normalized.llm.profiles = asArray(normalized.llm.profiles)",
    "const candidates = asArray(dec?.candidate_moves)",
    "const results = asArray(summary.results)",
    "const recommendations = asArray(review.recommendations)",
    "const items = asArray(records)",
    "renderBoardEmpty",
    "renderMoveListEmpty",
    "renderStrategyRows",
    "const dec = state?.last_decision",
    "activateTraceTab",
    "moveTraceTabFocus",
    "tabIndex = selectedTab ? 0 : -1",
    "focusBoardSquare",
    "focusBoardSquare(sq)",
    "focusBoardSquare(to)",
    "focusBoardSquare(from)",
    "busyControls",
    "busyDisabledState",
    "primaryActionRequirements",
    "\"#newGameBtn\": \"service\"",
    "\"#settingsBtn\": \"service\"",
    "\"#engineBtn\": \"ongoing\"",
    "\"#stopBtn\": \"thinking\"",
    "\"#undoBtn\": \"moves\"",
    "setPrimaryActionAvailability(snapshot)",
    "const hasService = !!api()",
    "requirement === \"service\" && !hasService",
    "requirement === \"thinking\" && (!hasService || activeThinkingOperationCount === 0)",
    "moveInput.disabled = !hasService || !ongoing",
    "busyDisabledState.set(control, control.disabled)",
    "const wasDisabled = busyDisabledState.get(control) || false",
    "control.disabled = wasDisabled",
    "setControlBusy",
    "withBusyControl",
    "withThinkingControl",
    "bindBusyButton",
    "workspaceViews",
    "activeOperationCount",
    "activeThinkingOperationCount",
    "appActivitySequence",
    "activityEvents",
    "maxActivityEvents",
    "operationLabels",
    "setAppActivity",
    "recordActivity",
    "renderActivityLog",
    "clearButton.disabled = activityEvents.length === 0",
    "formatActivityTime",
    "const formattedTime = formatActivityTime(event.at)",
    "row.setAttribute(\"aria-label\", `${formattedTime} ${event.label}: ${messageText}`)",
    "openActivityHistory",
    "clearActivityHistory",
    "No recent activity.",
    "Activity history cleared.",
    "reopenSettingsAfterProfiles",
    "settingsDialog.close(\"profiles\")",
    "restoreSettingsAfterProfiles",
    "if (settingsDialog && !settingsDialog.open) settingsDialog.showModal();",
    "settingsForm.scrollTop = settingsScrollBeforeProfiles",
    "dialog?.id === \"profilesDialog\" && reopenSettingsAfterProfiles",
    "beginAppOperation",
    "finishAppOperation",
    "activityLabelFor",
    "showSuccess",
    "activateTraceTab(document.querySelector(\"#summaryTab\"))",
    "showError(\"Enter a move to compare.\")",
    "markFieldInvalid(moveInput)",
    "clearFieldInvalid(moveInput)",
    "moveInput.focus()",
    "document.querySelector(\"#tabContent\").classList.remove(\"empty-copy\")",
    "showError(\"Enter an API key before saving it to the keychain.\", \"#settingsOutput\")",
    "markFieldInvalid(keyField)",
    "keyField.focus()",
    "keyField.select?.()",
    "showError(\"Acknowledge provider endpoint data sharing before saving.\", \"#settingsOutput\")",
    "markFieldInvalid(cloudAck)",
    "cloudAck.focus()",
    "Paste provider profiles before importing.",
    "markFieldInvalid(profilesText)",
    "profilesText.focus()",
    "profilesText.select?.()",
    "previousImportDisabled",
    "Move comparison ready.",
    "Game resigned.",
    "Recent games loaded.",
    "Provider health check complete.",
    "Random benchmark complete.",
    "Mode benchmark complete.",
    "setAppActivity(\"Needs attention\"",
    "setAppActivity(\"Thinking\"",
    "document.body.dataset.appBusy = \"true\"",
    "delete document.body.dataset.appBusy",
    "setWorkspaceView",
    "bindWorkspaceNavigation",
    "moveWorkspaceViewFocus",
    "document.body.dataset.workspaceView",
    "data-workspace-target",
    "next !== \"lab\" && [\"prompt\", \"raw\"].includes(activeTab)",
    "isTraceTabAvailable",
    "button.dataset.labOnly !== \"true\"",
    "filter(isTraceTabAvailable)",
    "workflowCommandTargets",
    "bindWorkflowCommands",
    "runWorkflowCommand",
    "renderWorkflowPanel",
    "workflowTitleText",
    "workflowDetailText",
    "setWorkflowActionAvailability",
    "workflowCommandBusy",
    "workflowCommandUnavailable",
    "button.disabled = busy || workflowCommandUnavailable(button.dataset.command)",
    "button.dataset.requires",
    "button.toggleAttribute(\"aria-busy\", busy)",
    "target.click()",
    "bindWorkflowCommands();",
    "document.querySelector(\"#activityHistoryBtn\").addEventListener(\"click\", openActivityHistory)",
    "document.querySelector(\"#clearActivityBtn\").addEventListener(\"click\"",
    "No decision recorded.",
    "No candidates recorded.",
    "renderCandidatesEmpty",
    "div.setAttribute(\"role\", \"listitem\")",
    "No moves recorded.",
    "Engine move applied.",
    "Position imported.",
    "Settings saved.",
    "Export canceled.",
    "showExportCanceled",
    "finishExport",
    "setExportStatus",
    "showExportError",
    "if (!await refreshExport()) showExportCanceled()",
    "bindDialogCloseButtons",
    "dialogReturnFocusTargets",
    "closeDialogAndRestoreFocus",
    "closeDialogAndRestoreFocus(dialog, \"cancel\")",
    "closeDialogAndRestoreFocus(document.querySelector(\"#recentDialog\"))",
    "closeDialogAndRestoreFocus(document.querySelector(\"#importDialog\"))",
    "closeDialogAndRestoreFocus(document.querySelector(\"#promotionDialog\"))",
    "restoreDialogFocus",
    "focusVisibleElement",
    "focusDialogInitialControl",
    "focusDialogInitialControl(\"#settingMode\")",
    "focusDialogInitialControl(\"#importText\")",
    "focusDialogInitialControl(\"#exportType\")",
    "focusDialogInitialControl(\"#refreshExportBtn\")",
    "focusDialogInitialControl(loaded ? \"#promptSaveDir\" : \"#reloadPromptBtn\")",
    "focusDialogInitialControl(\"#profilesText\")",
    "focusDialogInitialControl(\"#exportProfilesBtn\")",
    "focusDialogInitialControl(\"#backupDir\")",
    "focusDialogInitialControl(\"#saveSettingsBtn\")",
    "focusDialogInitialControl(\"#clearActivityBtn:not(:disabled)\", \"#activityDialog button[value='cancel']\")",
    "focusDialogInitialControl(\"#refreshReviewBtn\")",
    "focusDialogInitialControl(loaded ? \"#studyMemoryText\" : \"#refreshStudyBtn\")",
    "focusDialogInitialControl(\"#providerDashboardBtn\")",
    "focusDialogInitialControl(\"#recentList button:not(:disabled)\", \"#recentDialog button[value='cancel']\")",
    "focusDialogInitialControl(\"#promotionGrid button\")",
    "dialog:not([open])",
    "const hadFocus = document.activeElement === control",
    "hadFocus && document.activeElement === document.body",
    "[0, 80, 250, 750, 1250].forEach",
    "element.focus({ preventScroll: true })",
    "dialog button[value='cancel']",
    "dialog.close(returnValue)",
    "button.disabled = !gameID",
    "Cannot load ${title.textContent}; missing game id",
    "empty.className = \"recent-empty\"",
    "empty.setAttribute(\"role\", \"listitem\")",
    "controlFrom",
    "busyKey",
    "resetBoardEntry",
    "defaultSettings",
    "normalizeSettingsShape",
    "settings = normalizeSettingsShape(await call(\"GetSettings\"))",
    "settings = normalizeSettingsShape(settings)",
    "document.querySelector(\"#settingsOutput\").textContent = \"\"",
    "clearFieldInvalid(document.querySelector(\"#settingKey\"))",
    "const settingsSaveErrorFields",
    "llm\\.endpoint",
    "llm\\.model",
    "llm\\.temperature",
    "llm\\.timeout_ms",
    "verifier\\.movetime_ms",
    "clearSettingsSaveErrorFields()",
    "focusSettingsSaveError(err)",
    "const promptSaveDir = document.querySelector(\"#promptSaveDir\")",
    "renderBoardEmpty(\"Board unavailable\"",
    "renderCandidatesEmpty(\"Candidate moves are unavailable until a game is loaded.\")",
    "return withThinkingControl(\"#engineBtn\"",
    "return withThinkingControl(\"#analyzeBtn\"",
    "return withThinkingControl(\"#whyBtn\"",
    "applyGameStateResult(await call(\"RequestEngineMove\"))",
    "applyGameStateResult(await call(\"MakeUserMove\", normalizedMove))",
    "markFieldInvalid(moveInput)",
    "clearFieldInvalid(moveInput)",
    "applyGameStateResult(type === \"fen\" ? await call(\"ImportFEN\", text) : await call(\"ImportPGN\", text))",
    "Analysis complete. No game state is loaded.",
    "promptPack = normalizePromptPack(await call(\"PromptTemplatePack\"))",
    "return true;",
    "return false;",
    "textAreaValue(await call(\"ExportProviderProfiles\"))",
    "clearFieldInvalid(profilesText)",
    "clearFieldInvalid(backupDir)",
    "clearFieldInvalid(restoreArchive)",
    "clearFieldInvalid(restoreTarget)",
    "clearFieldInvalid(policyModelPath)",
    "clearFieldInvalid(openingBookPath)",
    "clearFieldInvalid(customBoardDefinition)",
    "markFieldInvalid(customBoardDefinition)",
    "customBoardDefinition?.focus()",
    "finishExport(workflow?.dataset_jsonl, \"Fine-tune export ready.\")",
    "clearFieldInvalid(memoryField)",
    "markFieldInvalid(memoryField)",
    "memoryField?.focus()",
    "parseJSONField",
    "markFieldInvalid",
    "clearFieldInvalid",
    "statusDescriberForField",
    "field.setAttribute(\"aria-invalid\", \"true\")",
    "addDescribedBy(field, describedBy)",
    "field?.closest?.(\"section\")?.querySelector?.(\"[role='status'][id]\")?.id",
    "field.addEventListener(\"input\", clear, { once: true })",
    "field.focus()",
    "field.select?.()",
    "must be valid JSON",
    "requireField",
    "Choose a backup archive before restoring.",
    "Choose a restore target before restoring.",
    "markFieldInvalid(backupDir)",
    "backupDir?.focus()",
    "markFieldInvalid(restoreArchive)",
    "restoreArchive?.focus()",
    "markFieldInvalid(restoreTarget)",
    "restoreTarget?.focus()",
    "Enter a policy model path before enabling the prior.",
    "markFieldInvalid(policyModelPath)",
    "policyModelPath?.focus()",
    "Enter an opening book path before importing.",
    "markFieldInvalid(openingBookPath)",
    "openingBookPath?.focus()",
    "Enter a save directory before saving the prompt pack.",
    "markFieldInvalid(promptSaveDir)",
    "promptSaveDir?.focus()",
    "Paste provider profiles before importing.",
    "aria-busy",
    "Enter a UCI move before playing.",
    "focusMoveInput(select = false)",
    "focusMoveInput()",
    "focusMoveInput(true)",
    "restoreBusyControlFocus",
    "#newGameBtn:not(:disabled)",
    "document.querySelector(\"#thinkingStage\").textContent = \"Move failed\"",
    "Paste a FEN or PGN before importing.",
    "document.querySelector(\"#moveInput\").addEventListener(\"keydown\"",
    "document.querySelector(\"#importText\").addEventListener(\"keydown\"",
    "markFieldInvalid(importText)",
    "clearFieldInvalid(importText)",
    "importText.focus()",
    "importText.select?.()",
    "moveInput.value = \"\"",
    "renderRecentGamesEmpty",
    "Loading recent games...",
    "Recent games could not be loaded.",
    "focusDialogInitialControl(\"#recentDialog button[value='cancel']\")",
    "Load ${title.textContent} from ${formatSavedAt(savedAt)}, ${outcomeStatus}",
    "row.setAttribute(\"role\", \"listitem\")",
    "Preparing export...",
    "Export canceled.",
    "settings.gui.clock_initial_ms = timeControl.initial_ms",
    "Number.isFinite(clockInitialMS)",
    "Choose Custom to edit the clock values.",
    "top-candidate",
    "ply ${snapshot.ply || 0}",
    "tablebase_enabled",
    "tablebase_timeout_ms",
    "temperature",
    "max_tokens",
    "retries",
    "personality_score",
    "search_score",
    "verifier_score",
    "strategy_metrics",
    "alert_level",
    "api_key_ref",
    "variant",
    "chess960",
    "applyTheme",
    "candidate_diversity",
    "multi_agent",
  ]) {
    expectScriptToken(token);
  }
  assert.match(appJS, /--board-files/);
  assert.match(appJS, /--board-ranks/);
  assert.match(appJS, /aria-rowcount/);
  assert.match(appJS, /aria-colindex/);
  assert.match(appJS, /row\.className = "board-row"/);
  assert.match(appJS, /row\.setAttribute\("role", "row"\)/);
  assert.match(appJS, /row\.setAttribute\("aria-rowindex", "1"\)/);
  assert.match(appJS, /empty\.setAttribute\("role", "gridcell"\)/);
  assert.match(appJS, /empty\.setAttribute\("aria-colspan", "8"\)/);
  assert.match(appJS, /board\.setAttribute\("aria-label", message\)/);
  assert.match(appJS, /board\.setAttribute\("aria-label", "Chess board"\)/);
  assert.match(appJS, /"openai_compatible", "anthropic", "gemini", "ollama"/);
  assert.match(appJS, /initialInput\.value = Math\.max\(0, Math\.round\(\(tc\.initial_ms \|\| 0\) \/ 60000\)\);/);
  assert.match(appJS, /bindBusyButton\("#runImportBtn"/);
  assert.match(appJS, /showExportError\(err\)/);
  assert.doesNotMatch(appJS, /\.at\(/);
  assert.match(appJS, /async function whyNotMove\(\)[\s\S]*call\("WhyNotMove", move\)[\s\S]*catch \(err\) \{\n      markFieldInvalid\(moveInput\);\n      showError\(err\);\n      focusMoveInput\(true\);/);
  assert.match(appJS, /target\.closest\("dialog\[open\]"\)/);
  assert.match(appJS, /input, textarea, select, button, a, \[role='button'\]/);
  assert.match(stylesCSS, /\.candidate-arrow-head/);
  assert.doesNotMatch(stylesCSS, /marker-end/);
});

test("dialog and control styles stay usable on narrow screens", () => {
  assert.match(stylesCSS, /color-scheme: light;/);
  assert.match(stylesCSS, /input,\nselect,\ntextarea \{[\s\S]*background: #ffffff;[\s\S]*color: #1c2026;/);
  assert.match(stylesCSS, /button:disabled,\ninput:disabled,\nselect:disabled,\ntextarea:disabled/);
  assert.match(stylesCSS, /button\[aria-busy="true"\]::after/);
  assert.match(stylesCSS, /@media \(prefers-reduced-motion: reduce\)/);
  assert.match(stylesCSS, /\.toolbar-group/);
  assert.match(stylesCSS, /#app \{[\s\S]*grid-template-rows: auto minmax\(0, 1fr\);/);
  assert.match(stylesCSS, /\.topbar \{[\s\S]*"activity activity";/);
  assert.match(stylesCSS, /\.activity-row \{[\s\S]*grid-area: activity;/);
  assert.match(stylesCSS, /\.app-activity\[data-tone="busy"\]/);
  assert.match(stylesCSS, /\.app-activity\[data-tone="success"\]/);
  assert.match(stylesCSS, /\.app-activity\[data-tone="error"\]/);
  assert.match(stylesCSS, /body\[data-app-busy="true"\] \.app-activity\[data-tone="busy"\] #appActivityLabel::after/);
  assert.match(stylesCSS, /\.activity-history-button/);
  assert.match(stylesCSS, /#exportOutput \{[\s\S]*min-height: 52px;/);
  assert.match(stylesCSS, /\.recent-empty \{[\s\S]*padding: 18px 12px;/);
  assert.match(stylesCSS, /\.activity-log \{[\s\S]*max-height: min\(58vh, 520px\);/);
  assert.match(stylesCSS, /\.activity-entry \{[\s\S]*grid-template-columns: minmax\(86px, auto\) minmax\(0, 1fr\);/);
  assert.match(stylesCSS, /\.activity-entry\[data-tone="error"\] strong/);
  assert.match(stylesCSS, /\.workflow-panel \{[\s\S]*grid-column: 1 \/ -1;/);
  assert.match(stylesCSS, /\.workflow-cards \{[\s\S]*grid-template-columns: repeat\(3, minmax\(0, 1fr\)\);/);
  assert.match(stylesCSS, /\.workflow-card \{[\s\S]*grid-template-rows: minmax\(0, 1fr\) auto;/);
  assert.match(stylesCSS, /\.workflow-card-actions/);
  assert.match(stylesCSS, /body\[data-workspace-view="study"\] \[data-workspace-card="play"\]/);
  assert.match(stylesCSS, /\.workspace-nav/);
  assert.match(stylesCSS, /\.workspace-nav button\[aria-selected="true"\]/);
  assert.match(stylesCSS, /body\[data-workspace-view="play"\] \[data-action-scope="study"\]/);
  assert.match(stylesCSS, /body:not\(\[data-workspace-view="lab"\]\) \[data-lab-only="true"\]/);
  assert.match(stylesCSS, /\.toolbar-group-global/);
  assert.match(stylesCSS, /\.board-empty-state/);
  assert.match(stylesCSS, /\.board-row,\n\.board-empty-row \{[\s\S]*display: contents;/);
  assert.match(stylesCSS, /\.empty-copy/);
  assert.match(stylesCSS, /\.trace-panel \{[\s\S]*grid-template-rows:/);
  assert.match(stylesCSS, /\.top-candidate/);
  assert.match(stylesCSS, /\.settings \.check-label/);
  assert.match(stylesCSS, /\.import-controls select,\n\.export-controls select \{[\s\S]*min-height: 34px;/);
  assert.match(stylesCSS, /\.lab-grid label,\n\.lab-tools > label \{[\s\S]*display: grid;[\s\S]*gap: 4px;/);
  assert.match(stylesCSS, /\.settings-section/);
  assert.match(stylesCSS, /button:focus-visible/);
  assert.match(stylesCSS, /-webkit-line-clamp: 2/);
  assert.match(stylesCSS, /\.tabs \{[\s\S]*flex-wrap: wrap;/);
  assert.match(stylesCSS, /dialog > form/);
  assert.match(stylesCSS, /position: sticky/);
  assert.match(stylesCSS, /\.settings \{[\s\S]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/);
  assert.match(stylesCSS, /\.settings menu \{[\s\S]*flex-wrap: wrap;/);
  assert.match(stylesCSS, /calc\(\(74vh - 154px\) \* var\(--board-files, 8\) \/ var\(--board-ranks, 8\)\)/);
  assert.match(stylesCSS, /body \{[\s\S]*min-width: 0;/);
  assert.match(stylesCSS, /@media \(min-width: 1241px\)/);
  assert.match(stylesCSS, /@media \(min-width: 761px\) and \(max-height: 860px\)/);
  assert.match(stylesCSS, /@media \(min-width: 761px\) and \(max-height: 860px\) \{[\s\S]*\.workflow-cards \{[\s\S]*display: none;/);
  assert.match(stylesCSS, /@media \(max-width: 1240px\)/);
  assert.match(stylesCSS, /@media \(max-width: 760px\)/);
  assert.match(stylesCSS, /@media \(max-width: 520px\)/);
  assert.match(stylesCSS, /@media \(max-width: 760px\) \{[\s\S]*\.topbar \{[\s\S]*"brand nav"[\s\S]*"toolbar toolbar"[\s\S]*"activity activity";/);
  assert.match(stylesCSS, /@media \(max-width: 760px\) \{[\s\S]*\.brand span \{[\s\S]*display: none;/);
  assert.match(stylesCSS, /@media \(max-width: 760px\) \{[\s\S]*\.workflow-cards \{[\s\S]*display: none;/);
  assert.match(stylesCSS, /@media \(max-width: 520px\) \{[\s\S]*\.workflow-card-actions \{[\s\S]*grid-template-columns: 1fr 1fr;/);
  assert.match(stylesCSS, /@media \(max-width: 520px\) \{[\s\S]*\.toolbar \{[\s\S]*flex-wrap: wrap;[\s\S]*overflow-x: visible;/);
  assert.match(stylesCSS, /@media \(max-width: 520px\) \{[\s\S]*\.toolbar-group \{[\s\S]*display: inline-flex;[\s\S]*flex: 1 1 100%;[\s\S]*border-bottom: 1px solid #d9dee7;/);
  assert.match(stylesCSS, /@media \(max-width: 520px\) \{[\s\S]*\.toolbar-group button \{[\s\S]*flex: 1 1 calc\(50% - 6px\);[\s\S]*white-space: normal;/);
  assert.match(stylesCSS, /@media \(max-width: 520px\) \{[\s\S]*\.activity-row \{[\s\S]*grid-template-columns: minmax\(0, 1fr\) auto;/);
  assert.match(stylesCSS, /@media \(max-width: 520px\) \{[\s\S]*\.app-activity \{[\s\S]*grid-template-columns: auto minmax\(0, 1fr\);/);
  assert.match(stylesCSS, /@media \(max-width: 760px\) \{[\s\S]*\.board-area \{[\s\S]*grid-template-rows: auto auto auto minmax\(96px, auto\);[\s\S]*min-height: 0;/);
  assert.match(stylesCSS, /\.board \{[\s\S]*width: min\(100%, calc\(100vw - 44px\)\);[\s\S]*max-height: none;/);
  assert.match(stylesCSS, /@media \(max-width: 760px\) \{[\s\S]*\.square \{[\s\S]*min\(calc\(37vh \/ var\(--board-ranks, 8\)\), calc\(52vh \/ var\(--board-files, 8\)\), 9vw\)/);
  assert.match(stylesCSS, /\.move-entry input \{[\s\S]*font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;/);
  assert.match(stylesCSS, /\.move-list \{[\s\S]*max-height: 160px;/);
  assert.match(stylesCSS, /pre,\ntextarea \{[\s\S]*font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;/);
  assert.match(stylesCSS, /pre,\n  textarea \{[\s\S]*min-height: 160px;/);
  assert.match(stylesCSS, /\.settings menu button,\n  \.dialog-actions button \{[\s\S]*flex: 1 1 128px;/);
  assert.match(stylesCSS, /\.candidate \{[\s\S]*grid-template-columns: minmax\(0, 1fr\) max-content;/);
  assert.match(stylesCSS, /\.candidate strong \{[\s\S]*grid-column: 1;/);
  assert.match(stylesCSS, /\.candidate span \{[\s\S]*grid-column: 1 \/ -1;/);
  assert.match(stylesCSS, /\.candidate-score \{[\s\S]*grid-column: 2;[\s\S]*grid-row: 1;/);
});
