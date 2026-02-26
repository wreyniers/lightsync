import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import {
  Lightbulb,
  Film,
  Settings,
  Camera,
  CameraOff,
} from "lucide-react";
import { useWailsEvent } from "@/hooks/useWails";
import { Toggle } from "@/components/ui/Toggle";
import type { Scene } from "@/lib/types";
import {
  IsMonitoringEnabled,
  SetMonitoringEnabled,
  GetScenes,
  GetActiveScene,
} from "../../wailsjs/go/main/App";

interface LayoutProps {
  children: React.ReactNode;
  currentPage: string;
  onNavigate: (page: string) => void;
}

const navItems = [
  { id: "lights", label: "Lights", icon: Lightbulb },
  { id: "scenes", label: "Scenes", icon: Film },
  { id: "settings", label: "Settings", icon: Settings },
];

export function Layout({ children, currentPage, onNavigate }: LayoutProps) {
  const cameraOn = useWailsEvent<boolean>("camera:state", false);
  const activeSceneEvent = useWailsEvent<string>("scene:active", "");
  const [monitoring, setMonitoring] = useState(true);
  const [scenes, setScenes] = useState<Scene[]>([]);
  const [activeSceneId, setActiveSceneId] = useState("");

  useEffect(() => {
    IsMonitoringEnabled().then(setMonitoring).catch(() => {});
    GetScenes().then((s) => setScenes(s || [])).catch(() => {});
    GetActiveScene().then(setActiveSceneId).catch(() => {});
  }, []);

  useEffect(() => {
    if (activeSceneEvent) setActiveSceneId(activeSceneEvent);
  }, [activeSceneEvent]);

  const activeSceneName = scenes.find((s) => s.id === activeSceneId)?.name;

  const handleToggleMonitoring = (enabled: boolean) => {
    SetMonitoringEnabled(enabled);
    setMonitoring(enabled);
  };

  return (
    <div className="flex h-screen">
      <aside className="w-64 border-r border-border bg-card flex flex-col">
        <div className="p-6">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-lg bg-primary/20 flex items-center justify-center">
              <Lightbulb className="h-5 w-5 text-primary" />
            </div>
            <div>
              <h1 className="text-lg font-bold tracking-tight">LightSync</h1>
              <p className="text-xs text-muted-foreground">Webcam to Lights</p>
            </div>
          </div>
        </div>

        <div className="px-3 mb-4 space-y-2">
          <div
            className={cn(
              "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm",
              cameraOn
                ? "bg-success/10 text-success"
                : "bg-muted text-muted-foreground"
            )}
          >
            {cameraOn ? (
              <Camera className="h-4 w-4" />
            ) : (
              <CameraOff className="h-4 w-4" />
            )}
            <span className="font-medium">
              Camera {cameraOn ? "Active" : "Off"}
            </span>
            <span
              className={cn(
                "ml-auto h-2 w-2 rounded-full",
                cameraOn ? "bg-success animate-pulse" : "bg-muted-foreground/40"
              )}
            />
          </div>

          <div
            className={cn(
              "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm",
              activeSceneName
                ? "bg-primary/10 text-primary"
                : "bg-muted text-muted-foreground"
            )}
          >
            <Film className="h-4 w-4" />
            <span className="font-medium truncate">
              {activeSceneName || "No Scene"}
            </span>
            <span
              className={cn(
                "ml-auto h-2 w-2 shrink-0 rounded-full",
                activeSceneName ? "bg-primary animate-pulse" : "bg-muted-foreground/40"
              )}
            />
          </div>
        </div>

        <nav className="flex-1 px-3 space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = currentPage === item.id;
            return (
              <button
                key={item.id}
                onClick={() => onNavigate(item.id)}
                className={cn(
                  "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                  active
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-secondary hover:text-foreground"
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </button>
            );
          })}
        </nav>

        <div className="flex items-center justify-between px-6 py-3">
          <span className="text-sm font-medium text-muted-foreground">Webcam Monitoring</span>
          <Toggle checked={monitoring} onChange={handleToggleMonitoring} />
        </div>

        <div className="p-4 border-t border-border">
          <p className="text-xs text-muted-foreground text-center">
            LightSync v1.0.0
          </p>
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <div className="p-8">{children}</div>
      </main>
    </div>
  );
}
