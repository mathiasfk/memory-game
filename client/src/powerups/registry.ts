export interface PowerUpDisplayInfo {
  icon: string;
  label: string;
  description: string;
}

export const POWER_UP_DISPLAY: Record<string, PowerUpDisplayInfo> = {
  shuffle: {
    icon: "SHF",
    label: "Shuffle",
    description: "Reshuffles all unmatched cards on the board.",
  },
};
