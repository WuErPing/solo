import { Image, type ImageStyle } from "react-native";

interface TmuxLogoProps {
  size?: number;
  color?: string;
}

export function TmuxLogo({ size = 8, color }: TmuxLogoProps) {
  const style: ImageStyle = {
    width: size * 3.8,
    height: size,
    resizeMode: "contain",
  };

  return (
    <Image
      source={require("../../../assets/images/tmux-logo.png")}
      style={style}
      tintColor={color}
    />
  );
}
