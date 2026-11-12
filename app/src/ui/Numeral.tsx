import { Text, type TextProps } from "./Text";
import { tabular } from "./tokens";

export type NumeralProps = Omit<TextProps, "variant" | "children"> & {
  value: number | string;
  /** `md` is 28, `lg` is 40. */
  size?: "md" | "lg";
};

export function Numeral({ value, size = "md", style, ...rest }: NumeralProps) {
  return (
    <Text
      variant={size === "lg" ? "display" : "title"}
      numberOfLines={1}
      style={[tabular, style]}
      {...rest}
    >
      {value}
    </Text>
  );
}
