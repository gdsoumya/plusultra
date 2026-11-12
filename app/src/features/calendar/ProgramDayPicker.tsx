import { useEffect, useRef, useState } from "react";
import { ScrollView } from "react-native";

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
  space,
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
  loadDays(program: ProgramSummary): Promise<PickDay[]>;
  // The calendar's own day for the date: preselected, never applied by itself.
  planned?: Planned;
};

const dayKey = (d: Pick<PickDay, "week" | "day" | "kind">) => `${d.week}:${d.day}:${d.kind}`;
const programIcon = (p: ProgramSummary) =>
  p.kind === "meal" ? "restaurant-outline" : "barbell-outline";

// Program, then week chips and day rows, then Create. The body mounts per opening so its state
// starts from the planned day each time.
export function ProgramDayPicker({ open, onClose, title, ...body }: Props) {
  return (
    <Sheet open={open} onClose={onClose} title={title} snapPoints={[0.9]} testID="day-picker">
      {open && <Body {...body} onClose={onClose} />}
    </Sheet>
  );
}

function Body({ programs, loadDays, planned, onClose }: Omit<Props, "open" | "title">) {
  const [program, setProgram] = useState(() => programs.find((p) => p.id === planned?.programId));
  const [days, setDays] = useState<PickDay[]>();
  const [week, setWeek] = useState(planned?.week ?? 1);
  const [selected, setSelected] = useState(planned && dayKey(planned));
  const load = useRef(loadDays);
  useEffect(() => {
    load.current = loadDays;
  });
  useEffect(() => {
    if (!program) return;
    let stale = false;
    load.current(program).then(
      (all) => {
        // An anchored program repeats a day per date; each is listed once.
        const seen = new Set<string>();
        if (!stale) setDays(all.filter((d) => !seen.has(dayKey(d)) && !!seen.add(dayKey(d))));
      },
      () => !stale && setDays([]),
    );
    return () => {
      stale = true;
    };
  }, [program]);

  const choose = (p?: ProgramSummary) => {
    setProgram(p);
    setDays(undefined);
    setWeek(1);
    setSelected(undefined);
  };
  const content = { gap: space[3], paddingBottom: space[6] };

  if (!program) {
    return (
      <ScrollView contentContainerStyle={content}>
        {programs.length === 0 ? (
          <EmptyState icon="barbell-outline" title="No programs" message="Write a program first." />
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
            <ListCard>
              {days
                .filter((d) => d.week === week)
                .map((d) => (
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
          </>
        )}
      </ScrollView>
      <Button
        title="Create"
        icon="add"
        disabled={!picked}
        onPress={() => {
          picked?.create();
          onClose();
        }}
      />
    </>
  );
}
