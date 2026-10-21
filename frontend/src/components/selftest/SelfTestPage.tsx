import { useEffect, useMemo, useState } from "react";
import { Events } from "@wailsio/runtime";
import { App } from "@bindings";
import {
  FlaskConical,
  Play,
  Square,
  ClipboardCopy,
  CheckCircle2,
  CircleDashed,
  PlayCircle,
  XCircle,
  SlidersHorizontal,
  Loader2,
} from "lucide-react";
import type { Color } from "@/lib/types";

// After this many ms of capture without locking, a cell that has a
// baseline is treated as a hard mismatch rather than "still settling".
// At 2Hz capture, 4s is 8 frames — more than enough for the smoother
// (which we run with colorSmoothing=0 in the self-test) to produce a
// stable output. Anything that hasn't locked by then has a real
// extraction discrepancy to investigate.
const SETTLE_DEADLINE_MS = 4000;

// ------- Types mirroring internal/screensync/selftest/selftest.go -------

interface Fixture {
  id: string;
  label: string;
  description: string;
  url: string;
  kind: "photo" | "synthetic";
  expected?: Record<string, Color[]>;
}

interface ModeCase {
  id: string;
  label: string;
  group: "single" | "palette" | "spatial";
}

interface CellResult {
  modeId: string;
  target: Color[];   // pre-handoff — what the extractor + smoother + assigner decided
  active: Color[];   // post-handoff — what would have been sent to the bulb
  expected?: Color[];
  locked: boolean;
  lockedSinceMs: number;
}

// Types mirror internal/screensync/selftest/suite.go.
interface SuiteProgress {
  fixtureIndex: number;
  totalCount: number;
  fixtureId: string;
  fixtureLabel: string;
  phase: "warming" | "capturing" | "settled";
  elapsedMs: number;
}

interface FixtureRunResult {
  fixtureId: string;
  fixtureLabel: string;
  durationMs: number;
  lockedCells: number;
  totalCells: number;
  diagnostic: {
    cells: Array<{
      modeId: string;
      modeLabel: string;
      expected?: Color[];
      active?: Color[];
      locked: boolean;
      verdict: string;
    }>;
  };
}

interface SuiteRun {
  startedAt: string;
  finishedAt: string;
  totalMs: number;
  fixtureCount: number;
  lockedCells: number;
  totalCells: number;
  fixtures: FixtureRunResult[];
}

// How long to wait before offering the "Copy Diagnostics" call-out. Beyond
// this point, if a cell still hasn't locked, something is probably wrong
// with extraction and the user should ship us a diagnostic dump.
const DIAGNOSTIC_THRESHOLD_MS = 10_000;

// ------- Alignment logic — the same thresholds the sidebar uses -------

const ALIGN_HUE_DEG = 15; // looser than bulb alignment because DWM/JPEG add noise
const ALIGN_SAT = 0.15;
const ALIGN_BRIGHT = 0.15;

function isAligned(a: Color | undefined, b: Color | undefined): boolean {
  if (!a || !b) return false;
  // If either side is near-grey the hue is meaningless; only compare S+B.
  const greyish = a.s < 0.08 && b.s < 0.08;
  let hueOk = true;
  if (!greyish) {
    let dh = Math.abs(a.h - b.h);
    if (dh > 180) dh = 360 - dh;
    hueOk = dh <= ALIGN_HUE_DEG;
  }
  return hueOk && Math.abs(a.s - b.s) <= ALIGN_SAT && Math.abs(a.b - b.b) <= ALIGN_BRIGHT;
}

function toCss(c: Color): string {
  return `hsl(${c.h}, ${Math.round(c.s * 100)}%, ${Math.round(c.b * 50)}%)`;
}

// ------- Triple swatch: expected (top) | target (mid) | active (bottom) -------
//
// Top  = Expected — the fixture's known-good ground truth
// Mid  = Target   — what Extract + Smooth + Assign decided this frame
// Bot  = Active   — what the bulb would have just been sent (post-handoff)
//
// During a handoff ramp Target is stable and Active slides toward it, so the
// bottom stripe visibly animates. Once Active ≈ Expected for 2 consecutive
// frames the cell is "locked" and we colour the ring green.

interface SwatchTripleProps {
  expected?: Color;
  target?: Color;
  active?: Color;
}

function SwatchTriple({ expected, target, active }: SwatchTripleProps) {
  const aligned = isAligned(expected, active);
  const expectedBg = expected ? toCss(expected) : "rgba(255,255,255,0.04)";
  const targetBg = target ? toCss(target) : "rgba(255,255,255,0.04)";
  const activeBg = active ? toCss(active) : "rgba(255,255,255,0.04)";
  const hasContent = !!(expected || target || active);

  const fmt = (c: Color | undefined) =>
    c ? `H:${Math.round(c.h)}° S:${Math.round(c.s * 100)}% B:${Math.round(c.b * 100)}%` : "—";
  const tooltip =
    `expected: ${fmt(expected)}\n` +
    `target:   ${fmt(target)}\n` +
    `active:   ${fmt(active)}` +
    (aligned ? "\n✓ active aligned with expected" : hasContent ? "\n… active not yet aligned" : "");

  const ring = aligned
    ? "ring-2 ring-success/80"
    : expected && active
      ? "ring-1 ring-destructive/40"
      : "ring-1 ring-white/10";

  return (
    <div
      className={`relative h-10 w-12 rounded overflow-hidden shrink-0 flex flex-col ${ring}`}
      title={tooltip}
    >
      <div className="h-1/3" style={{ background: expectedBg }} />
      <div className="h-1/3" style={{ background: targetBg }} />
      <div className="h-1/3 transition-colors duration-500" style={{ background: activeBg }} />
    </div>
  );
}

// ------- Multi-colour row: shows N swatch triples side by side -------

