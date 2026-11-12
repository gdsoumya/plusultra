import type { ServiceImpl } from "@connectrpc/connect";

import { CheckinService, type Checkin } from "../../gen/plusultra/v1/checkin_pb";
import { fromProto, toProto } from "../convert";
import {
  find,
  inDates,
  invalid,
  newId,
  onDate,
  page,
  remove,
  required,
  save,
  type MockDb,
} from "./db";

export function checkins(db: MockDb): ServiceImpl<typeof CheckinService> {
  // A sent photo replaces the stored one; an empty one keeps it unless clear is set.
  // shortcut: neither the 2 MB cap nor a photo sent together with clearPhoto is rejected as on the
  // server; upgrade if the check-in screen relies on those errors.
  function photoId(sent: Checkin, current: string | undefined, clear: boolean) {
    if (!sent.photo.length) return clear ? undefined : current;
    if (!["image/png", "image/jpeg"].includes(sent.photoContentType)) {
      throw invalid("a photo must be image/png or image/jpeg");
    }
    const id = newId(db, "photo");
    db.photos.set(id, { bytes: sent.photo, contentType: sent.photoContentType });
    return id;
  }

  function store(sent: Checkin, id: string, current: string | undefined, clear: boolean) {
    const { date, weight, bodyFat, measurements } = fromProto.checkin(sent, db.user.id);
    const row = { id, date, weight, bodyFat, measurements, photoId: photoId(sent, current, clear) };
    return { checkin: toProto.checkin(save(db, "checkins", row)) };
  }

  return {
    listCheckins({ dates, cursor, limit, includePhotos }) {
      const { items, ...more } = page(inDates(db, "checkins", dates), cursor, limit);
      return {
        checkins: items.map((c) => {
          const photo = includePhotos && c.photoId ? db.photos.get(c.photoId) : undefined;
          const out = toProto.checkin(c);
          return photo ? { ...out, photo: photo.bytes, photoContentType: photo.contentType } : out;
        }),
        ...more,
      };
    },
    // One check-in per date: a second one replaces the values of the first and keeps its id.
    createCheckin({ checkin }) {
      const sent = required(checkin, "checkin");
      const current = onDate(db, "checkins", sent.date);
      return store(sent, current?.id ?? required(sent.id, "checkin id"), current?.photoId, false);
    },
    updateCheckin({ checkin, clearPhoto }) {
      const sent = required(checkin, "checkin");
      const current = find(db, "checkins", sent.id);
      return store(sent, current.id, current.photoId, clearPhoto);
    },
    deleteCheckin: ({ id }) => remove(db, "checkins", id),
  };
}
