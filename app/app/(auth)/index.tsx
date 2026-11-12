import { Redirect } from "expo-router";

import { useSession } from "../../src/data/session";

// A rejected token goes straight back to login; anything else starts at the server.
export default function AuthIndex() {
  return <Redirect href={useSession().expired ? "/login" : "/server"} />;
}
