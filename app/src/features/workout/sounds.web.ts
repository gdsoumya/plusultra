import type { Beep } from "./sounds";

export type { Beep };

// The same two signals as the WAV assets, synthesised: 880 Hz once to start, 660 Hz three times
// to end. Tones in seconds.
const tones: Record<Beep, [hz: number, seconds: number][]> = {
  start: [[880, 0.12]],
  end: [
    [660, 0.12],
    [660, 0.12],
    [660, 0.12],
  ],
};
const gap = 0.08;

let context: AudioContext | undefined;

// Browsers only allow audio after a user gesture; the first beep follows a tap on play, so the
// context opens then. Where there is no Web Audio the bar stays silent.
export function beep(kind: Beep) {
  try {
    context ??= new AudioContext();
    let at = context.currentTime;
    for (const [hz, seconds] of tones[kind]) {
      const osc = context.createOscillator();
      const gain = context.createGain();
      osc.frequency.value = hz;
      gain.gain.value = 0.3;
      osc.connect(gain).connect(context.destination);
      osc.start(at);
      osc.stop(at + seconds);
      at += seconds + gap;
    }
  } catch {}
}
