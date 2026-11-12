import { useLocalSearchParams } from "expo-router";

import { useApi } from "../../../../src/api/useApi";
import { useSession } from "../../../../src/data/session";
import { useStore } from "../../../../src/data/StoreProvider";
import MealsScreen from "../../../../src/features/meals/MealsScreen";

export default function MealsRoute() {
  const { date } = useLocalSearchParams<{ date: string }>();
  const store = useStore();
  const api = useApi();
  const { user } = useSession();
  return <MealsScreen date={date} store={store} api={api} userId={user?.id ?? ""} />;
}
