import { timestampDate } from "@bufbuild/protobuf/wkt";
import { router, useFocusEffect } from "expo-router";
import { useCallback, useState } from "react";
import { Share } from "react-native";

import { errorMessage } from "../../api/errors";
import { useApi } from "../../api/useApi";
import { fromProto } from "../../api/convert";
import { programsRepo } from "../../data/repositories/programs";
import { useStore } from "../../data/StoreProvider";
import { useSync } from "../../data/useSync";
import { formatDateTime } from "../../domain/dates";
import type { MealDay, ResolvedDay, Unit } from "../../domain/types";
import {
  ProgramStatus,
  type Program,
  type ProgramRevision,
} from "../../gen/plusultra/v1/program_pb";
import {
  BackButton,
  Button,
  Card,
  ConfirmSheet,
  EmptyState,
  ErrorLine,
  IconButton,
  ListCard,
  ListRow,
  Screen,
  Sheet,
  Text,
} from "../../ui";
import { ActivateSheet } from "./ActivateSheet";
import { MealWeekPlan } from "./MealWeekPlan";
import {
  cachedDays,
  cachedMealDays,
  remember,
  rememberDays,
  rememberMealDays,
} from "./programStore";
import { kindLabel, modeLabel, statusLabel } from "./summary";
import { WeekPlan } from "./WeekPlan";

const PAGE = 20;

type Confirm = { title: string; message: string; action: string; run: () => Promise<void> };