function MultiSwatches({ expected, target, active }: {
  expected?: Color[];
  target?: Color[];
  active?: Color[];
}) {
  const n = Math.max(expected?.length ?? 0, target?.length ?? 0, active?.length ?? 0, 1);
  return (
    <div className="flex gap-1">
      {Array.from({ length: n }).map((_, i) => (
        <SwatchTriple
          key={i}
          expected={expected?.[i]}
          target={target?.[i]}
          active={active?.[i]}
        />
      ))}
    </div>
  );
}

// =====================================================================
//                        Main page
// =====================================================================

export function SelfTestPage() {
  const [fixtures, setFixtures] = useState<Fixture[]>([]);
  const [modes, setModes] = useState<ModeCase[]>([]);
  const [activeFixture, setActiveFixture] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<Record<string, CellResult>>({});
  const [lastUpdate, setLastUpdate] = useState<number | null>(null);
  const [elapsedMs, setElapsedMs] = useState<number>(0);
  const [copyState, setCopyState] = useState<"idle" | "copying" | "copied" | "error">("idle");

  // Suite state — independent from the per-fixture live state so a user
  // can leave the Self-Test tab, come back, and still see the last suite
  // report.
  const [suiteRunning, setSuiteRunning] = useState(false);
  const [suiteProgress, setSuiteProgress] = useState<SuiteProgress | null>(null);
  const [suiteResult, setSuiteResult] = useState<SuiteRun | null>(null);
  const [suiteCopyState, setSuiteCopyState] = useState<"idle" | "copying" | "copied" | "error">("idle");

  // Screen-sync calibration (subset suite + coordinate search).
  const [screenSyncScenes, setScreenSyncScenes] = useState<{ id: string; name: string }[]>([]);
  const [calSceneId, setCalSceneId] = useState<string>("");
  const [calRunning, setCalRunning] = useState(false);
  const [calReport, setCalReport] = useState<Record<string, unknown> | null>(null);
  const [calActionState, setCalActionState] = useState<"idle" | "busy" | "ok" | "error">("idle");
  const [calEvalCount, setCalEvalCount] = useState(0);

  // Hydrate fixture + mode catalogue on mount.
  useEffect(() => {
    App.ListSelfTestFixtures().then((fs) => setFixtures((fs ?? []) as Fixture[])).catch(() => {});
    App.ListSelfTestModes().then((ms) => setModes((ms ?? []) as ModeCase[])).catch(() => {});
  }, []);

  // Subscribe to backend events.
  useEffect(() => {
    const offResults = Events.On("selftest:results", (e) => {
      const data = e.data as {
        fixture?: string;
        cells?: CellResult[];
        at?: number;
        elapsedMs?: number;
      };
      if (!data?.cells) return;
      const next: Record<string, CellResult> = {};
      for (const c of data.cells) next[c.modeId] = c;
      setResults(next);
      setLastUpdate(data.at ?? Date.now());
      if (typeof data.elapsedMs === "number") setElapsedMs(data.elapsedMs);
    });
    const offState = Events.On("selftest:state", (e) => {
      const data = e.data as { running?: boolean; fixture?: string; mode?: string };
      if (typeof data?.running === "boolean") {
        setRunning(data.running);
        if (data.mode === "suite") {
          setSuiteRunning(data.running);
        } else if (!data.running) {
          setSuiteRunning(false);
        }
      }
      // Only clear live grid when the backend explicitly reports stopped — if
      // `running` is missing from the payload, !undefined would wipe results
      // while a suite is still capturing.
      if (typeof data?.running === "boolean" && !data.running) {
        setResults({});
        setLastUpdate(null);
        setElapsedMs(0);
        setCopyState("idle");
      }
    });
    const offSuiteProgress = Events.On("selftest:suite-progress", (e) => {
      const p = e.data as SuiteProgress;
      setSuiteProgress(p);
      // Mirror the progress fixture into the single-fixture header too so
      // the image thumbnail and description update while the suite runs.
      setActiveFixture(p.fixtureId);
      // New fixture: drop previous frame's cells so we never show old targets
      // against new expected colours, and reset elapsed so "mismatch" does not
      // latch instantly from the prior fixture's dwell clock.
      if (p.phase === "warming") {
        setResults({});
        setElapsedMs(0);
        setLastUpdate(null);
      }
    });
    const offSuiteComplete = Events.On("selftest:suite-complete", (e) => {
      const run = e.data as SuiteRun;
      setSuiteResult(run);
      setSuiteRunning(false);
      setSuiteProgress(null);
      setSuiteCopyState("idle");
    });
    const offSuiteCancelled = Events.On("selftest:suite-cancelled", () => {
      setSuiteRunning(false);
      setSuiteProgress(null);
    });
    const offCalState = Events.On("selftest:calibrate-state", (e) => {
      const d = e.data as { running?: boolean };
      if (typeof d?.running === "boolean") {
        setCalRunning(d.running);
        // Calibration reuses the fixture popup with EmitSuiteResults=false, so
        // the grid would otherwise keep showing the last suite fixture forever
        // (e.g. "settling…") while synthetics cycle in the popup.
        if (d.running) {
          setResults({});
          setElapsedMs(0);
          setLastUpdate(null);
        }
      }
    });
    const offCalProgress = Events.On("selftest:calibrate-progress", () => {
      setCalEvalCount((n) => n + 1);
    });
    const offCalComplete = Events.On("selftest:calibrate-complete", (e) => {
      setCalRunning(false);
      const raw = e.data as Record<string, unknown> | undefined;
      setCalReport(raw && typeof raw === "object" ? raw : null);
      const ec = raw && typeof raw === "object" ? (raw as { searchEvalCount?: number }).searchEvalCount : undefined;
      if (typeof ec === "number") setCalEvalCount(ec);
    });
    return () => {
      offResults?.();
      offState?.();
      offSuiteProgress?.();
      offSuiteComplete?.();
      offSuiteCancelled?.();
      offCalState?.();
      offCalProgress?.();
      offCalComplete?.();
    };
  }, []);

  const handleCopyDiagnostic = async () => {
    setCopyState("copying");
    try {
      const blob = await App.GetSelfTestDiagnostic();
      await navigator.clipboard.writeText(blob as string);
      setCopyState("copied");
      setTimeout(() => setCopyState("idle"), 3000);
    } catch (err) {
      console.error(err);
      setCopyState("error");
      setTimeout(() => setCopyState("idle"), 3000);
    }
  };

  const handleRunSuite = async () => {
    setSuiteResult(null);
    setCalReport(null);
    setSuiteRunning(true);
    setSuiteProgress(null);
    setSuiteCopyState("idle");
    try {
      await App.StartSelfTestSuite();
    } catch (err) {
      console.error(err);
      setSuiteRunning(false);
    }
  };

  // Load screen_sync scenes when a suite report is shown (for calibration target).
  useEffect(() => {
    if (!suiteResult || suiteRunning) return;
    App.GetScenes()
      .then((ss) => {
        const list = (ss ?? [])
          .filter((s: { trigger?: string }) => s.trigger === "screen_sync")
          .map((s: { id?: string; name?: string }) => ({
            id: String(s.id ?? ""),
            name: String(s.name ?? s.id ?? ""),
          }))
          .filter((s) => s.id);
        setScreenSyncScenes(list);
        setCalSceneId((prev) => {
          if (prev && list.some((x) => x.id === prev)) return prev;
          return list[0]?.id ?? "";
        });
      })
      .catch(() => {});
  }, [suiteResult, suiteRunning]);

  const handleStartCalibrate = async () => {
    if (!calSceneId || calRunning) return;
    setCalReport(null);
    setCalEvalCount(0);
    setCalActionState("busy");
    try {
      await App.StartSelfTestCalibrate(calSceneId);
      setCalActionState("idle");
    } catch (e) {
      console.error(e);
      setCalActionState("error");
      setTimeout(() => setCalActionState("idle"), 3000);
    }
  };

  const handleStopCalibrate = async () => {
    try {
      await App.StopSelfTestCalibrate();
    } catch (e) {
      console.error(e);
    }
  };

  const handleApplyCalibrate = async () => {
    if (!calSceneId || !calReport || (calReport.error as string | undefined)) return;
    setCalActionState("busy");
    try {
      await App.SelfTestCalibrateApply(calSceneId);
      setCalActionState("ok");
      setTimeout(() => setCalActionState("idle"), 2500);
    } catch (e) {
      console.error(e);
      setCalActionState("error");
      setTimeout(() => setCalActionState("idle"), 3000);
    }
  };

  const handleRollbackCalibrate = async () => {
    if (!calSceneId) return;
    setCalActionState("busy");
    try {
      await App.SelfTestCalibrateRollback(calSceneId);
      setCalActionState("ok");
      setTimeout(() => setCalActionState("idle"), 2500);
    } catch (e) {
      console.error(e);
      setCalActionState("error");
      setTimeout(() => setCalActionState("idle"), 3000);
    }
  };

  const handleCopyCalibrateReport = async () => {
    try {
      const json = await App.GetSelfTestCalibrateReportJSON();
      await navigator.clipboard.writeText(json as string);
      setCalActionState("ok");
      setTimeout(() => setCalActionState("idle"), 2000);
    } catch (e) {
      console.error(e);
      setCalActionState("error");
      setTimeout(() => setCalActionState("idle"), 3000);
    }
  };

  const handlePromotePortableDefaults = async () => {
    if (!calReport || (calReport.error as string | undefined)) return;
    setCalActionState("busy");
    try {
      await App.PromoteLastCalibrationToScreenSyncDefaults();
      setCalActionState("ok");
      setTimeout(() => setCalActionState("idle"), 2500);
    } catch (e) {
      console.error(e);
      setCalActionState("error");
      setTimeout(() => setCalActionState("idle"), 3000);
    }
  };

  const handleClearPortableDefaults = async () => {
    setCalActionState("busy");
    try {
      await App.ClearScreenSyncPortableDefaults();
      setCalActionState("ok");
      setTimeout(() => setCalActionState("idle"), 2500);
    } catch (e) {
      console.error(e);
      setCalActionState("error");
      setTimeout(() => setCalActionState("idle"), 3000);
    }
  };

  const handleCopySuite = async () => {
    setSuiteCopyState("copying");
    try {
      const blob = await App.GetSelfTestSuiteDiagnostic();
      await navigator.clipboard.writeText(blob as string);
      setSuiteCopyState("copied");
      setTimeout(() => setSuiteCopyState("idle"), 3000);
    } catch (err) {
      console.error(err);
      setSuiteCopyState("error");
      setTimeout(() => setSuiteCopyState("idle"), 3000);
    }
  };

  // Convergence summary for the header pill.
  const cellList = Object.values(results);
  const lockedCount = cellList.filter((c) => c.locked).length;
  const totalWithExpectation = cellList.filter((c) => (c.expected?.length ?? 0) > 0).length;
  const mismatchCount =
    elapsedMs >= SETTLE_DEADLINE_MS
      ? cellList.filter(
          (c) => (c.expected?.length ?? 0) > 0 && !c.locked,
        ).length
      : 0;
  const showDiagnosticPrompt =
    running &&
    elapsedMs > DIAGNOSTIC_THRESHOLD_MS &&
    totalWithExpectation > 0 &&
    lockedCount < totalWithExpectation;

  const handleStart = async (fixtureId: string) => {
    if (suiteRunning) return; // suite owns the popup — ignore manual picks
    setActiveFixture(fixtureId);
    setResults({});
    try {
      await App.StartSelfTest(fixtureId);
    } catch (err) {
      console.error(err);
    }
  };

  const handleStop = async () => {
    try {
      await App.StopSelfTest();
    } catch (err) {
      console.error(err);
    }
    setActiveFixture(null);
  };

  // Build rendered rows grouped by mode.group.
  const modesByGroup = useMemo(() => {
    const g: Record<string, ModeCase[]> = { single: [], palette: [], spatial: [] };
    for (const m of modes) {
      if (!g[m.group]) g[m.group] = [];
      g[m.group].push(m);
    }
    return g;
  }, [modes]);

  const activeFx = fixtures.find((f) => f.id === activeFixture) ?? null;

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-start justify-between px-6 py-5 border-b border-border">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-muted flex items-center justify-center">
            <FlaskConical className="h-5 w-5 text-muted-foreground" />
          </div>
          <div>
            <h1 className="text-lg font-semibold leading-tight">Self-Test</h1>
            <p className="text-xs text-muted-foreground">
              Renders a test fixture fullscreen, captures via the real DXGI pipeline, and compares
              extracted colours to expected values across every mode the app supports.
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {!running && !suiteRunning && (
            <button
              type="button"
              onClick={handleRunSuite}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm bg-primary/15 text-primary hover:bg-primary/25 transition-colors"
              title="Walk every fixture in sequence, record locked/failed cells, and produce an aggregated report."
            >
              <PlayCircle className="h-3.5 w-3.5" />
              Run Full Suite
            </button>
          )}
          {running && (
            <button
              type="button"
              onClick={handleStop}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm bg-destructive/15 text-destructive hover:bg-destructive/25 transition-colors"
            >
              <Square className="h-3.5 w-3.5" />
              Stop
            </button>
          )}
        </div>
      </div>

      {/* Suite progress bar — visible only during a suite run. */}
      {suiteRunning && suiteProgress && (
        <SuiteProgressBar progress={suiteProgress} />
      )}

      <div className="flex-1 grid grid-cols-[280px_1fr] overflow-hidden">
        {/* Fixture picker */}
        <aside className="border-r border-border overflow-y-auto">
          <FixtureList
            fixtures={fixtures}
            activeId={activeFixture}
            running={running}
            onPick={handleStart}
          />
        </aside>

        {/* Results grid */}
        <section className="overflow-y-auto p-6 space-y-5">
          {/* Master suite report — shown whenever a completed run exists,
             even if the user later kicks off a manual fixture run. */}
          {suiteResult && !suiteRunning && (
            <>
              <SuiteReport
                result={suiteResult}
                modes={modes}
                onCopy={handleCopySuite}
                copyState={suiteCopyState}
                onDismiss={() => setSuiteResult(null)}
              />
              <CalibrationPanel
                scenes={screenSyncScenes}
                selectedSceneId={calSceneId}
                onSelectScene={setCalSceneId}
                running={calRunning}
                evalCountLive={calEvalCount}
                report={calReport}
                actionState={calActionState}
                onStart={handleStartCalibrate}
                onStop={handleStopCalibrate}
                onApply={handleApplyCalibrate}
                onRollback={handleRollbackCalibrate}
                onCopyReport={handleCopyCalibrateReport}
                onPromotePortableDefaults={handlePromotePortableDefaults}
                onClearPortableDefaults={handleClearPortableDefaults}
              />
            </>
          )}

          {!activeFx ? (
            <div className="text-sm text-muted-foreground">
              Pick a fixture from the left to launch it fullscreen and start the live capture loop
              — or click <span className="font-semibold text-foreground">Run Full Suite</span> above
              to walk every fixture in sequence and produce a master report.
            </div>
          ) : (
            <>
              <header className="flex items-start gap-4">
                <img
                  src={activeFx.url}
                  alt={activeFx.label}
                  className="h-20 w-28 object-cover rounded-lg ring-1 ring-white/10"
                />
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h2 className="text-sm font-semibold">{activeFx.label}</h2>
                    <span className="text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                      {activeFx.kind}
                    </span>
                    {running && (
                      <span className="text-[10px] text-success flex items-center gap-1">
                        <span className="h-1 w-1 rounded-full bg-success animate-pulse" />
                        live
                        {lastUpdate && (
                          <span className="text-muted-foreground/60">
                            · {Math.round((Date.now() - lastUpdate) / 100) / 10}s
                          </span>
                        )}
                      </span>
                    )}
                    {totalWithExpectation > 0 && (
                      <span className="text-[10px]">
                        <span className="text-muted-foreground">· </span>
                        <span className="text-success">{lockedCount}</span>
                        {mismatchCount > 0 && (
                          <>
                            <span className="text-muted-foreground"> · </span>
                            <span className="text-destructive">{mismatchCount} mismatch</span>
                          </>
                        )}
                        <span className="text-muted-foreground"> / {totalWithExpectation}</span>
                      </span>
                    )}
                    <span className="text-[10px] text-muted-foreground">
                      · {(elapsedMs / 1000).toFixed(1)}s
                    </span>
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">{activeFx.description}</p>
                </div>
                {showDiagnosticPrompt && (
                  <button
                    type="button"
                    onClick={handleCopyDiagnostic}
                    className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs transition-colors ${
                      copyState === "copied"
                        ? "bg-success/15 text-success"
                        : copyState === "error"
                          ? "bg-destructive/15 text-destructive"
                          : "bg-warning/10 text-warning hover:bg-warning/20"
                    }`}
                    title="Not all cells locked within 10s. Copy a diagnostic blob to share."
                  >
                    <ClipboardCopy className="h-3.5 w-3.5" />
                    {copyState === "copied"
                      ? "Copied to clipboard"
                      : copyState === "error"
                        ? "Copy failed"
                        : copyState === "copying"
                          ? "Copying…"
                          : "Copy Diagnostics"}
                  </button>
                )}
              </header>

              <ResultsGroup
                title="Single colour"
                modes={modesByGroup.single ?? []}
                results={results}
                expected={activeFx.expected}
                elapsedMs={elapsedMs}
              />
              <ResultsGroup
                title="Multi — Scene Palette"
                modes={modesByGroup.palette ?? []}
                results={results}
                expected={activeFx.expected}
                elapsedMs={elapsedMs}
              />
              <ResultsGroup
                title="Multi — Spatial Grid"
                modes={modesByGroup.spatial ?? []}
                results={results}
                expected={activeFx.expected}
                elapsedMs={elapsedMs}
              />
            </>
          )}
        </section>
      </div>
    </div>
  );
}

// ------- Sub-components -------

function FixtureList({
  fixtures,
  activeId,
  running,
  onPick,
}: {
  fixtures: Fixture[];
  activeId: string | null;
  running: boolean;
  onPick: (id: string) => void;
}) {
  const synth = fixtures.filter((f) => f.kind === "synthetic");
  const photos = fixtures.filter((f) => f.kind === "photo");

  return (
    <div className="p-4 space-y-5">
      <FixtureGroup title="Synthetic" fixtures={synth} activeId={activeId} running={running} onPick={onPick} />
      <FixtureGroup title="Photos" fixtures={photos} activeId={activeId} running={running} onPick={onPick} />
    </div>
  );
}

function FixtureGroup({
  title,
  fixtures,
  activeId,
  running,
  onPick,
}: {
  title: string;
  fixtures: Fixture[];
  activeId: string | null;
  running: boolean;
  onPick: (id: string) => void;
}) {
  if (fixtures.length === 0) return null;
  return (
    <div>
      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/80 mb-2">{title}</p>
      <div className="space-y-1">
        {fixtures.map((f) => {
          const active = activeId === f.id;
          return (
            <button
              key={f.id}
              type="button"
              onClick={() => onPick(f.id)}
              className={`w-full flex items-center gap-3 p-2 rounded-lg text-left transition-colors ${
                active ? "bg-primary/15 ring-1 ring-primary/40" : "hover:bg-muted"
              }`}
            >
              <img
                src={f.url}
                alt={f.label}
                className="h-10 w-14 object-cover rounded shrink-0 ring-1 ring-white/10"
              />
              <div className="min-w-0 flex-1">
                <div className="text-xs font-medium truncate">{f.label}</div>
                <div className="text-[10px] text-muted-foreground truncate">{f.id}</div>
              </div>
              {active && running && <Play className="h-3 w-3 text-success shrink-0" />}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function ResultsGroup({
  title,
  modes,
  results,
  expected,
  elapsedMs,
}: {
  title: string;
  modes: ModeCase[];
  results: Record<string, CellResult>;
  expected?: Record<string, Color[]>;
  elapsedMs: number;
}) {
  if (modes.length === 0) return null;
  return (
    <div>
      <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground/80 mb-2">
        {title}
      </h3>
      <div className="rounded-lg bg-muted/40 divide-y divide-border">
        {modes.map((m) => {
          const r = results[m.id];
          const exp = expected?.[m.id];
          const hasExpectation = (exp?.length ?? 0) > 0;
          return (
            <div key={m.id} className="flex items-center gap-4 px-3 py-2">
              <div className="w-48 text-xs font-medium truncate" title={m.label}>
                {m.label}
              </div>
              <div className="flex-1">
                <MultiSwatches expected={exp} target={r?.target} active={r?.active} />
              </div>
              <LockIndicator
                hasExpectation={hasExpectation}
                locked={r?.locked ?? false}
                lockedSinceMs={r?.lockedSinceMs ?? 0}
                elapsedMs={elapsedMs}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}

function LockIndicator({
  hasExpectation,
  locked,
  lockedSinceMs,
  elapsedMs,
}: {
  hasExpectation: boolean;
  locked: boolean;
  lockedSinceMs: number;
  elapsedMs: number;
}) {
  if (!hasExpectation) {
    return (
      <span className="text-[10px] text-muted-foreground/60 w-24 text-right">no baseline</span>
    );
  }
  if (locked) {
    return (
      <span className="text-[11px] text-success flex items-center gap-1 w-24 justify-end">
        <CheckCircle2 className="h-3.5 w-3.5" />
        locked · {(lockedSinceMs / 1000).toFixed(1)}s
      </span>
    );
  }
  // Static fixtures should converge near-instantly. If a cell hasn't
  // locked within SETTLE_DEADLINE_MS, what we're looking at isn't
  // handoff ramp or smoother lag — it's a reproducible mismatch between
  // the extractor output and the baseline. Flag it as such so the
  // signal is unambiguous.
  if (elapsedMs >= SETTLE_DEADLINE_MS) {
    return (
      <span
        className="text-[11px] text-destructive flex items-center gap-1 w-24 justify-end"
        title="Active output differs from the baseline beyond tolerance and is no longer changing. This is a real pipeline discrepancy."
      >
        <XCircle className="h-3.5 w-3.5" />
        mismatch
      </span>
    );
  }
  return (
    <span className="text-[11px] text-muted-foreground/70 flex items-center gap-1 w-24 justify-end">
      <CircleDashed className="h-3.5 w-3.5 animate-pulse" />
      settling…
    </span>
  );
}

// ------- Suite progress + master report -------

function SuiteProgressBar({ progress }: { progress: SuiteProgress }) {
  const pct =
    progress.totalCount > 0
      ? Math.round(((progress.fixtureIndex + (progress.phase === "settled" ? 1 : 0.5)) /
          progress.totalCount) *
          100)
      : 0;
  return (
    <div className="border-b border-border bg-muted/40 px-6 py-3">
      <div className="flex items-center justify-between text-xs">
        <div className="flex items-center gap-2">
          <CircleDashed className="h-3.5 w-3.5 text-primary animate-spin" />
          <span className="text-muted-foreground">
            Suite {progress.fixtureIndex + 1} / {progress.totalCount} ·{" "}
            <span className="text-foreground font-medium">{progress.fixtureLabel}</span> ·{" "}
            <span className="text-muted-foreground/70">{progress.phase}</span>
          </span>
        </div>
        <span className="text-muted-foreground font-mono">
          {(progress.elapsedMs / 1000).toFixed(1)}s · {pct}%
        </span>
      </div>
      <div className="mt-2 h-1 rounded-full bg-background/40 overflow-hidden">
        <div
          className="h-full bg-primary transition-all duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

/** Tunable knobs included in v1 coordinate search (JSON field names on `ScreenSyncConfig`). */
const CALIB_KNOB_KEYS = [
  "saturationBoost",
  "whiteBias",
  "brightnessMultiplier",
  "paletteStability",
  "colorSmoothing",
  "brightnessSmoothing",
  "assignmentHandoffMs",
] as const;

function humanizeKnobKey(k: string): string {
  if (k.startsWith("colorTuning.")) {
    const rest = k.slice("colorTuning.".length);
    return `Color tuning · ${humanizeKnobKey(rest)}`;
  }
  return k
    .replace(/([A-Z])/g, " $1")
    .replace(/^./, (c) => c.toUpperCase())
    .trim();
}

function formatKnobValue(key: string, v: unknown): string {
  if (typeof v !== "number" || Number.isNaN(v)) return String(v ?? "—");
  if (key === "assignmentHandoffMs") return `${Math.round(v)} ms`;
  return Number(v.toFixed(4)).toString();
}

function diffCalibrationKnobs(
  base: Record<string, unknown> | undefined,
  cand: Record<string, unknown> | undefined,
): { key: string; before: unknown; after: unknown }[] {
  if (!base || !cand) return [];
  const out: { key: string; before: unknown; after: unknown }[] = [];
  for (const k of CALIB_KNOB_KEYS) {
    if (base[k] === cand[k]) continue;
    out.push({ key: k, before: base[k], after: cand[k] });
  }
  return out;
}

const COLOR_TUNING_KEYS = [
  "brightestPaletteChromaBias",
  "brightestRankMinSaturation",
  "paletteSlotBlend",
] as const;

function diffColorTuningKnobs(
  base: Record<string, unknown> | undefined,
  cand: Record<string, unknown> | undefined,
): { key: string; before: unknown; after: unknown }[] {
  if (!base || !cand) return [];
  const b = base.colorTuning as Record<string, unknown> | undefined;
  const c = cand.colorTuning as Record<string, unknown> | undefined;
  if (!b && !c) return [];
  const out: { key: string; before: unknown; after: unknown }[] = [];
  for (const k of COLOR_TUNING_KEYS) {
    const bv = b?.[k];
    const cv = c?.[k];
    if (bv === cv) continue;
    out.push({ key: `colorTuning.${k}`, before: bv ?? "—", after: cv ?? "—" });
  }
  return out;
}

interface SuiteCalibrationMetricsShape {
  total?: number;
  accuracyCost?: number;
  lockCost?: number;
  stabilityCost?: number;
  lockedCells?: number;
  totalCells?: number;
}

interface CalibrationPanelProps {
  scenes: { id: string; name: string }[];
  selectedSceneId: string;
  onSelectScene: (id: string) => void;
  running: boolean;
  evalCountLive: number;
  report: Record<string, unknown> | null;
  actionState: "idle" | "busy" | "ok" | "error";
  onStart: () => void;
  onStop: () => void;
  onApply: () => void;
  onRollback: () => void;
  onCopyReport: () => void;
  onPromotePortableDefaults: () => void;
  onClearPortableDefaults: () => void;
}

function CalibrationPanel({
  scenes,
  selectedSceneId,
  onSelectScene,
  running,
  evalCountLive,
  report,
  actionState,
  onStart,
  onStop,
  onApply,
  onRollback,
  onCopyReport,
  onPromotePortableDefaults,
  onClearPortableDefaults,
}: CalibrationPanelProps) {
  const baselineCfg = report?.baselineConfig as Record<string, unknown> | undefined;
  const candidateCfg = report?.candidateConfig as Record<string, unknown> | undefined;
  const knobDiff = [...diffCalibrationKnobs(baselineCfg, candidateCfg), ...diffColorTuningKnobs(baselineCfg, candidateCfg)];
  const baselineM = report?.baselineMetrics as SuiteCalibrationMetricsShape | undefined;
  const candidateM = report?.candidateMetrics as SuiteCalibrationMetricsShape | undefined;
  const err = typeof report?.error === "string" ? (report.error as string) : "";
  const searchEval =
    typeof report?.searchEvalCount === "number" ? (report.searchEvalCount as number) : undefined;
  const evalShown = running ? evalCountLive : searchEval ?? evalCountLive;
  const totalDelta =
    baselineM?.total !== undefined && candidateM?.total !== undefined
      ? candidateM.total - baselineM.total
      : undefined;
  const canApply =
    !!selectedSceneId && !!report && !err && knobDiff.length > 0 && !running && actionState !== "busy";
  const canPromotePortable =
    !!report && !err && !running && actionState !== "busy";

  return (
    <div className="mt-4 rounded-xl border border-border bg-muted/15 overflow-hidden">
      <div className="flex flex-wrap items-start justify-between gap-3 p-4 border-b border-border">
        <div className="flex items-start gap-2 min-w-0">
          <div className="h-9 w-9 rounded-lg bg-muted flex items-center justify-center shrink-0">
            <SlidersHorizontal className="h-4 w-4 text-muted-foreground" />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-semibold">Screen sync calibration</h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              After a full suite, searches a <span className="text-foreground/80">subset</span> of
              built-in fixtures with bounded coordinate steps over global + temporal knobs. Nothing
              is saved until you click <span className="font-medium text-foreground">Apply</span>.
              Use <span className="font-medium text-foreground">Rollback</span> to restore the
              config from before the last successful apply for this scene.{" "}
              <span className="text-foreground/80">
                Save candidate as global default tuning
              </span>{" "}
              stores portable knobs from the last successful run; they merge at runtime only where a
              scene still matches factory defaults on each field.
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={selectedSceneId}
            onChange={(e) => onSelectScene(e.target.value)}
            disabled={running || scenes.length === 0}
            className="text-xs rounded-lg border border-border bg-background px-2 py-1.5 max-w-[220px]"
            title="screen_sync scene to tune"
          >
            {scenes.length === 0 ? (
              <option value="">No screen_sync scenes</option>
            ) : (
              scenes.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))
            )}
          </select>
          {!running ? (
            <button
              type="button"
              onClick={onStart}
              disabled={!selectedSceneId || actionState === "busy"}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-primary/15 text-primary hover:bg-primary/25 transition-colors disabled:opacity-40"
            >
              <Play className="h-3.5 w-3.5" />
              Calibrate
            </button>
          ) : (
            <button
              type="button"
              onClick={onStop}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-destructive/15 text-destructive hover:bg-destructive/25 transition-colors"
            >
              <Square className="h-3.5 w-3.5" />
              Stop
            </button>
          )}
        </div>
      </div>

      <div className="px-4 py-3 space-y-3 text-xs">
        {running && (
          <div className="flex items-center gap-2 text-muted-foreground">
            <Loader2 className="h-3.5 w-3.5 animate-spin shrink-0" />
            <span>
              Running coordinate search…{" "}
              <span className="font-mono text-foreground">{evalShown}</span> eval
              {evalShown === 1 ? "" : "s"}
            </span>
          </div>
        )}

        {actionState === "busy" && !running && (
          <p className="text-muted-foreground flex items-center gap-2">
            <Loader2 className="h-3.5 w-3.5 animate-spin" /> Working…
          </p>
        )}
        {actionState === "ok" && !running && (
          <p className="text-success text-[11px]">Done — check your scene or clipboard.</p>
        )}
        {actionState === "error" && !running && (
          <p className="text-destructive text-[11px]">Action failed — see console.</p>
        )}

        {report && err && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-destructive">
            {err}
          </div>
        )}

        {report && !err && (
          <>
            <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
              {searchEval !== undefined && (
                <span>
                  Search evaluations:{" "}
                  <span className="font-mono text-foreground">{searchEval}</span>
                </span>
              )}
              {totalDelta !== undefined && (
                <span>
                  Total cost Δ:{" "}
                  <span
                    className={
                      totalDelta < -1e-6
                        ? "text-success font-mono"
                        : totalDelta > 1e-6
                          ? "text-warning font-mono"
                          : "text-muted-foreground font-mono"
                    }
                  >
                    {totalDelta > 0 ? "+" : ""}
                    {totalDelta.toFixed(5)}
                  </span>{" "}
                  <span className="text-muted-foreground/70">(lower is better)</span>
                </span>
              )}
            </div>

            <div className="overflow-x-auto rounded-lg border border-border/60">
              <table className="w-full text-[11px] font-mono">
                <thead>
                  <tr className="text-muted-foreground border-b border-border bg-muted/30">
                    <th className="text-left px-2 py-1.5 font-medium">Metric</th>
                    <th className="text-right px-2 py-1.5 font-medium">Baseline</th>
                    <th className="text-right px-2 py-1.5 font-medium">Candidate</th>
                  </tr>
                </thead>
                <tbody>
                  {(
                    [
                      ["total", "Total"],
                      ["accuracyCost", "Accuracy"],
                      ["lockCost", "Lock"],
                      ["stabilityCost", "Stability"],
                      ["lockedCells", "Locked cells"],
                    ] as const
                  ).map(([k, label]) => {
                    const bv = baselineM?.[k];
                    const cv = candidateM?.[k];
                    return (
                      <tr key={k} className="border-b border-border/40">
                        <td className="px-2 py-1 text-foreground/90">{label}</td>
                        <td className="px-2 py-1 text-right">
                          {bv === undefined ? "—" : typeof bv === "number" ? bv.toFixed(4) : String(bv)}
                        </td>
                        <td className="px-2 py-1 text-right">
                          {cv === undefined ? "—" : typeof cv === "number" ? cv.toFixed(4) : String(cv)}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {knobDiff.length === 0 ? (
              <p className="text-muted-foreground text-[11px]">
                No knob changes vs baseline — candidate matches current tuning (or search hit
                limits).
              </p>
            ) : (
              <div className="overflow-x-auto rounded-lg border border-border/60">
                <table className="w-full text-[11px]">
                  <thead>
                    <tr className="text-muted-foreground border-b border-border bg-muted/30">
                      <th className="text-left px-2 py-1.5 font-medium">Knob</th>
                      <th className="text-right px-2 py-1.5 font-medium">Current</th>
                      <th className="text-right px-2 py-1.5 font-medium">Proposed</th>
                    </tr>
                  </thead>
                  <tbody>
                    {knobDiff.map((row) => (
                      <tr key={row.key} className="border-b border-border/40">
                        <td className="px-2 py-1">{humanizeKnobKey(row.key)}</td>
                        <td className="px-2 py-1 text-right font-mono">
                          {formatKnobValue(row.key, row.before)}
                        </td>
                        <td className="px-2 py-1 text-right font-mono text-primary font-medium">
                          {formatKnobValue(row.key, row.after)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            <div className="flex flex-wrap gap-2 pt-1">
              <button
                type="button"
                onClick={onApply}
                disabled={!canApply}
                className="px-3 py-1.5 rounded-lg text-xs bg-primary text-primary-foreground hover:opacity-90 disabled:opacity-40"
              >
                Apply to scene
              </button>
              <button
                type="button"
                onClick={onRollback}
                disabled={!selectedSceneId || running || actionState === "busy"}
                className="px-3 py-1.5 rounded-lg text-xs border border-border hover:bg-muted/50 disabled:opacity-40"
              >
                Rollback last apply
              </button>
              <button
                type="button"
                onClick={onCopyReport}
                disabled={running || actionState === "busy"}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-muted hover:bg-muted/80 disabled:opacity-40"
              >
                <ClipboardCopy className="h-3.5 w-3.5" />
                Copy report JSON
              </button>
              <button
                type="button"
                onClick={onPromotePortableDefaults}
                disabled={!canPromotePortable}
                className="px-3 py-1.5 rounded-lg text-xs border border-primary/40 text-primary hover:bg-primary/10 disabled:opacity-40"
                title="Persist the calibration candidate as global portable defaults (merge where scenes still use factory values)."
              >
                Save candidate as global default tuning
              </button>
            </div>
          </>
        )}

        {!report && !running && (
          <p className="text-muted-foreground text-[11px]">
            Run <span className="font-medium text-foreground">Calibrate</span> to evaluate candidates.
            When finished, review metrics and changed knobs, then apply or discard.
          </p>
        )}

        <div className="pt-2 mt-1 border-t border-border/60">
          <button
            type="button"
            onClick={onClearPortableDefaults}
            disabled={running || actionState === "busy"}
            className="px-3 py-1.5 rounded-lg text-xs border border-border text-muted-foreground hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 disabled:opacity-40"
            title="Remove saved global portable defaults."
          >
            Clear global default tuning
          </button>
        </div>
      </div>
    </div>
  );
}

interface SuiteReportProps {
  result: SuiteRun;
  modes: ModeCase[];
  onCopy: () => void;
  copyState: "idle" | "copying" | "copied" | "error";
  onDismiss: () => void;
}

function SuiteReport({ result, modes, onCopy, copyState, onDismiss }: SuiteReportProps) {
  // Build a quick lookup: fixtureId → modeId → cell summary.
  const byFixture = useMemo(() => {
    const m: Record<string, Record<string, { locked: boolean; verdict: string; hasExpected: boolean }>> = {};
    for (const fr of result.fixtures) {
      m[fr.fixtureId] = {};
      for (const c of fr.diagnostic.cells) {
        m[fr.fixtureId][c.modeId] = {
          locked: c.locked,
          verdict: c.verdict,
          hasExpected: (c.expected?.length ?? 0) > 0,
        };
      }
    }
    return m;
  }, [result]);

  const modesByGroup = useMemo(() => {
    const g: Record<string, ModeCase[]> = { single: [], palette: [], spatial: [] };
    for (const m of modes) {
      if (!g[m.group]) g[m.group] = [];
      g[m.group].push(m);
    }
    return g;
  }, [modes]);

  const overallPct =
    result.totalCells > 0 ? Math.round((result.lockedCells / result.totalCells) * 100) : 0;
  const overallColor =
    overallPct >= 95 ? "text-success" : overallPct >= 80 ? "text-warning" : "text-destructive";

  return (
    <div className="rounded-xl border border-border bg-muted/20 overflow-hidden">
      <div className="flex items-start justify-between gap-4 p-4 border-b border-border">
        <div>
          <h2 className="text-sm font-semibold flex items-center gap-2">
            <CheckCircle2 className={`h-4 w-4 ${overallColor}`} />
            Suite Results
          </h2>
          <p className="text-xs text-muted-foreground mt-1">
            <span className={`font-mono font-semibold ${overallColor}`}>
              {result.lockedCells}/{result.totalCells}
            </span>{" "}
            cells locked ({overallPct}%) across {result.fixtureCount} fixtures · total{" "}
            {(result.totalMs / 1000).toFixed(1)}s
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onCopy}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs transition-colors ${
              copyState === "copied"
                ? "bg-success/15 text-success"
                : copyState === "error"
                  ? "bg-destructive/15 text-destructive"
                  : "bg-primary/10 text-primary hover:bg-primary/20"
            }`}
          >
            <ClipboardCopy className="h-3.5 w-3.5" />
            {copyState === "copied"
              ? "Copied"
              : copyState === "error"
                ? "Copy failed"
                : copyState === "copying"
                  ? "Copying…"
                  : "Copy Full Report"}
          </button>
          <button
            type="button"
            onClick={onDismiss}
            className="px-2 py-1.5 rounded-lg text-xs text-muted-foreground hover:bg-muted"
            title="Hide report"
          >
            ×
          </button>
        </div>
      </div>

      {/* Per-fixture summary table */}
      <div className="overflow-x-auto">
        <table className="w-full text-[11px] font-mono">
          <thead>
            <tr className="text-muted-foreground border-b border-border">
              <th className="text-left px-3 py-2 font-medium sticky left-0 bg-muted/40">fixture</th>
              <th className="text-right px-3 py-2 font-medium">locked</th>
              {(["single", "palette", "spatial"] as const).map((group) =>
                modesByGroup[group]?.map((m) => (
                  <th
                    key={m.id}
                    className="text-center px-1 py-2 font-medium whitespace-nowrap"
                    title={m.label}
                  >
                    {m.id.replace(`${group}-`, `${group[0]}·`)}
                  </th>
                ))
              )}
            </tr>
          </thead>
          <tbody>
            {result.fixtures.map((fr) => (
              <tr key={fr.fixtureId} className="border-b border-border/40 hover:bg-muted/30">
                <td
                  className="px-3 py-1.5 sticky left-0 bg-muted/10 whitespace-nowrap"
                  title={fr.fixtureId}
                >
                  {fr.fixtureLabel}
                </td>
                <td className="px-3 py-1.5 text-right">
                  <span
                    className={
                      fr.lockedCells === fr.totalCells && fr.totalCells > 0
                        ? "text-success"
                        : fr.lockedCells > fr.totalCells * 0.6
                          ? "text-warning"
                          : "text-destructive"
                    }
                  >
                    {fr.lockedCells}/{fr.totalCells}
                  </span>
                </td>
                {(["single", "palette", "spatial"] as const).map((group) =>
                  modesByGroup[group]?.map((m) => {
                    const cell = byFixture[fr.fixtureId]?.[m.id];
                    const icon = !cell
                      ? "—"
                      : !cell.hasExpected
                        ? "·"
                        : cell.locked
                          ? "✓"
                          : "✗";
                    const color = !cell
                      ? "text-muted-foreground/30"
                      : !cell.hasExpected
                        ? "text-muted-foreground/40"
                        : cell.locked
                          ? "text-success"
                          : "text-destructive";
                    const title = cell
                      ? `${m.label}\n${cell.locked ? "locked ✓" : `verdict: ${cell.verdict}`}${
                          cell.hasExpected ? "" : "\n(no baseline)"
                        }`
                      : m.label;
                    return (
                      <td
                        key={m.id}
                        className={`text-center px-1 py-1.5 ${color}`}
                        title={title}
                      >
                        {icon}
                      </td>
                    );
                  })
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <p className="text-[10px] text-muted-foreground/60 px-4 py-2 border-t border-border">
        ✓ locked · ✗ out of tolerance · · no baseline for this mode on this fixture
      </p>
    </div>
  );
}
