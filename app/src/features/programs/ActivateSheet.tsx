import { Code, ConnectError } from "@connectrpc/connect";
import { useState } from "react";
import { View } from "react-native";

import { errorMessage } from "../../api/errors";
import { useApi } from "../../api/useApi";
import { programsRepo } from "../../data/repositories/programs";
import { useStore } from "../../data/StoreProvider";
import { isDate, nextMonday, today, weekday, weekdayName } from "../../domain/dates";
import type { ProgramSummary } from "../../domain/types";
import type { Program } from "../../gen/plusultra/v1/program_pb";
import { Button, Chip, ErrorLine, Field, Sheet, space, Text } from "../../ui";
import { remember } from "./programStore";

type Props = {
  program: ProgramSummary;
  open: boolean;
  onClose: () => void;
  onActivated: (program: Program) => void;
};

export function ActivateSheet({ program, open, onClose, onActivated }: Props) {
  const api = useApi();
  const store = useStore();
  const [anchored, setAnchored] = useState(true);
  const [date, setDate] = useState(() => nextMonday(today()));
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string>();
  // The active program of the same kind that the server refused to run alongside.
  const [blocker, setBlocker] = useState<ProgramSummary>();

  const close = () => {
    setMessage(undefined);
    setBlocker(undefined);
    onClose();
  };

  const validDate = isDate(date);

  const start = async () => {
    setBusy(true);
    setMessage(undefined);
    setBlocker(undefined);
    try {
      const { program: active } = await api.programs.activateProgram({
        id: program.id,
        startDate: anchored ? date : "",
      });
      onActivated(active!);
    } catch (e) {
      setMessage(errorMessage(e));
      if (e instanceof ConnectError && e.code === Code.FailedPrecondition) {
        setBlocker(
          programsRepo(store)
            .all()
            .find((p) => p.kind === program.kind && p.status === "active" && p.id !== program.id),
        );
      }
    } finally {
      setBusy(false);
    }
  };

  const finishBlocker = async () => {
    setBusy(true);
    try {
      const { program: finished } = await api.programs.finishProgram({ id: blocker!.id });
      remember(store, finished!);
    } catch (e) {
      setMessage(errorMessage(e));
      setBusy(false);
      return;
    }
    await start();
  };

  return (
    <Sheet
      open={open}
      onClose={close}
      title={`Activate ${program.name}`}
      snapPoints={[0.6]}
      footer={
        <>
          {blocker && (
            <Button title="Finish it" variant="secondary" loading={busy} onPress={finishBlocker} />
          )}
          <Button
            title="Start program"
            loading={busy}
            disabled={anchored && !validDate}
            onPress={start}
          />
        </>
      }
    >
      <View style={{ flexDirection: "row", gap: space[2] }}>
        <Chip label="Anchored to dates" selected={anchored} onPress={() => setAnchored(true)} />
        <Chip label="Floating" selected={!anchored} onPress={() => setAnchored(false)} />
      </View>
      {anchored ? (
        <Field
          label="Start date"
          value={date}
          onChangeText={setDate}
          placeholder="YYYY-MM-DD"
          autoCapitalize="none"
          autoCorrect={false}
          helper={
            validDate
              ? `Starts on a ${weekdayName(date)}, at week 1 day ${weekday(date)}`
              : undefined
          }
          error={validDate ? undefined : "Enter a date as YYYY-MM-DD"}
        />
      ) : (
        <Text color="textMuted">
          Days run in order from week 1 day 1. Finishing a workout moves on to the next day.
        </Text>
      )}
      <ErrorLine message={message} />
    </Sheet>
  );
}
