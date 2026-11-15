import { router } from "expo-router";
import { useEffect, useState } from "react";

import { errorMessage } from "../../api/errors";
import { fromProto } from "../../api/convert";
import { useApi } from "../../api/useApi";
import { useStore } from "../../data/StoreProvider";
import type { ResolvedDay, Unit } from "../../domain/types";
import { ProgramKind, type CatalogProgram } from "../../gen/plusultra/v1/program_pb";
import { BackButton, Button, Card, ConfirmSheet, ErrorLine, Screen, Text } from "../../ui";
import { remember } from "./programStore";
import { levelLabel } from "./summary";
import { WeekPlan } from "./WeekPlan";

// A built-in program: what it is, a simulated preview, and a button that copies it into a draft.
export default function CatalogProgramScreen({ slug, unit = "kg" }: { slug: string; unit?: Unit }) {
  const api = useApi();
  const store = useStore();
  const [program, setProgram] = useState<CatalogProgram>();
  const [days, setDays] = useState<ResolvedDay[]>([]);
  const [week, setWeek] = useState(1);
  const [message, setMessage] = useState<string>();
  const [adding, setAdding] = useState(false);
  const [busy, setBusy] = useState(false);
  const [addError, setAddError] = useState<string>();

  useEffect(() => {
    api.programs
      .getCatalogProgram({ slug })
      .then(async ({ program: p }) => {
        if (!p) return;
        setProgram(p);
        const r = await api.programs.simulateProgram({ script: p.script, weeks: p.weeks });
        setDays(r.days.map((d) => fromProto.resolvedDay(d).payload));
      })
      .catch((e) => setMessage(errorMessage(e)));
  }, [api, slug]);

  const add = async () => {
    setBusy(true);
    try {
      const { program: p } = await api.programs.createProgram({
        kind: ProgramKind.WORKOUT,
        script: program!.script,
      });
      remember(store, p!);
      router.replace(`/program/${p!.id}`);
    } catch (e) {
      setAddError(errorMessage(e));
    }
    setBusy(false);
  };

  return (
    <Screen title={program?.name ?? "Catalog"} left={<BackButton fallback="/programs" />}>
      <ErrorLine message={message} />
      {program && (
        <>
          <Card>
            <Text variant="label">
              {`${levelLabel[program.level] ?? program.level}, ${program.daysPerWeek} days a week`}
            </Text>
            <Text>{`${program.weeks} weeks${program.loop ? ", repeats" : ""}`}</Text>
            <Text color="textMuted">{`After ${program.author}`}</Text>
            <Text>{program.summary}</Text>
          </Card>
          {program.maxes.length > 0 && (
            <>
              <Text color="textMuted">
                {`Uses your 1RM for ${program.maxes.join(", ")}. Set them under Settings › Maxes before activating.`}
              </Text>
              <Button
                title="Open Maxes"
                variant="ghost"
                onPress={() => router.push("/settings/maxes")}
              />
            </>
          )}
          {days.length > 0 && (
            <>
              <Text variant="heading">Preview</Text>
              <WeekPlan
                unit={unit}
                weeks={program.weeks}
                days={days}
                plans={days}
                week={week}
                onWeek={setWeek}
              />
            </>
          )}
          <Button title="Add to my programs" icon="add" onPress={() => setAdding(true)} />
          <ConfirmSheet
            open={adding}
            title={`Add ${program.name}?`}
            message="It's saved as a draft under Mine, with no dates. Review or edit it there, then Activate to pick a start date or run it floating."
            action="Add as draft"
            variant="primary"
            busy={busy}
            error={addError}
            onConfirm={add}
            onClose={() => setAdding(false)}
          />
        </>
      )}
    </Screen>
  );
}
