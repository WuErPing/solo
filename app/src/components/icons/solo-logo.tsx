import { Image, type ImageStyle } from "react-native";
import { useUnistyles } from "react-native-unistyles";

interface SoloLogoProps {
  size?: number;
  color?: string;
}

export function SoloLogo({ size = 64, color }: SoloLogoProps) {
  useUnistyles();

  const style: ImageStyle = {
    width: size,
    height: size,
    resizeMode: "contain",
  };

  return (
    <Image
      source={require("../../../assets/images/solo-logo.png")}
      style={style}
      tintColor={color}
    />
  );
}
