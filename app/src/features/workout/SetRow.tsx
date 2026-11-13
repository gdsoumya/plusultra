import type { ReactNode } from "react";
import { Pressable, View, type ViewStyle } from "react-native";

import { clock } from "../../domain/dates";
import { isHold, targetLabel } from "../../domain/sets";
import type { SetEntry, Unit } from "../../domain/types";
import { formatWeight } from "../../domain/units";
import {
  Icon,
  Text,
  control,
  doneOpacity,
  iconSize,
  minTouch,
  pressedOpacity,
  radius,
  space,
  stroke,
  tabular,
  useTheme,
} from "../../ui";
import { SetEditor, type Values } from "./SetEditor";
import { setGroup } from "./setGroup";
import { shown } from "./workoutEdits";

type Props = {
  set: SetEntry;
  /** 1-based position shown to the user: "Set 2" or "Warmup 2". */
  number: number;
  unit: Unit;
  /** Left and right reps are logged separately. */
  unilateral: boolean;
  /** A finished workout's set: shown as logged, nothing to press. */
  /** The next set to do; drawn with an accent border. */
  active: boolean;
  /** This set's work timer is running; the control stops it. */
  running: boolean;
  /** Shows the inline reps, weight and RPE editor under the row. */
  editing: boolean;
  onDone: () => void;
  onEdit: () => void;
  onRemove: () => void;
  onChange: (values: Values) => void;
  testID: string;
};

// One number in the bubble: the target that completing saves; AMRAP appends its plus (P2-R46).
const plannedReps = ({ planned: p }: SetEntry) => `${p.reps}${p.amrap ? "+" : ""}`;
const isRange = ({ planned: p }: SetEntry) => p.minReps > 0 && p.minReps !== p.reps;

const isTimed = ({ planned: p }: SetEntry) => p.work > 0;

// A planned hold stays one until the user logs reps against it.
const hold = (set: SetEntry) => isHold({ ...set.planned, reps: shown(set).reps });

// The plan's words (AMRAP, ask) until the user edits or completes the set. A timed set shows its
// time in place of reps; bodyweight work has no weight to show.
function labels(set: SetEntry, unit: Unit, unilateral: boolean) {
  const fromPlan = !set.edited && !set.completed;
  const { reps, repsLeft, weight } = shown(set, unilateral);
  const time = fromPlan ? targetLabel(set.planned) : clock(set.workTime ?? set.planned.work);
  return {
    reps: hold(set) ? time : fromPlan ? plannedReps(set) : `${reps}`,
    repsLeft: fromPlan ? plannedReps(set) : `${repsLeft}`,
    weight: fromPlan && set.planned.ask ? "?" : weight.value ? formatWeight(weight, unit) : "",
  };
}

// What the plan asks of the set besides reps and weight, visible before anything is pressed.
function caption(set: SetEntry) {
  const p = set.planned;
  const fromPlan = !set.edited && !set.completed;
  return [
    isRange(set) ? `${p.minReps}-${p.reps} reps · aim ${p.reps}` : "",
    p.work ? `work ${clock(p.work)}${p.auto ? " · auto" : ""}` : "",
    p.rest ? `rest ${clock(p.rest)}` : "",
    set.rpe ? `RPE ${set.rpe}` : p.rpe ? `RPE ${p.rpe}` : "",
    p.amrap ? "AMRAP" : "",
    fromPlan && p.ask ? "ask" : "",
  ].filter(Boolean);
}

const marker = 3;

const bubble: ViewStyle = {
  minHeight: control.sm,
  minWidth: control.sm,
  paddingHorizontal: space[2],
  borderRadius: radius.control,
  alignItems: "center",
  justifyContent: "center",
};

