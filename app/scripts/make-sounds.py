#!/usr/bin/env python3
"""Writes the timer beeps under assets/sounds as 16-bit mono WAV, so the assets are reproducible
and carry no licence: start.wav is one 880 Hz tone, end.wav three 660 Hz tones."""

import math
import struct
import wave
from pathlib import Path

RATE = 44100
GAP = 0.08
FADE = 0.01

SOUNDS = {
    "start": [(880, 0.12)],
    "end": [(660, 0.12)] * 3,
}


def tone(hz: float, seconds: float) -> list[int]:
    n = int(RATE * seconds)
    fade = int(RATE * FADE)
    out = []
    for i in range(n):
        env = min(1.0, i / fade, (n - i) / fade)
        out.append(int(0.5 * env * 32767 * math.sin(2 * math.pi * hz * i / RATE)))
    return out


def main() -> None:
    out = Path(__file__).resolve().parent.parent / "assets" / "sounds"
    out.mkdir(parents=True, exist_ok=True)
    for name, tones in SOUNDS.items():
        samples: list[int] = []
        for i, (hz, seconds) in enumerate(tones):
            if i:
                samples += [0] * int(RATE * GAP)
            samples += tone(hz, seconds)
        with wave.open(str(out / f"{name}.wav"), "wb") as w:
            w.setnchannels(1)
            w.setsampwidth(2)
            w.setframerate(RATE)
            w.writeframes(struct.pack(f"<{len(samples)}h", *samples))


if __name__ == "__main__":
    main()
