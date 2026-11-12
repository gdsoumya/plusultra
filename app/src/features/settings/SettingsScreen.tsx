import { useRouter } from "expo-router";
import { useState } from "react";

import { useSession } from "../../data/session";
import { useStore } from "../../data/StoreProvider";
import { useSync } from "../../data/useSync";
import { formatDateTime } from "../../domain/dates";
import { ConfirmSheet, ListCard, ListRow, Screen, type IconName } from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { SyncSheet } from "../sync/SyncSheet";
import { useCall } from "./useRemote";
import { signOut } from "./useServer";

const pages: { title: string; icon: IconName; href: string }[] = [
  { title: "Account", icon: "person-circle-outline", href: "/settings/account" },
  { title: "Equipment and plates", icon: "barbell-outline", href: "/settings/equipment" },
  { title: "1RMs", icon: "trophy-outline", href: "/settings/maxes" },
  { title: "Catalog", icon: "list-outline", href: "/settings/catalog" },
  { title: "API keys", icon: "key-outline", href: "/settings/api-keys" },
  { title: "Export", icon: "download-outline", href: "/settings/export" },
  { title: "About", icon: "information-circle-outline", href: "/settings/about" },
];

export default function SettingsScreen() {
  const router = useRouter();
  const session = useSession();
  const store = useStore();
  const { lastSyncedAt } = useSync();
  const [sheet, setSheet] = useState<"sync" | "server">();
  const close = () => setSheet(undefined);
  const { call, busy, error } = useCall();
  // The session keeps the address, so the server screen comes up with it prefilled.
  const changeServer = () =>
    call(async () => {
      await signOut(session, store);
      router.replace("/server");
    });
  return (
    <Screen title="Settings" testID="settings-screen" right={<SyncBadge />}>
      <ListCard>
        {pages.map(({ title, icon, href }) => (
          <ListRow
            key={href}
            title={title}
            icon={icon}
            subtitle={title === "Account" ? session.user?.displayName : undefined}
            onPress={() => router.push(href)}
          />
        ))}
      </ListCard>
      <ListCard>
        <ListRow
          title="Sync now"
          icon="sync-outline"
          subtitle={
            lastSyncedAt ? `Last synced ${formatDateTime(new Date(lastSyncedAt))}` : "Never synced"
          }
          onPress={() => setSheet("sync")}
        />
        <ListRow
          title="Change server"
          icon="server-outline"
          subtitle={session.serverUrl}
          onPress={() => setSheet("server")}
        />
      </ListCard>
      <SyncSheet open={sheet === "sync"} onClose={close} />
      <ConfirmSheet
        open={sheet === "server"}
        title="Change server"
        message="You will be signed out and local data for this server cleared."
        action="Change server"
        busy={busy}
        error={error}
        onConfirm={changeServer}
        onClose={close}
      />
    </Screen>
  );
}
