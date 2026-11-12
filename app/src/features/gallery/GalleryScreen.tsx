import { useState } from "react";
import { View } from "react-native";

import {
  Badge,
  Button,
  Card,
  Divider,
  EmptyState,
  ErrorLine,
  Field,
  Icon,
  ListCard,
  ListRow,
  Loading,
  Numeral,
  Screen,
  Text,
} from "../../ui";
import { ChartSample, ChipSample, ProgressSample, SheetSample, TextScaleSample } from "./Samples";
import { row, Section } from "./Section";

const icons = ["barbell", "flame", "timer-outline", "checkmark-circle", "trophy", "body"] as const;
const iconColors = ["text", "accent", "textMuted", "success", "warning", "danger"] as const;

export default function GalleryScreen() {
  const [weight, setWeight] = useState("102.5");

  return (
    <Screen title="Gallery" testID="gallery-screen">
      <Section title="Text">
        <Text variant="title">Title 28</Text>
        <Text variant="heading">Heading 20</Text>
        <Text>Body 16, the default for running text.</Text>
        <Text variant="label">Label 14</Text>
        <Text variant="caption">Caption 13</Text>
        <Text variant="mono">mono 14: squat(5x5, 80kg)</Text>
      </Section>
      <Section title="Numeral">
        <View style={row}>
          <Numeral value={102.5} size="lg" />
          <Numeral value="5 x 3" color="accent" />
        </View>
      </Section>
      <Section title="Button">
        <Button title="Start workout" icon="play" />
        <Button title="Skip today" variant="secondary" />
        <Button title="View details" variant="ghost" />
        <Button title="Delete program" variant="danger" icon="trash-outline" />
        <View style={row}>
          <Button title="Saving" loading />
          <Button title="Disabled" disabled />
        </View>
      </Section>
      <ChipSample />
      <Section title="Badge">
        <View style={row}>
          <Badge label="Offline" icon="cloud-offline-outline" tone="warning" />
          <Badge label="Sync failed" icon="alert-circle-outline" tone="danger" />
          <Badge label="Modified" icon="pencil" />
          <Badge label="On target" icon="checkmark-circle" tone="accent" />
        </View>
      </Section>
      <Section title="Field">
        <Field
          label="Server URL"
          placeholder="https://lift.example.com"
          helper="Your self-hosted server"
        />
        <Field
          label="Weight (kg)"
          value={weight}
          onChangeText={setWeight}
          numeric
          error={Number.isNaN(Number(weight)) ? "Enter a number" : undefined}
        />
        <Field label="Reps" value="five" numeric error="Enter a whole number" editable={false} />
      </Section>
      <ProgressSample />
      <ChartSample />
      <Section title="ListRow">
        <ListCard>
          <ListRow
            icon="barbell"
            title="Squat"
            subtitle="3 x 5 at 100 kg"
            value="Done"
            onPress={() => {}}
          />
          <ListRow icon="scale-outline" title="Units" value="kg" onPress={() => {}} />
          <ListRow
            title="A very long exercise name that would otherwise push the value away"
            subtitle="Subtitles clamp to two lines, so a long note from the program still ends here"
            value="3 x 5"
            onPress={() => {}}
          />
          <ListRow title="Version" value="1.0.0" />
        </ListCard>
      </Section>
      <Section title="Loading, ErrorLine and Divider">
        <Loading />
        <ErrorLine message="Could not reach the server. Check your connection and try again." />
        <Divider />
      </Section>
      <Section title="Card">
        <Card>
          <Text variant="caption">Week 3, day 2</Text>
          <Text variant="heading">Upper body A</Text>
          <Text color="textMuted">5 exercises, about 55 min</Text>
          <Button title="Start" icon="play" />
        </Card>
      </Section>
      <Section title="Icon">
        <View style={row}>
          {icons.map((name, i) => (
            <Icon key={name} name={name} size={28} color={iconColors[i]} />
          ))}
        </View>
      </Section>
      <Section title="EmptyState">
        <Card>
          <EmptyState
            icon="calendar-outline"
            title="No program yet"
            message="Pick a program to see your plan here."
            action={{ title: "Browse programs", onPress: () => {} }}
          />
        </Card>
      </Section>
      <SheetSample />
      <TextScaleSample />
    </Screen>
  );
}
