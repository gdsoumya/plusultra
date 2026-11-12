import { Text } from "./Text";

// A failure under a form or action; announced to screen readers when it appears.
export const ErrorLine = ({ message }: { message?: string }) =>
  message ? (
    <Text color="danger" accessibilityLiveRegion="polite">
      {message}
    </Text>
  ) : null;