export default function ProgramDetailScreen({ id, unit = "kg" }: { id: string; unit?: Unit }) {
  const api = useApi();
  const store = useStore();
  const { syncNow } = useSync();
  const [summary, setSummary] = useState(() => programsRepo(store).get(id));
  const [program, setProgram] = useState<Program>();
  const [revisions, setRevisions] = useState<ProgramRevision[]>([]);
  const [nextCursor, setNextCursor] = useState("");
  const [message, setMessage] = useState<string>();
  const [confirm, setConfirm] = useState<Confirm>();
  const [busy, setBusy] = useState(false);
  const [activating, setActivating] = useState(false);
  const [more, setMore] = useState(false);
  const [week, setWeek] = useState(1);
  const [days, setDays] = useState<ResolvedDay[]>(() => cachedDays(store, id));
  const [mealDays, setMealDays] = useState<MealDay[]>(() => cachedMealDays(store, id));

  const apply = useCallback(
    (p: Program) => {
      remember(store, p);
      setProgram(p);
    },
    [store],
  );

  // Only an active program has resolved days; a draft or finished program is simulated instead.
  const loadDays = useCallback(
    (p: Program) => {
      if (p.status === ProgramStatus.ACTIVE) {
        api.programs.getResolvedDays({ programId: id }).then(
          (r) => {
            const loaded = r.days.map((d) => fromProto.resolvedDay(d).payload);
            rememberDays(store, id, loaded);
            setDays(loaded);
            const meals = r.mealDays.map((d) => fromProto.mealDay(d).payload);
            rememberMealDays(store, id, meals);
            setMealDays(meals);
          },
          () => {},
        );
      } else {
        api.programs.simulateProgram({ script: p.script, weeks: p.structure?.weeks || 1 }).then(
          (r) => {
            const loaded = r.days.map((d) => fromProto.resolvedDay(d).payload);
            rememberDays(store, id, loaded);
            setDays(loaded);
            const meals = r.mealDays.map((d) => fromProto.mealDay(d).payload);
            rememberMealDays(store, id, meals);
            setMealDays(meals);
          },
          () => {},
        );
      }
    },
    [api, store, id],
  );

  const load = useCallback(() => {
    setSummary(programsRepo(store).get(id));
    api.programs.getProgram({ id }).then(
      ({ program: p }) => {
        if (!p) return;
        apply(p);
        loadDays(p);
      },
      (e) => setMessage(errorMessage(e)),
    );
    api.programs.listRevisions({ programId: id, limit: PAGE }).then(
      (r) => {
        setRevisions(r.revisions);
        setNextCursor(r.nextCursor);
      },
      () => {},
    );
  }, [api, store, id, apply, loadDays]);
  useFocusEffect(load);

  const loadMore = async () => {
    const r = await api.programs.listRevisions({ programId: id, limit: PAGE, cursor: nextCursor });
    setRevisions((all) => [...all, ...r.revisions]);
    setNextCursor(r.nextCursor);
  };

  const exportScript = async () => {
    try {
      const { script, filename } = await api.export.exportProgram({ id });
      await Share.share({ title: filename, message: script });
    } catch (e) {
      setMessage(errorMessage(e));
    }
  };

  const ask = (c: Confirm) => {
    setMore(false);
    setConfirm(c);
  };

  const runConfirmed = async () => {
    setBusy(true);
    try {
      await confirm!.run();
    } catch (e) {
      setMessage(errorMessage(e));
    }
    setBusy(false);
    setConfirm(undefined);
  };

  const view = program ? fromProto.program(program) : summary;
  const structure = program?.structure;

  return (
    <Screen
      title={view?.name ?? "Program"}
      testID="program-screen"
      left={<BackButton fallback="/programs" />}
      right={
        view && <IconButton icon="ellipsis-horizontal" label="More" onPress={() => setMore(true)} />
      }
    >
      <ErrorLine message={message} />
      {!view ? (
        <EmptyState
          icon="cloud-offline-outline"
          title="Program not available"
          message="Connect to your server to load it."
        />
      ) : (
        <>
          <Card>
            <Text variant="label">{`${kindLabel[view.kind]}, ${statusLabel[view.status]}`}</Text>
            <Text>{modeLabel(view)}</Text>
            {structure && (
              <Text color="textMuted">{`${structure.weeks} weeks, ${structure.days.length} days`}</Text>
            )}
          </Card>
          {view.status === "active" ? (
            <Button
              title="Finish"
              icon="flag-outline"
              variant="secondary"
              onPress={() =>
                ask({
                  title: "Finish this program?",
                  message: "It stops planning days. Logged workouts and meals are kept.",
                  action: "Finish program",
                  run: async () => {
                    const { program: p } = await api.programs.finishProgram({ id });
                    apply(p!);
                    void syncNow();
                  },
                })
              }
            />
          ) : (
            <Button title="Activate" icon="play" onPress={() => setActivating(true)} />
          )}
          {view.kind === "meal" ? (
            <MealWeekPlan
              programId={id}
              weeks={view.weeks}
              days={mealDays}
              week={week}
              onWeek={setWeek}
            />
          ) : (
            (structure || days.length > 0) && (
              <WeekPlan
                programId={id}
                unit={unit}
                weeks={view.weeks}
                days={structure?.days ?? days}
                plans={days}
                week={week}
                onWeek={setWeek}
              />
            )
          )}
          {revisions.length > 0 && (
            <>
              <Text variant="heading">Revisions</Text>
              <ListCard>
                {revisions.map((r) => (
                  <ListRow
                    key={r.revision}
                    title={`Revision ${r.revision}`}
                    subtitle={`${r.createdBy}, ${r.createdAt ? formatDateTime(timestampDate(r.createdAt)) : ""}`}
                    value={r.structure && `${r.structure.weeks} wk`}
                  />
                ))}
              </ListCard>
              {nextCursor && <Button title="Load more" variant="ghost" onPress={loadMore} />}
            </>
          )}
          <ActivateSheet
            program={view}
            open={activating}
            onClose={() => setActivating(false)}
            onActivated={(p) => {
              apply(p);
              setActivating(false);
              // The server resolved the new plan; pull it so Today and the calendar show it.
              void syncNow();
            }}
          />
        </>
      )}
      <Sheet open={more} onClose={() => setMore(false)} title={view?.name} snapPoints={[0.4]}>
        <ListCard>
          <ListRow
            title="Edit"
            icon="create-outline"
            onPress={() => {
              setMore(false);
              router.push({ pathname: "/program/edit", params: { id } });
            }}
          />
          <ListRow
            title="Export"
            icon="share-outline"
            onPress={() => {
              setMore(false);
              void exportScript();
            }}
          />
          <ListRow
            title="Delete"
            icon="trash-outline"
            tone="danger"
            onPress={() =>
              ask({
                title: "Delete this program?",
                message: "This cannot be undone. Past workout and meal logs are kept.",
                action: "Delete program",
                run: async () => {
                  await api.programs.deleteProgram({ id });
                  programsRepo(store).remove(id);
                  router.back();
                },
              })
            }
          />
        </ListCard>
      </Sheet>
      <ConfirmSheet
        open={!!confirm}
        title={confirm?.title ?? ""}
        message={confirm?.message ?? ""}
        action={confirm?.action ?? ""}
        busy={busy}
        onConfirm={runConfirmed}
        onClose={() => setConfirm(undefined)}
      />
    </Screen>
  );
}
