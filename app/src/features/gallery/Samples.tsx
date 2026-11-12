import { useState } from "react";
import { View } from "react-native";

import { Button, Card, Chip, Field, Numeral, Progress, Sheet, Text, ThemeProvider } from "../../ui";
import { BarChart } from "../../ui/charts/BarChart";
import { LineChart } from "../../ui/charts/LineChart";
import { StatTiles } from "../../ui/charts/StatTiles";
import { row, Section } from "./Section";

const equipment = ["Barbell", "Dumbbell", "Machine", "Cable"];

export function ChipSample() {
  const [selected, setSelected] = useState(["Barbell"]);
  const toggle = (name: string) =>
    setSelected((s) => (s.includes(name) ? s.filter((n) => n !== name) : [...s, name]));
  return (
    <Section title="Chip">
      <View style={row}>
        {equipment.map((name) => (
          <Chip
            key={name}
            label={name}
            selected={selected.includes(name)}
            onPress={() => toggle(name)}
          />
        ))}
      </View>
    </Section>
  );
}

export function ProgressSample() {
  const [progress, setProgress] = useState(0.6);
  return (
    <Section title="Progress">
      <Progress value={progress} />
      <View style={row}>
        <Progress value={progress} variant="ring">
          <Text variant="label">{`${Math.round(progress * 100)}%`}</Text>
        </Progress>
        <Button
          title="Step"
          variant="secondary"
          onPress={() => setProgress((p) => (p >= 1 ? 0 : Math.min(1, p + 0.2)))}
        />
      </View>
    </Section>
  );
}

const weights = [82.4, 82.1, 81.9, 82.2, 81.6, 81.5, 81.2, 81.4, 81.0, 80.8];

export function ChartSample() {
  const points = weights.map((y, x) => ({ x: x * 3, y }));
  const trend = points.map((p, i) => ({
    x: p.x,
    y: weights.slice(Math.max(0, i - 2), i + 1).reduce((a, b) => a + b) / Math.min(i + 1, 3),
  }));
  return (
    <Section title="Charts">
      <Card>
        <StatTiles
          items={[
            { label: "7-day average", value: "80.9" },
            { label: "kg per week", value: "-0.4" },
          ]}
        />
        <LineChart
          width={300}
          points={points}
          trend={trend}
          xDomain={[0, 28]}
          xLabels={["Oct 14", "Nov 11"]}
          format={(v) => v.toFixed(1)}
          legend={["Weigh-ins", "7-day average"]}
          summary="Ten weigh-ins trending down from 82.4 to 80.8 kg."
          table={{ columns: ["Day", "kg"], rows: points.map((p) => [`${p.x}`, `${p.y}`]) }}
        />
      </Card>
      <Card>
        <BarChart
          bars={[
            { label: "Quads", value: 14 },
            { label: "Chest", value: 11 },
            { label: "Back", value: 9 },
            { label: "Hamstrings", value: 6 },
          ]}
          columns={["Muscle", "Sets"]}
          summary="40 working sets this week, most for quads."
        />
      </Card>
    </Section>
  );
}

export function SheetSample() {
  const [open, setOpen] = useState(false);
  const [weight, setWeight] = useState("80");
  const close = () => setOpen(false);
  return (
    <Section title="Sheet">
      <Button
        title="Open sheet"
        variant="secondary"
        icon="chevron-up"
        onPress={() => setOpen(true)}
      />
      <Sheet
        open={open}
        onClose={close}
        title="Log set"
        snapPoints={[0.5, 0.9]}
        testID="gallery-sheet"
      >
        <Field label="Weight (kg)" value={weight} onChangeText={setWeight} numeric />
        <Button title="Save" onPress={close} style={{ marginTop: "auto" }} />
      </Sheet>
    </Section>
  );
}

export function TextScaleSample() {
  return (
    <Section title="Text scale">
      {[1, 1.3, 2].map((fontScale) => (
        <ThemeProvider key={fontScale} fontScale={fontScale}>
          <Card>
            <Text variant="caption">{`${fontScale}x`}</Text>
            <Text variant="heading">Bench press</Text>
            <View style={row}>
              <Numeral value={80} />
              <Text color="textMuted">kg x 5</Text>
            </View>
            <Button title="Log set" icon="checkmark" />
          </Card>
        </ThemeProvider>
      ))}
    </Section>
  );
}
