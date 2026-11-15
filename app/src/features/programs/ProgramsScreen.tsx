import { router, useFocusEffect } from "expo-router";
import { useEffect, useReducer, useState } from "react";
import { View } from "react-native";

import { errorMessage } from "../../api/errors";
import { useApi } from "../../api/useApi";
import { programsRepo } from "../../data/repositories/programs";
import { useStore } from "../../data/StoreProvider";
import { useRefresh } from "../../data/useRefresh";
import { useSync } from "../../data/useSync";
import type { CatalogProgram } from "../../gen/plusultra/v1/program_pb";
import {
  Button,
  Chip,
  ChipStrip,
  EmptyState,
  ErrorLine,
  IconButton,
  ListCard,
  ListRow,
  Screen,
  space,
  Text,
  useWide,
} from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { kindLabel, levelLabel, modeLabel, sections } from "./summary";

const openEditor = () => router.push("/program/edit");

const CATALOG_KEY = "programCatalog";

export default function ProgramsScreen() {
  const api = useApi();
  const store = useStore();
  // A finished sync re-renders with what it stored, such as a program made over MCP; focus and
  // pull-to-refresh read the store again too.
  useSync();
  const [tick, reload] = useReducer((n: number) => n + 1, 0);
  useFocusEffect(reload);
  const refresh = useRefresh(reload);
  const programs = programsRepo(store).all();
  const wide = useWide();
  const [tab, setTab] = useState<"mine" | "catalog">("mine");
  const [catalog, setCatalog] = useState(() => store.getKv<CatalogProgram[]>(CATALOG_KEY) ?? []);
  const [catalogError, setCatalogError] = useState<string>();

  // The cached list shows at once; the server's replaces it when the catalog is in view.
  useEffect(() => {
    if (tab !== "catalog") return;
    api.programs.listCatalog({}).then(
      ({ programs: fresh }) => {
        store.setKv(CATALOG_KEY, fresh);
        setCatalog(fresh);
        setCatalogError(undefined);
      },
      (e) => setCatalogError(errorMessage(e)),
    );
  }, [api, store, tab, tick]);

  const levels = Object.entries(levelLabel)
    .map(([level, title]) => ({ title, programs: catalog.filter((p) => p.level === level) }))
    .filter((l) => l.programs.length > 0);

  return (
    <Screen
      title="Programs"
      testID="programs-screen"
      right={
        <>
          <SyncBadge />
          {wide ? (
            <Button title="New" icon="add" variant="secondary" onPress={openEditor} />
          ) : (
            <IconButton icon="add" label="New" onPress={openEditor} />
          )}
        </>
      }
      refresh={refresh}
    >
      <ChipStrip>
        <Chip label="Mine" selected={tab === "mine"} onPress={() => setTab("mine")} />
        <Chip label="Catalog" selected={tab === "catalog"} onPress={() => setTab("catalog")} />
      </ChipStrip>
      {tab === "catalog" ? (
        <>
          <ErrorLine message={catalogError} />
          {levels.map((level) => (
            <View key={level.title} style={{ gap: space[2] }}>
              <Text variant="heading">{level.title}</Text>
              <ListCard>
                {level.programs.map((p) => (
                  <ListRow
                    key={p.slug}
                    icon="barbell-outline"
                    title={p.name}
                    subtitle={`${p.author} · ${p.daysPerWeek} days a week`}
                    value={`${p.weeks} wk`}
                    onPress={() => router.push(`/catalog/${p.slug}`)}
                  />
                ))}
              </ListCard>
            </View>
          ))}
        </>
      ) : programs.length === 0 ? (
        <EmptyState
          icon="barbell-outline"
          title="No programs yet"
          message="Pick one from the catalog or write your own."
          action={{ title: "Browse the catalog", onPress: () => setTab("catalog") }}
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
