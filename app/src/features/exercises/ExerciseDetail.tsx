import { useEffect, useState } from "react";
import { Image, View } from "react-native";

import type { Exercise } from "../../gen/plusultra/v1/exercise_pb";
import { useResolvedExercise } from "../../data/exerciseInfo";
import { Badge, Carousel, Chip, ChipStrip, space, Text } from "../../ui";
import { DetailLine } from "../settings/parts";

// One image per page, as tall as its own aspect needs (4:3 until known), so nothing letterboxes.
function Page({ uri, width }: { uri: string; width: number }) {
  const [ratio, setRatio] = useState(0.75);
  useEffect(() => {
    let live = true;
    Image.getSize(uri, (w, h) => live && w > 0 && setRatio(h / w));
    return () => {
      live = false;
    };
  }, [uri]);
  return (
    <Image
      source={{ uri }}
      resizeMode="contain"
      style={{ width, height: Math.round(width * ratio) }}
      testID="exercise-image"
      accessible={false}
    />
  );
}
const image = (uri: string, width: number) => <Page key={uri} uri={uri} width={width} />;

function Chips({ label, items }: { label: string; items: string[] }) {
  return items.length ? (
    <View style={{ gap: space[1] }}>
      <Text variant="caption">{label}</Text>
      <ChipStrip>
        {items.map((item) => (
          <Chip key={item} label={item} />
        ))}
      </ChipStrip>
    </View>
  ) : null;
}

// Everything the catalog knows about an exercise, for the detail screen and the catalog sheet.
export function ExerciseDetail({ exercise }: { exercise: Exercise }) {
  const e = useResolvedExercise(exercise);
  const steps = e.instructions;
  return (
    <>
      {e.images.length > 0 && (
        <Carousel items={e.images} renderItem={image} testID="exercise-images" />
      )}
      {e.custom && (
        // In a row, so the pill keeps its own width in the column.
        <View style={{ flexDirection: "row" }}>
          <Badge label="Custom" icon="person-outline" />
        </View>
      )}
      <DetailLine label="Also known as" value={e.aliases.join(", ")} />
      <Chips label="Primary muscles" items={e.primaryMuscles} />
      <Chips label="Also works" items={e.secondaryMuscles} />
      <DetailLine label="Equipment" value={e.equipment} />
      <DetailLine label="Category" value={e.category} />
      {steps.length > 0 && (
        <View style={{ gap: space[1] }}>
          <Text variant="caption">Instructions</Text>
          {steps.map((step, i) => (
            <Text key={i} selectable>{`${i + 1}. ${step}`}</Text>
          ))}
        </View>
      )}
    </>
  );
}
