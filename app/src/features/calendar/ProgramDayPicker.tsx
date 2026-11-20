import { useEffect, useRef, useState } from "react";
import { ScrollView } from "react-native";

import { errorMessage } from "../../api/errors";
import type { ProgramSummary } from "../../domain/types";
import {
  Button,
  Chip,
  ChipStrip,
  EmptyState,
  ListCard,
  ListRow,
  Loading,
  Sheet,
  SheetFooter,
  space,
  Text,
} from "../../ui";
import { statusLabel } from "../programs/summary";

// A day the picker offers; create writes the record or log built from it.
export interface PickDay {
  week: number;
  day: number;
  name: string;
  kind: string;
  summary: string;
  create(): void;
}

export type Planned = Pick<PickDay, "week" | "day" | "kind"> & { programId: string };

type Props = {
  open: boolean;
  onClose(): void;
  title: string;
  programs: ProgramSummary[];
  /** No draft or active program of the kind, though finished ones exist. */
  finishedOnly?: boolean;
  loadDays(program: ProgramSummary): Promise<PickDay[]>;
  /** Days known offline, shown at once; loadDays' answer replaces them and its failure keeps them. */
  cached?(program: ProgramSummary): PickDay[];
  // The calendar's own day for the date: preselected, never applied by itself.
  planned?: Planned;
};

const dayKey = (d: Pick<PickDay, "week" | "day" | "kind">) => `${d.week}:${d.day}:${d.kind}`;
const programIcon = (p: ProgramSummary) =>
  p.kind === "meal" ? "restaurant-outline" : "barbell-outline";

// Program, then week chips and day rows that scroll, then Create pinned under them. The body
// mounts per opening so its state starts from the planned day each time.
export function ProgramDayPicker({ open, onClose, title, ...body }: Props) {
  return (
    <Sheet
      open={open}
      onClose={onClose}
      title={title}
      snapPoints={[0.9]}
      scroll={false}
      testID="day-picker"
    >
      {open && <Body {...body} onClose={onClose} />}
    </Sheet>
  );
}

// An anchored program repeats a day per date; each is listed once.
function unique(all: PickDay[]) {
  const seen = new Set<string>();
  return all.filter((d) => !seen.has(dayKey(d)) && !!seen.add(dayKey(d)));
}

function Body({
  programs,
  finishedOnly,
  loadDays,
  cached,
  planned,
  onClose,
}: Omit<Props, "open" | "title">) {
  const [program, setProgram] = useState(() => programs.find((p) => p.id === planned?.programId));
  const [days, setDays] = useState<PickDay[]>();
  const [error, setError] = useState<string>();
  const [week, setWeek] = useState(planned?.week ?? 1);
  const [selected, setSelected] = useState(planned && dayKey(planned));
  const source = useRef({ loadDays, cached });
  useEffect(() => {
    source.current = { loadDays, cached };
  });
  useEffect(() => {
    if (!program) return;
    let stale = false;
    const known = source.current.cached?.(program) ?? [];
    if (known.length) setDays(unique(known));
    source.current.loadDays(program).then(
      (all) => {
        if (!stale) setDays(unique(all));
      },
      (e) => {
        if (stale || known.length) return;
        setError(errorMessage(e));
        setDays([]);
      },
    );
    return () => {
      stale = true;
    };
  }, [program]);

  const choose = (p?: ProgramSummary) => {
    setProgram(p);
    setDays(undefined);
    setError(undefined);
    setWeek(1);
    setSelected(undefined);
  };
  const content = { gap: space[3], paddingBottom: space[6] };

  if (!program) {
    return (
      <ScrollView contentContainerStyle={content}>
        {programs.length === 0 ? (
          finishedOnly ? (
            <EmptyState
              icon="barbell-outline"
              title="No active programs"
              message="Activate a program to pick a day."
            />
          ) : (
            <EmptyState
              icon="barbell-outline"
              title="No programs"
              message="Write a program first."
            />
          )
        ) : (
          <ListCard>
            {programs.map((p) => (
              <ListRow
                key={p.id}
                icon={programIcon(p)}
                title={p.name}
                subtitle={`${statusLabel[p.status]}, ${p.weeks} wk`}
                onPress={() => choose(p)}
              />
            ))}
          </ListCard>
        )}
      </ScrollView>
    );
  }

  const weeks = Array.from(
    { length: Math.max(1, ...(days ?? []).map((d) => d.week)) },
    (_, i) => i + 1,
  );
  const isPlanned = (d: PickDay) =>
    planned && program.id === planned.programId && dayKey(d) === dayKey(planned);
  const picked = days?.find((d) => dayKey(d) === selected);
  const inWeek = days?.filter((d) => d.week === week) ?? [];
  return (
    <>
      <ScrollView contentContainerStyle={content}>
        <ListRow
          icon={programIcon(program)}
          title={program.name}
          value="Change"
          onPress={() => choose(undefined)}
        />
        {!days ? (
          <Loading />
        ) : error ? (
          <EmptyState icon="alert-circle-outline" title="Plan not available" message={error} />
        ) : days.length === 0 ? (
          <EmptyState
            icon="cloud-offline-outline"
            title="No days to show"
            message="Connect to your server to load this program."
          />
        ) : (
          <>
            {weeks.length > 1 && (
              <ChipStrip selected={week - 1}>
                {weeks.map((w) => (
                  <Chip
                    key={w}
                    label={`Week ${w}`}
                    selected={w === week}
                    onPress={() => setWeek(w)}
                  />
                ))}
              </ChipStrip>
            )}
            {inWeek.length === 0 ? (
              <Text variant="caption" color="textMuted">
                No days in week {week}
              </Text>
            ) : (
              <ListCard>
                {inWeek.map((d) => (
                  <ListRow
                    key={dayKey(d)}
                    icon={dayKey(d) === selected ? "radio-button-on" : "radio-button-off"}
                    title={d.day ? `${d.day}. ${d.name}` : d.name}
                    // In the subtitle, not as a value: a value squeezes the day's name.
                    subtitle={[isPlanned(d) && "Planned for this date", d.summary]
                      .filter(Boolean)
                      .join(" · ")}
                    onPress={() => setSelected(dayKey(d))}
                  />
                ))}
              </ListCard>
            )}
          </>
        )}
      </ScrollView>
      <SheetFooter testID="day-picker-footer">
        <Button
          title="Create"
          icon="add"
          disabled={!picked}
          onPress={() => {
            picked?.create();
            onClose();
          }}
        />
      </SheetFooter>
    </>
  );
}
