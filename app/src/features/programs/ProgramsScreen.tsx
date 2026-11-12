import { router, useFocusEffect } from "expo-router";
import { useReducer } from "react";
import { View } from "react-native";

import { programsRepo } from "../../data/repositories/programs";
import { useStore } from "../../data/StoreProvider";
import { useRefresh } from "../../data/useRefresh";
import { useSync } from "../../data/useSync";
import { Button, EmptyState, ListCard, ListRow, Screen, space, Text, useWide } from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { kindLabel, modeLabel, sections } from "./summary";

const openEditor = () => router.push("/program/edit");

export default function ProgramsScreen() {
  const store = useStore();
  // A finished sync re-renders with what it stored, such as a program made over MCP; focus and
  // pull-to-refresh read the store again too.
  useSync();
  const [, reload] = useReducer((n: number) => n + 1, 0);
  useFocusEffect(reload);
  const refresh = useRefresh(reload);
  const programs = programsRepo(store).all();
  const wide = useWide();

  return (
    <Screen
      title="Programs"
      testID="programs-screen"
      right={
        <>
          <SyncBadge />
          <Button title="New" icon="add" variant="secondary" compact={!wide} onPress={openEditor} />
        </>
      }
      refresh={refresh}
    >
      {programs.length === 0 ? (
        <EmptyState
          icon="barbell-outline"
          title="No programs yet"
          message="Write a program as a script, check it, then activate it."
          action={{ title: "Write a program", onPress: openEditor }}
        />
      ) : (
        sections(programs).map((section) => (
          <View key={section.title} style={{ gap: space[2] }}>
            <Text variant="heading">{section.title}</Text>
            <ListCard>
              {section.programs.map((p) => (
                <ListRow
                  key={p.id}
                  icon={p.kind === "meal" ? "restaurant-outline" : "barbell-outline"}
                  title={p.name}
                  subtitle={`${kindLabel[p.kind]}, ${modeLabel(p)}`}
                  value={`${p.weeks} wk`}
                  onPress={() => router.push(`/program/${p.id}`)}
                />
              ))}
            </ListCard>
          </View>
        ))
      )}
    </Screen>
  );
}
