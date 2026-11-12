import { Children, Fragment, type ReactNode } from "react";
import type { ViewProps } from "react-native";

import { Card } from "./Card";
import { Divider } from "./Divider";

// A Card of ListRows: the rows bring their own padding and a Divider sits between them.
export function ListCard({ children, style, ...rest }: ViewProps & { children: ReactNode }) {
  return (
    <Card style={[{ padding: 0, gap: 0, overflow: "hidden" }, style]} {...rest}>
      {Children.toArray(children).map((child, i) => (
        <Fragment key={i}>
          {i > 0 && <Divider />}
          {child}
        </Fragment>
      ))}
    </Card>
  );
}
