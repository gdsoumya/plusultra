// The muscle taxonomy (R83): groups in display order, each with its regions. Mirrors
// internal/catalog/muscles.go; change both together (muscles.test.ts pins them).
export const muscleRegions: Record<string, readonly string[]> = {
  chest: ["upper chest", "mid chest", "lower chest"],
  shoulders: ["front delts", "side delts", "rear delts"],
  "rotator cuff": [],
  biceps: ["biceps long head", "biceps short head", "brachialis"],
  triceps: ["triceps long head", "triceps lateral head", "triceps medial head"],
  forearms: ["forearm flexors", "forearm extensors", "brachioradialis"],
  abdominals: ["upper abs", "lower abs"],
  obliques: [],
  serratus: [],
  lats: [],
  "middle back": ["rhomboids"],
  "lower back": ["erectors"],
  traps: ["upper traps", "mid traps", "lower traps"],
  neck: [],
  glutes: ["glute max", "glute med"],
  "hip flexors": [],
  quadriceps: ["rectus femoris", "vastus"],
  hamstrings: [],
  adductors: [],
  abductors: [],
  calves: ["gastrocnemius", "soleus", "tibialis"],
};

export const muscleGroups = Object.keys(muscleRegions);

// The group a region belongs to; a group (or an unknown name) is its own.
export const groupOf = (muscle: string) =>
  muscleGroups.find((g) => muscleRegions[g].includes(muscle)) ?? muscle;
