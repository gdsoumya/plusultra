import { useEffect, useRef, useState } from "react";

import { cancelTimerEnd, type Notice, scheduleTimerEnd } from "./notifications";

type Running<M> = { meta: M; startedAt: number; endsAt?: number; notice?: Notice };
type Scheduled = Promise<string | undefined>;
/** A timer that follows this one when it ends, e.g. the rest after a timed set. */
export type Next = { seconds: number; notice: Notice };

const cancel = (id?: Scheduled) =>
  id?.then((n) => {
    if (n) void cancelTimerEnd(n);
  });

// One timer at a time (rest or a timed set). Keeps the end time, not a tick count, so the
// countdown is right after backgrounding. Without `seconds` it counts up until stopped.
// No interval runs in the background, so onEnd gets the time the timer really ended and the next
// timer can start from it; the next one's notification is scheduled up front for the same reason.
export function useTimer<M>(onEnd: (meta: M, endedAt: number) => void) {
  const [timer, setTimer] = useState<Running<M>>();
  const [now, setNow] = useState(() => Date.now());
  const notification = useRef<Scheduled>(undefined);
  const queued = useRef<{ endsAt: number; id?: Scheduled }>(undefined);
  const ended = useRef(onEnd);
  useEffect(() => {
    ended.current = onEnd;
  });

  useEffect(() => {
    if (!timer) return;
    const tick = setInterval(() => {
      const t = Date.now();
      setNow(t);
      if (timer.endsAt && t >= timer.endsAt) {
        setTimer(undefined);
        notification.current = undefined;
        ended.current(timer.meta, timer.endsAt);
      }
    }, 1000);
    return () => clearInterval(tick);
  }, [timer]);

  const seconds = (ms: number) => Math.max(0, Math.round(ms / 1000));
  const schedule = (endsAt: number, notice: Notice) =>
    endsAt > Date.now() ? scheduleTimerEnd(endsAt, notice) : undefined;

  const cancelNotice = () => {
    cancel(notification.current);
    notification.current = undefined;
  };

  const stop = () => {
    setTimer(undefined);
    cancelNotice();
    cancel(queued.current?.id);
    queued.current = undefined;
    return timer ? Math.max(1, seconds(Date.now() - timer.startedAt)) : 0;
  };

  return {
    running: timer?.meta,
    remaining: timer?.endsAt ? Math.max(0, Math.ceil((timer.endsAt - now) / 1000)) : 0,
    elapsed: timer ? seconds(now - timer.startedAt) : 0,
    /**
     * Starts now, or at `from` when following a timer that ended while the app was away. A
     * countdown with `next` also schedules the notification of the timer that follows it.
     */
    start(meta: M, seconds?: number, notice?: Notice, from = Date.now(), next?: Next) {
      const ahead = queued.current;
      queued.current = undefined;
      stop();
      const endsAt = seconds ? from + seconds * 1000 : undefined;
      setNow(Date.now());
      setTimer({ meta, startedAt: from, endsAt, notice });
      if (ahead && ahead.endsAt === endsAt) notification.current = ahead.id;
      else {
        cancel(ahead?.id);
        if (endsAt && notice) notification.current = schedule(endsAt, notice);
      }
      if (endsAt && next) {
        const at = endsAt + next.seconds * 1000;
        queued.current = { endsAt: at, id: schedule(at, next.notice) };
      }
    },
    /** Moves a countdown's end by `delta` seconds, leaving at least one second. */
    adjust(delta: number) {
      if (!timer?.endsAt) return;
      const endsAt = Math.max(Date.now() + 1000, timer.endsAt + delta * 1000);
      setTimer({ ...timer, endsAt });
      cancelNotice();
      if (timer.notice) notification.current = scheduleTimerEnd(endsAt, timer.notice);
    },
    /** Ends the timer early; returns the elapsed seconds, at least one. */
    stop,
  };
}
