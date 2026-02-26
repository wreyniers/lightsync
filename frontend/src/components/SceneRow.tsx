import { Play, Pencil, Trash2 } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { sceneSwatchBackground } from "@/lib/sceneColors";
import type { Scene } from "@/lib/types";

interface SceneRowProps {
  scene: Scene;
  isActive: boolean;
  onActivate: () => void;
  onEdit: () => void;
  onDelete: () => void;
}

export function SceneRow({
  scene,
  isActive,
  onActivate,
  onEdit,
  onDelete,
}: SceneRowProps) {
  const swatchBg = sceneSwatchBackground(scene);

  const triggerLabel =
    scene.trigger === "camera_on"
      ? "Trigger: Camera On"
      : scene.trigger === "camera_off"
      ? "Trigger: Camera Off"
      : "Manual only";

  return (
    <Card className="p-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div
            className="h-10 w-10 shrink-0 rounded-full ring-1 ring-border"
            style={{ background: swatchBg ?? "hsl(var(--muted))" }}
          />
          <div>
            <div className="flex items-center gap-2">
              <p className="font-semibold">{scene.name}</p>
              {isActive && <Badge variant="success">Active</Badge>}
            </div>
            <p className="text-xs text-muted-foreground">
              {triggerLabel} · {Object.keys(scene.devices || {}).length} light(s)
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex gap-1">
            <Button variant="ghost" size="icon" onClick={onActivate} title="Activate">
              <Play className="h-4 w-4 text-primary" />
            </Button>
            <Button variant="ghost" size="icon" onClick={onEdit} title="Edit">
              <Pencil className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" onClick={onDelete} title="Delete">
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </div>
        </div>
      </div>
    </Card>
  );
}
