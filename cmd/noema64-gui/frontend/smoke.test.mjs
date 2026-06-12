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
});

test("primary toolbar and dialogs expose expected controls", () => {
  for (const id of [
    "newGameBtn",
    "recentBtn",
    "engineBtn",
    "stopBtn",
    "resignBtn",
    "undoBtn",
    "flipBtn",
    "importBtn",
    "exportBtn",
    "settingsBtn",
    "settingsDialog",
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
    "settingKey",
    "settingVerifier",
    "settingTraceEnabled",
    "settingRaw",
    "settingRawResponses",
    "healthBtn",
    "benchBtn",
    "modeBenchBtn",
  ]) {
    expectMarkupID(id);
  }
});

test("bundle wires core actions and renders trace metadata", () => {
  for (const token of [
    "RequestEngineMove",
    "MakeUserMove",
    "NewGame",
    "StopEngine",
    "Resign",
    "Undo",
    "ExportPGN",
    "ImportPGN",
    "RunRandomBenchmark",
    "RunModeBenchmark",
    "renderBenchmarkSummary",
    "subscribeDecisionStageEvents",
    "stageSummary",
    "decision.stage",
    "populateProviderProfiles",
    "applySelectedProviderProfile",
    "search_score",
    "verifier_score",
  ]) {
    expectScriptToken(token);
  }
});
