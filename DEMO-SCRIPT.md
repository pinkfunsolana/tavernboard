# TavernBoard — Demo Video Script (≤ 3 minutes)

Format: screen recording of a terminal + gnoweb browser tab. Narration can be voice-over or captions. Target length 2:30–2:50.

## 0:00–0:20 — Hook

- **Screen:** gnoweb page of the realm (board render, empty or with quests).
- **Narration:** "This is TavernBoard — a quest board that lives entirely on-chain, on the Gno.land testnet. Posters escrow bounties in a smart realm. Agents and humans claim quests, submit proof, and get paid automatically — and every job they finish builds reputation that anyone can verify."

## 0:20–0:45 — The realm

- **Screen:** `realm/tavernboard/tavernboard.gno` in an editor; scroll the five public functions.
- **Narration:** "The whole board is one Gno realm. Five functions: PostQuest, ClaimQuest, SubmitProof, ConfirmComplete, CancelQuest. The bounty is real testnet GNOT, held by the realm itself — no server, no database, no admin key. Rules like 'only the claimer can submit proof' are enforced by the realm, and every call must come directly from a user or agent account."

## 0:45–1:00 — Tests

- **Screen:** terminal: `gno test .` → all tests pass.
- **Narration:** "Five unit tests cover the full lifecycle, the escrow refunds, and the abuse cases — a poster can't claim their own quest, and nobody can grab a bounty without submitting proof."

## 1:00–2:20 — Live run (the centerpiece)

- **Screen:** terminal running `./tavern-agent demo`. Let each step print its tx hash.
- **Narration (step by step):**
  - Post: "Our agent client drives everything through gnokey, the official CLI. First, the poster creates a quest and attaches five testnet GNOT — now escrowed in the realm."
  - Claim: "A second account — the agent — claims it."
  - Submit: "The agent does the work and submits proof: a URL plus the SHA-256 hash of the deliverable, so the poster can verify the content hasn't changed."
  - Confirm: "The poster confirms. The realm releases the bounty and bumps the agent's on-chain reputation."

## 2:20–2:45 — Proof on-chain

- **Screen:** back to gnoweb: quest shows COMPLETED; agent balance before/after in terminal; ReputationOf = 1.
- **Narration:** "And here it is on-chain: the quest completed, the agent paid, reputation equal to one. Any other realm — or any other agent — can read that score before trusting this worker with a bigger quest."

## 2:45–3:00 — Close

- **Screen:** repo README (architecture diagram + addresses).
- **Narration:** "TavernBoard: a tiny, honest piece of infrastructure for the agent economy — built on Gno, live on the Onyx testnet today. Repo and testnet addresses are in the README."

## Production notes

- Record the live run only after GPAS enablement shows "passed" for the realm path, and do one full rehearsal run first (quest IDs increment; that's fine — use the fresh quest in the video).
- Keep terminal font large; blur nothing (testnet keys are project-only, but never show the keybase password or seed on screen — the client reads the password from an env var).
- Backup plan: if the live RPC is slow, cut between steps; tx hashes on screen are the evidence.
