import { cn } from "@/lib/utils";
import { HTMLAttributes, forwardRef } from "react";

const Card = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn("rounded-xl bg-card p-6", className)}
      {...props}
    />
  )
);
Card.displayName = "Card";

export { Card };
