import Svg, { Path } from "react-native-svg";

interface KimiIconProps {
  size?: number;
  color?: string;
}

export function KimiIcon({ size = 16, color = "currentColor" }: KimiIconProps) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill={color}>
      <Path d="M5 2L5 22L9 22L9 14L15 22L20 22L13 12L20 2L15 2L9 10L9 2L5 2Z" />
    </Svg>
  );
}