export function SetRow(p: Props) {
  const { set, number, unit, unilateral, active, running, editing, testID } = p;
  const { colors } = useTheme();
  const title = `${set.planned.warmup ? "Warmup" : "Set"} ${number}`;
  const text = labels(set, unit, unilateral);
  const group = setGroup(set.planned);
  const quiet = set.completed;
  const timed = isTimed(set);
  const names = {
    repsLeft: "Left reps",
    reps: hold(set) ? "Time" : unilateral ? "Right reps" : "Reps",
    weight: "Weight",
  };
  const sides = { repsLeft: "L ", reps: unilateral ? "R " : "", weight: "" };
  const logged = unilateral
    ? `L ${text.repsLeft} R ${text.reps}`
    : timed
      ? text.reps
      : `${set.reps} reps`;
  const doneLabel = set.completed
    ? `${title} done, ${logged}, tap to undo`
    : running
      ? `Stop ${title.toLowerCase()}`
      : `${timed ? "Start" : "Complete"} ${title.toLowerCase()}${active ? ", next up" : ""}`;

  const value = (which: keyof typeof names) => (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`${names[which]} ${text[which]}`}
      accessibilityHint="Edit"
      onPress={p.onEdit}
      testID={`${testID}-${which}`}
      style={({ pressed }) => [
        bubble,
        {
          backgroundColor: colors.surfaceRaised,
          borderWidth: stroke.control,
          borderColor: editing ? colors.accent : colors.border,
          opacity: pressed ? pressedOpacity : 1,
        },
      ]}
    >
      <Text variant="label" numberOfLines={1} style={tabular}>
        {sides[which] + text[which]}
      </Text>
    </Pressable>
  );
  const iconButton = (
    label: string,
    onPress: () => void,
    children: ReactNode,
    role: "checkbox" | "button",
    id: string,
  ) => (
    <Pressable
      accessibilityRole={role}
      accessibilityLabel={label}
      accessibilityState={role === "checkbox" ? { checked: set.completed } : undefined}
      aria-checked={role === "checkbox" ? set.completed : undefined}
      onPress={onPress}
      // React Native Web presses on Space only for role="button"; a checkbox takes it too.
      onKeyDown={
        role === "checkbox"
          ? (e) => {
              if (e.nativeEvent.key !== " ") return;
              e.preventDefault();
              onPress();
            }
          : undefined
      }
      testID={`${testID}-${id}`}
      style={({ pressed }) => ({
        width: minTouch,
        height: minTouch,
        alignItems: "center",
        justifyContent: "center",
        opacity: pressed ? pressedOpacity : 1,
      })}
    >
      {children}
    </Pressable>
  );

  return (
    <View style={{ gap: space[2] }} testID={testID}>
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: space[2],
          paddingVertical: space[1],
          paddingLeft: space[2],
          borderRadius: radius.control,
          borderWidth: stroke.control,
          borderColor: active ? colors.accent : "transparent",
          backgroundColor: active ? colors.accentSoft : "transparent",
        }}
      >
        <View
          testID={`${testID}-group`}
          style={{
            width: marker,
            alignSelf: "stretch",
            borderRadius: marker / 2,
            backgroundColor: colors[group.color],
            opacity: quiet ? doneOpacity : 1,
          }}
        />
        <View
          testID={`${testID}-body`}
          style={{ flex: 1, gap: space[1], opacity: quiet ? doneOpacity : 1 }}
        >
          <Text variant="caption" numberOfLines={1} testID={`${testID}-caption`}>
            <Text variant="caption" color={group.color}>
              {group.label}
            </Text>
            {caption(set).map((part) => ` · ${part}`)}
          </Text>
          <View
            testID={`${testID}-values`}
            style={{ flexDirection: "row", alignItems: "center", gap: space[2] }}
          >
            {unilateral && value("repsLeft")}
            {value("reps")}
            {text.weight ? (
              <>
                <Text variant="caption">x</Text>
                {value("weight")}
              </>
            ) : null}
          </View>
        </View>
        {iconButton(
          doneLabel,
          p.onDone,
          <Icon
            name={
              set.completed
                ? "checkmark-circle"
                : running
                  ? "stop-circle"
                  : timed
                    ? "play-circle"
                    : "checkmark-circle-outline"
            }
            size={iconSize.lg}
            color={set.completed ? "success" : "accent"}
          />,
          "checkbox",
          "done",
        )}
        {iconButton(
          `Remove ${title.toLowerCase()}`,
          p.onRemove,
          <Icon name="close" color="textMuted" />,
          "button",
          "remove",
        )}
      </View>
      {editing && (
        // Swallows taps, so the pane's tap-to-close never fires from inside the editor.
        <Pressable accessible={false} onPress={() => {}} testID={`${testID}-editor`}>
          <SetEditor set={set} unit={unit} unilateral={unilateral} onChange={p.onChange} />
        </Pressable>
      )}
    </View>
  );
}
