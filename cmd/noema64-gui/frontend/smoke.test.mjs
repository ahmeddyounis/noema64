import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const dist = new URL("./dist/", import.meta.url);
const indexHTML = await readFile(new URL("index.html", dist), "utf8");
const appJS = await readFile(new URL("app.js", dist), "utf8");

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
    "experimentsBtn",
    "promptEditorBtn",
    "importBtn",
    "exportBtn",
    "settingsBtn",
    "settingsDialog",
    "reviewDialog",
    "experimentsDialog",
    "promptDialog",
    "profilesDialog",
    "importDialog",
    "exportDialog",
    "recentDialog",
    "promotionDialog",
  ]) {
    expectMarkupID(id);
  }
});

test("settings surface covers MVP and profile controls", () => {
  for (const id of [
    "settingMode",
    "settingPersonality",
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
  ]) {
    expectMarkupID(id);
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
    "ProviderDashboard",
    "PostGameReview",
    "PromptTemplatePack",
    "ValidatePromptTemplatePack",
    "SavePromptTemplatePack",
    "ExportProviderProfiles",
    "ImportProviderProfiles",
    "renderBenchmarkSummary",
    "renderPositionSuiteSummary",
    "renderProviderComparison",
    "renderProviderDashboard",
    "renderReview",
    "promptPackFromInputs",
    "subscribeDecisionStageEvents",
    "stageSummary",
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
  ]) {
    expectScriptToken(token);
  }
});
