import { ShieldX } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

export function Forbidden() {
  return (
    <Card role="region" className="mx-auto mt-10 max-w-md" aria-labelledby="forbidden-title">
      <CardContent className="grid justify-items-center gap-3 py-8 text-center">
        <ShieldX className="size-10 text-muted-foreground" aria-hidden />
        <h1 id="forbidden-title" className="text-xl font-semibold">
          Not permitted
        </h1>
        <p className="text-sm text-muted-foreground">Your role does not include this workspace. Ask a manager if you need access.</p>
      </CardContent>
    </Card>
  );
}
