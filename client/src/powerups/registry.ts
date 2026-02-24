export interface PowerUpDisplayInfo {
  icon: string;
  label: string;
  description: string;
}

export const POWER_UP_DISPLAY: Record<string, PowerUpDisplayInfo> = {
  chaos: {
    icon: "CHS",
    label: "Chaos",
    description: "Reshuffles the positions of all cards that are not yet matched.",
  },
  clairvoyance: {
    icon: "CLV",
    label: "Clairvoyance",
    description: "Reveals a 3x3 area around the card you choose for 2 seconds, then hides it again.",
  },
  necromancy: {
    icon: "NEC",
    label: "Necromancy",
    description: "Returns all collected tiles back to the board in new random positions.",
  },
  discernment: {
    icon: "DSC",
    label: "Discernment",
    description: "Highlights all tiles that have never been revealed.",
  },
};
