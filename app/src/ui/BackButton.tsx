import { useRouter, type Href } from "expo-router";

import { Button } from "./Button";

// The header's way back from a sub-screen; a deep link with no history lands on `fallback`.
// `onPress` replaces the navigation, e.g. to ask about unsaved changes first.
export function BackButton({ fallback = "/", onPress }: { fallback?: Href; onPress?: () => void }) {
  const router = useRouter();
  return (
    <Button
      title="Back"
      icon="chevron-back"
      variant="ghost"
      onPress={onPress ?? (() => (router.canGoBack() ? router.back() : router.replace(fallback)))}
    />
  );
}
