import { useState } from "react";
import { Monitor, Crop, AppWindow, Focus, ChevronRight, X, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/Button";
import type { ScreenSyncConfig, CaptureMode, ColorMode, ExtractionMethod } from "@/lib/types";

interface SetupWizardProps {
  config: ScreenSyncConfig;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
  onFinish: () => void;
  onSkip: () => void;
}

type WizardStep = "capture" | "colors" | "summary";

const captureModeOptions: { value: CaptureMode; icon: typeof Monitor; label: string; desc: string }[] = [
  { value: "monitor",       icon: Monitor,   label: "Monitor",        desc: "Capture a full display" },
  { value: "region",        icon: Crop,      label: "Custom Region",  desc: "Define a screen area" },
  { value: "window",        icon: AppWindow, label: "Application",    desc: "Follow a specific app" },
  { value: "active_window", icon: Focus,     label: "Active Window",  desc: "Follow focused app" },
];

const colorModeOptions: { value: ColorMode; label: string; desc: string }[] = [
  { value: "single", label: "Single Color", desc: "All lights show the same extracted color" },
  { value: "multi",  label: "Multi Color",  desc: "Each light gets a unique color from the screen" },
];

const extractionOptions: { value: ExtractionMethod; label: string; desc: string }[] = [
  { value: "dominant",  label: "Dominant",  desc: "Most common color by area — recommended" },
  { value: "brightest", label: "Brightest", desc: "The most luminous color" },
  { value: "saturated", label: "Saturated", desc: "The most vivid color" },
];

export function SetupWizard({ config, onChange, onFinish, onSkip }: SetupWizardProps) {
  const [step, setStep] = useState<WizardStep>("capture");

  const steps: WizardStep[] = ["capture", "colors", "summary"];
  const stepIndex = steps.indexOf(step);

  const next = () => {
    const nextStep = steps[stepIndex + 1];
    if (nextStep) setStep(nextStep);
    else onFinish();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-card border border-border rounded-2xl w-[480px] shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 pt-6 pb-4">
          <div className="flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-bold">Screen Sync Setup</h2>
          </div>
          <button type="button" onClick={onSkip} className="text-muted-foreground hover:text-foreground">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Progress */}
        <div className="flex gap-1 px-6 mb-5">
          {steps.map((s, i) => (
            <div
              key={s}
              className={`h-1 flex-1 rounded-full transition-colors ${
                i <= stepIndex ? "bg-primary" : "bg-white/10"
              }`}
            />
          ))}
        </div>

        {/* Step content */}
        <div className="px-6 pb-6">
          {step === "capture" && (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Choose how you'd like to capture your screen content.
              </p>
              <div className="grid grid-cols-2 gap-2">
                {captureModeOptions.map(({ value, icon: Icon, label, desc }) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => onChange({ captureMode: value })}
                    className={`flex flex-col items-start gap-2 p-4 rounded-xl text-left transition-all ${
                      config.captureMode === value
                        ? "bg-primary/15 ring-1 ring-primary/50"
                        : "bg-background/40 hover:bg-background/70"
                    }`}
                  >
                    <Icon className={`h-5 w-5 ${config.captureMode === value ? "text-primary" : "text-muted-foreground"}`} />
                    <div>
                      <p className="text-sm font-semibold">{label}</p>
                      <p className="text-xs text-muted-foreground">{desc}</p>
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}

          {step === "colors" && (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                How should extracted colors be applied to your lights?
              </p>
              <div className="space-y-2">
                {colorModeOptions.map(({ value, label, desc }) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => onChange({ colorMode: value })}
                    className={`w-full flex items-start gap-3 p-3.5 rounded-xl text-left transition-all ${
                      config.colorMode === value
                        ? "bg-primary/15 ring-1 ring-primary/50"
                        : "bg-background/40 hover:bg-background/70"
                    }`}
                  >
                    <div className={`h-3 w-3 rounded-full mt-1 shrink-0 ring-2 ${
                      config.colorMode === value ? "bg-primary ring-primary/30" : "bg-transparent ring-border"
                    }`} />
                    <div>
                      <p className="text-sm font-semibold">{label}</p>
                      <p className="text-xs text-muted-foreground">{desc}</p>
                    </div>
                  </button>
                ))}
              </div>

              <div>
                <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2">
                  Extraction Method
                </p>
                <div className="space-y-1.5">
                  {extractionOptions.map(({ value, label, desc }) => (
                    <button
                      key={value}
                      type="button"
                      onClick={() => onChange({ extractionMethod: value })}
                      className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-left transition-all ${
                        config.extractionMethod === value
                          ? "bg-primary/15 ring-1 ring-primary/30"
                          : "bg-background/30 hover:bg-background/60"
                      }`}
                    >
                      <span className="text-sm font-medium">{label}</span>
                      <span className="text-xs text-muted-foreground ml-4 text-right">{desc}</span>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}

          {step === "summary" && (
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                You're all set! Here's a summary of your configuration:
              </p>
              <div className="bg-background/40 rounded-xl p-4 space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Capture mode</span>
                  <span className="font-medium capitalize">{config.captureMode.replace("_", " ")}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Color mode</span>
                  <span className="font-medium capitalize">{config.colorMode.replace("_", " ")}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Extraction</span>
                  <span className="font-medium capitalize">{config.extractionMethod}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Speed</span>
                  <span className="font-medium capitalize">{config.speedPreset.replace("_", " ")}</span>
                </div>
              </div>
              <p className="text-xs text-muted-foreground">
                All other settings have sensible defaults. You can adjust them in the scene editor after creation.
              </p>
            </div>
          )}

          {/* Actions */}
          <div className="flex items-center justify-between mt-6">
            <button
              type="button"
              onClick={onSkip}
              className="text-sm text-muted-foreground hover:text-foreground"
            >
              Skip setup
            </button>
            <Button onClick={next}>
              {step === "summary" ? "Finish" : "Continue"}
              {step !== "summary" && <ChevronRight className="h-4 w-4 ml-1" />}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
