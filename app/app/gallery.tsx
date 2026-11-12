import { Redirect } from "expo-router";

import GalleryScreen from "../src/features/gallery/GalleryScreen";

export default function GalleryRoute() {
  return __DEV__ ? <GalleryScreen /> : <Redirect href="/" />;
}
