/** A host to name the forge of, set by a remote's menu (#921) before opening Settings; the
 * Forges section starts a rule for it and clears it. */
class ForgeAsk {
  host = $state('')
}

export const forgeAsk = new ForgeAsk()
