import { readFile } from "node:fs/promises";

const expectedCommands = [
  "AdminPassword",
  "Shutdown",
  "DoExit",
  "Broadcast",
  "KickPlayer",
  "BanPlayer",
  "TeleportToPlayer",
  "TeleportToMe",
  "ShowPlayers",
  "Info",
  "Save",
  "UnBanPlayer",
  "ToggleSpectate",
].sort();

const sourceUrl = new URL(
  "../web/src/features/operations/server-controls.tsx",
  import.meta.url,
);
const source = await readFile(sourceUrl, "utf8");
const commands = [...source.matchAll(/command:\s*"([A-Za-z]+)(?:\s|"|$)/g)]
  .map((match) => match[1])
  .sort();

if (new Set(commands).size !== commands.length) {
  throw new Error(`RCON templates contain duplicates: ${commands.join(", ")}`);
}
if (
  commands.length !== expectedCommands.length ||
  commands.some((command, index) => command !== expectedCommands[index])
) {
  throw new Error(
    `RCON templates do not match the 13 Palworld 1.0.0 official commands. ` +
      `Expected ${expectedCommands.join(", ")}; found ${commands.join(", ")}`,
  );
}

console.log(
  `Verified ${commands.length}/13 official Palworld 1.0.0 RCON command templates.`,
);
