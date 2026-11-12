// Metro turns an imported sound into an asset id that expo-audio plays.
declare module "*.wav" {
  const asset: number;
  export default asset;
}
