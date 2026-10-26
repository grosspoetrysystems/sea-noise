# Gate Checklist (TDD-Style)

Use this checklist at wave gates and phase transitions.
Mark each assertion as `pass` or `fail` and attach concrete evidence.

Companion eval matrix:
- `.agents/EVALS.md`

## Metadata

- Date:
- Plan ID:
- Phase:
- Wave (if applicable):
- Reviewer(s):
- Skill attachments:
  - `git-commit-workflow` (required)
- Plan context snapshot (`Plan/Phase/Wave/Track` + objective):
- Product benefits summary:
- Product tradeoffs/risks summary:
- Planning artifact links reviewed (`PLAN_INDEX*`, kickoff, lookahead, source plan/decomposition):
- Related kickoff:
- Related lookahead:
- Related closeout (if applicable):

---

## A) Wave-End Gate (Tactical)

### Given
- [ ] Active kickoff exists for this phase.
- [ ] Wave tracks are defined with owners.
- [ ] Track isolation plan is explicit (`git worktree` per track or bounded exception rationale).
- [ ] Current blocker list is available.
- [ ] Ceremony artifact explicitly attaches `git-commit-workflow`.
- [ ] Plan context + product benefits/tradeoffs are summarized with planning-doc links.

### When
- [ ] Wave-end gate review executed.

### Then (must/must not)
- [ ] Planned vs delivered scope alignment was checked (`must`).
- [ ] Sequencing and blocker coordination actions were recorded (`must`).
- [ ] Tactical correction actions were bounded (`must`).
- [ ] Track-level integration boundaries and merge-order constraints were reviewed (`must`).
- [ ] Immediate re-council was avoided for first-wave drift detection (`must`).
- [ ] Defer list (if any) has rationale and owner (`must`).
- [ ] Product benefit and tradeoff implications are explicitly documented (`must`).

### Evidence
- Gate notes path:
- Worktree/branch map path:
- Blocker table path:
- Defer log path:
- Related commits:

Wave gate result:
- [ ] pass
- [ ] fail

---

## B) Phase-End Closeout (Structural)

### Given
- [ ] All phase waves have gate outcomes recorded.
- [ ] Deferred items and unresolved unknowns are listed.
- [ ] Ceremony artifact explicitly attaches `git-commit-workflow`.
- [ ] Plan context + product benefits/tradeoffs are summarized with planning-doc links.

### When
- [ ] Phase closeout + transition review executed.

### Then (must/must not)
- [ ] Phase completion/defer status is documented (`must`).
- [ ] Drift is classified as resolved or sustained (`must`).
- [ ] Active phase has kickoff artifact (`must`).
- [ ] Next horizon has lookahead artifact (`must`).
- [ ] Transition rebaselined prior lookahead into next kickoff (`must`).
- [ ] Following lookahead was created/updated (`must`).
- [ ] Plan index has a single active phase in `Present` (`must`).
- [ ] No phase appears in multiple index sections (`must`).
- [ ] Active phase is not listed under `Future` as `NEXT` (`must not`).
- [ ] `Future` begins with the next not-yet-active phase (`must`).
- [ ] Product-level benefits achieved and tradeoffs accepted are documented (`must`).

### Evidence
- Closeout doc path:
- Updated kickoff path:
- Updated/new lookahead path:
- Updated plan index path:

Phase closeout result:
- [ ] pass
- [ ] fail

---

## C) Re-Council Threshold Check (Rare)

Run only when phase-end review indicates sustained drift or planning-document exhaustion.

### Given
- [ ] Sustained drift across phase, or planning document no longer coherent/fit.

### When
- [ ] Re-council threshold review executed.

### Then (must/must not)
- [ ] Re-council rationale is explicitly documented (`must`).
- [ ] Evidence shows wave/phase controls were insufficient (`must`).
- [ ] Fresh council deliberation reference is captured (`must`, if triggered).
- [ ] Kickoff was rebaselined from deliberation output (`must`, if triggered).
- [ ] Boundary note confirms local workflow only, not Tempor `src` feature (`must`).

### Evidence
- Threshold decision notes path:
- Deliberation artifact refs:
- Rebaselined kickoff path:

Re-council decision:
- [ ] not needed
- [ ] triggered

---

## D) Final Gate Decision

- [ ] Approve next wave
- [ ] Approve phase transition
- [ ] Require bounded remediation
- [ ] Escalate to re-council threshold review

Reviewer summary:
