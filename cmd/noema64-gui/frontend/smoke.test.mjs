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
  for (const id of ["board", "statusText", "clockText", "modeText", "moveInput", "moveList", "tabContent", "candidates", "strategyMemory"]) {
    expectMarkupID(id);
  }
  assert.match(indexHTML, /aria-label="Chess board workspace"/);
  assert.match(indexHTML, /aria-label="Decision trace"/);
  assert.match(indexHTML, /aria-label="Strategy memory"/);
  assert.match(indexHTML, /data-tab="prompt"/);
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
  assert.match(indexHTML, /aria-keyshortcuts="N"/);
  assert.match(indexHTML, /aria-keyshortcuts="Space"/);
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
    "subscribeDecisionStageEvents",
    "stageSummary",
    "statusSummary",
    "statusDetail",
    "compactFen",
    "decisionSummaryText",
    "candidatePurpose",
    "formatStrategyAlert",
    "humanizeToken",
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
    "arrowGeometry",
    "renderBoardOverlay",
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
  assert.match(appJS, /target\.closest\("dialog\[open\]"\)/);
  assert.match(appJS, /input, textarea, select, button, a, \[role='button'\]/);
  assert.match(stylesCSS, /\.candidate-arrow-head/);
  assert.doesNotMatch(stylesCSS, /marker-end/);
});

test("dialog and control styles stay usable on narrow screens", () => {
  assert.match(stylesCSS, /button:focus-visible/);
  assert.match(stylesCSS, /-webkit-line-clamp: 2/);
  assert.match(stylesCSS, /\.tabs \{[\s\S]*flex-wrap: wrap;/);
  assert.match(stylesCSS, /dialog > form/);
  assert.match(stylesCSS, /position: sticky/);
  assert.match(stylesCSS, /\.settings menu \{[\s\S]*flex-wrap: wrap;/);
  assert.match(stylesCSS, /@media \(max-width: 520px\)/);
  assert.match(stylesCSS, /\.settings menu button,\n  \.dialog-actions button \{[\s\S]*flex: 1 1 128px;/);
  assert.match(stylesCSS, /\.candidate \{[\s\S]*grid-template-columns: minmax\(0, 1fr\) max-content;/);
  assert.match(stylesCSS, /\.candidate strong \{[\s\S]*grid-column: 1;/);
  assert.match(stylesCSS, /\.candidate span \{[\s\S]*grid-column: 1 \/ -1;/);
  assert.match(stylesCSS, /\.candidate-score \{[\s\S]*grid-column: 2;[\s\S]*grid-row: 1;/);
});
