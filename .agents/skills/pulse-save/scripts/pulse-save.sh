#!/usr/bin/env bash
set -euo pipefail

PLAN_DIR="${1:-}"
OUT_FILE="${2:-.tempor/PULSE_CHECK.md}"

if [[ -z "${PLAN_DIR}" ]]; then
  for candidate in $(ls -td .tempor/plans/*/*/plan_* 2>/dev/null || true); do
    if ls "${candidate}"/PLAN_INDEX_V*.md >/dev/null 2>&1; then
      PLAN_DIR="${candidate}"
      break
    fi
  done
fi

if [[ -z "${PLAN_DIR}" || ! -d "${PLAN_DIR}" ]]; then
  echo "Unable to resolve plan directory. Pass <plan_dir> explicitly." >&2
  exit 1
fi

INDEX_FILE="$(ls "${PLAN_DIR}"/PLAN_INDEX_V*.md 2>/dev/null | sort -V | tail -n 1 || true)"
if [[ -z "${INDEX_FILE}" || ! -f "${INDEX_FILE}" ]]; then
  echo "Missing PLAN_INDEX_V*.md under ${PLAN_DIR}" >&2
  exit 1
fi

extract_field() {
  local label="$1"
  local file="$2"
  rg -n "^\*\*${label}:\*\*" "${file}" | head -n 1 | sed -E "s/^[0-9]+:\*\*${label}:\*\* //"
}

strip_ticks() {
  echo "$1" | tr -d '\`'
}

extract_active_context_value() {
  local label="$1"
  local file="$2"
  local value
  value="$(rg -n "^- \`${label}\`:" "${file}" | head -n 1 | sed -E "s/^[0-9]+:- \`${label}\`: //" || true)"
  if [[ -n "${value}" ]]; then
    echo "${value}"
    return 0
  fi
  rg -n "^- ${label}:" "${file}" | head -n 1 | sed -E "s/^[0-9]+:- ${label}: //"
}

extract_wave_section_field() {
  local wave="$1"
  local label="$2"
  local file="$3"
  [[ -z "${wave}" ]] && return 0
  awk -v wave="### ${wave}" -v label="- ${label}:" '
    $0 == wave { in_section=1; next }
    in_section && /^### / { exit }
    in_section && index($0, label) == 1 {
      sub(label " ", "")
      print
      exit
    }
  ' "${file}"
}

derive_kickoff_from_lookahead() {
  local lookahead="$1"
  [[ -z "${lookahead}" ]] && return 0
  echo "${lookahead}" | sed 's/lookahead/kickoff/'
}

collect_candidate_targets() {
  local file="$1"
  [[ ! -f "${file}" ]] && return 0
  awk '
    /^Candidate targets:/ { in_targets=1; next }
    in_targets && /^## / { exit }
    in_targets && /^- / { print }
  ' "${file}"
}

compact_status_lines() {
  awk '
    NF == 0 { next }
    {
      status = substr($0, 1, 2);
      path = substr($0, 4);
      n = split(path, parts, "/");
      if (n > 1) {
        dir = parts[1];
        for (i = 2; i < n; i++) {
          dir = dir "/" parts[i];
        }
      } else {
        dir = ".";
      }
      key = status SUBSEP dir;
      if (!(key in seen)) { order[++k] = key; seen[key] = 1; }
      count[key]++;
      if (!(key in firstline)) { firstline[key] = $0; }
    }
    END {
      for (i = 1; i <= k; i++) {
        key = order[i];
        c = count[key];
        if (c == 1) {
          print firstline[key];
        } else {
          split(key, a, SUBSEP);
          print a[1] " " a[2] "/ (" c " files)";
        }
      }
    }
  '
}

ACTIVE_KICKOFF="$(extract_field "Active kickoff" "${INDEX_FILE}" || true)"
ACTIVE_LOOKAHEAD="$(extract_field "Active lookahead" "${INDEX_FILE}" || true)"
ACTIVE_WAVE_KICKOFF="$(extract_field "Active wave kickoff" "${INDEX_FILE}" || true)"
ACTIVE_WAVE_LOOKAHEAD="$(extract_field "Active wave lookahead" "${INDEX_FILE}" || true)"
ACTIVE_WAVE_ID="$(extract_active_context_value "Wave" "${INDEX_FILE}" || true)"
ACTIVE_TRACKS="$(extract_active_context_value "Active tracks" "${INDEX_FILE}" || true)"
LATEST_GATE="$(extract_field "Latest gate artifact" "${INDEX_FILE}" || true)"
ACTIVE_WAVE="$(extract_field "Active wave runbook" "${INDEX_FILE}" || true)"
LINKED_CHILD="$(extract_field "Linked child plan integration" "${INDEX_FILE}" || true)"

WAVE_CHECKLIST="$(extract_wave_section_field "$(strip_ticks "${ACTIVE_WAVE_ID:-}")" "Checklist" "${INDEX_FILE}" || true)"
WAVE_STATUS="$(extract_wave_section_field "$(strip_ticks "${ACTIVE_WAVE_ID:-}")" "Status" "${INDEX_FILE}" || true)"
WAVE_COMMIT="$(extract_wave_section_field "$(strip_ticks "${ACTIVE_WAVE_ID:-}")" "Commit" "${INDEX_FILE}" || true)"
WAVE_VALIDATION="$(extract_wave_section_field "$(strip_ticks "${ACTIVE_WAVE_ID:-}")" "Validation" "${INDEX_FILE}" || true)"
WAVE_DRIFT="$(extract_wave_section_field "$(strip_ticks "${ACTIVE_WAVE_ID:-}")" "Drift" "${INDEX_FILE}" || true)"

if [[ -z "${LATEST_GATE}" && -n "${WAVE_CHECKLIST}" ]]; then
  LATEST_GATE="${WAVE_CHECKLIST}"
fi
if [[ -z "${ACTIVE_WAVE}" && -n "${ACTIVE_WAVE_KICKOFF}" ]]; then
  ACTIVE_WAVE="${ACTIVE_WAVE_KICKOFF}"
fi

HEAD_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
HEAD_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")"
GENERATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

PLAN_STATUS_FULL="$(git status --short -- "${PLAN_DIR}" 2>/dev/null || true)"
PLAN_STATUS_COMPACT="$(echo "${PLAN_STATUS_FULL}" | compact_status_lines)"
PLAN_STATUS_COUNT="$(echo "${PLAN_STATUS_COMPACT}" | sed '/^$/d' | wc -l | tr -d ' ')"
PLAN_STATUS="$(echo "${PLAN_STATUS_COMPACT}" | head -n 10)"
if [[ -z "${PLAN_STATUS}" ]]; then
  PLAN_STATUS="clean"
fi

REPO_STATUS_FULL="$(git status --short 2>/dev/null || true)"
REPO_STATUS_COMPACT="$(echo "${REPO_STATUS_FULL}" | compact_status_lines)"
REPO_STATUS="$(echo "${REPO_STATUS_COMPACT}" | head -n 10)"
REPO_STATUS_COUNT="$(echo "${REPO_STATUS_COMPACT}" | sed '/^$/d' | wc -l | tr -d ' ')"
if [[ -z "${REPO_STATUS}" ]]; then
  REPO_STATUS="clean"
fi

PLAN_COMMITS="$(git log --oneline -n 5 -- "${PLAN_DIR}" 2>/dev/null || true)"
if [[ -z "${PLAN_COMMITS}" ]]; then
  PLAN_COMMITS="(no commit history found for path)"
fi

SUPPORTING_DOCS=(
  ".agents/GUIDELINES.md"
  ".agents/ARCHITECTURE_DECISIONS.md"
  ".agents/CONTENT_NOTES.md"
  "CHANGELOG.md"
  "${INDEX_FILE}"
)

KICKOFF_FILE="$(strip_ticks "${ACTIVE_KICKOFF:-}")"
LOOKAHEAD_FILE="$(strip_ticks "${ACTIVE_LOOKAHEAD:-}")"
ACTIVE_WAVE_KICKOFF_FILE="$(strip_ticks "${ACTIVE_WAVE_KICKOFF:-}")"
ACTIVE_WAVE_LOOKAHEAD_FILE="$(strip_ticks "${ACTIVE_WAVE_LOOKAHEAD:-}")"
LATEST_GATE_FILE="$(echo "${LATEST_GATE:-}" | sed -n 's/.*`\([^`]*\)`.*/\1/p' | head -n 1)"
if [[ -z "${LATEST_GATE_FILE}" ]]; then
  LATEST_GATE_FILE="$(strip_ticks "${LATEST_GATE:-}")"
fi
NEXT_WAVE_KICKOFF_FILE="$(derive_kickoff_from_lookahead "${ACTIVE_WAVE_LOOKAHEAD_FILE}")"
ACTIVE_WAVE_LOOKAHEAD_PATH="${PLAN_DIR}/${ACTIVE_WAVE_LOOKAHEAD_FILE}"
CANDIDATE_TARGETS="$(collect_candidate_targets "${ACTIVE_WAVE_LOOKAHEAD_PATH}")"
if [[ -z "${CANDIDATE_TARGETS}" ]]; then
  CANDIDATE_TARGETS="- select one bounded boundary from the active wave lookahead"
fi

LATEST_COMMITTED_WAVE="unknown"
if [[ -n "${ACTIVE_WAVE_ID}" && -n "${WAVE_COMMIT}" && "${WAVE_COMMIT}" != "pending" ]]; then
  LATEST_COMMITTED_WAVE="$(strip_ticks "${ACTIVE_WAVE_ID}") @ $(strip_ticks "${WAVE_COMMIT}")"
fi

PLAN_LINKS_FILE="${PLAN_DIR}/plan_links.json"
PLAN_LINKS_SUMMARY="(plan_links.json not found)"
if [[ -f "${PLAN_LINKS_FILE}" ]]; then
  PLAN_LINKS_SUMMARY="$(node -e '
const fs=require("fs");
const p=process.argv[1];
const j=JSON.parse(fs.readFileSync(p,"utf8"));
const owner=j.owner_plan_id||"unknown";
const links=Array.isArray(j.links)?j.links:[];
const targets=links.map(l=>`${l.type||"unknown"}:${(l.target&&l.target.plan_id)||"unknown"}@v${(l.target&&l.target.version)||"?"}`);
console.log(`owner=${owner}; links=${links.length}; targets=${targets.join(", ") || "none"}`);
' "${PLAN_LINKS_FILE}" 2>/dev/null || echo "(unable to parse plan_links.json)")"
fi

SUGGESTED_CONTEXT_REFS_JSON="$(node -e '
const fs=require("fs");
const pulseTargets=(process.argv[1]||"").split("\n").map(s=>s.trim()).filter(Boolean);
const picks=[];
for (const raw of pulseTargets) {
  const cleaned=raw.replace(/^- /,"").replace(/^`|`$/g,"");
  if (!cleaned.endsWith(".md")) continue;
  if (!fs.existsSync(cleaned)) continue;
  picks.push(cleaned);
  if (picks.length>=8) break;
}
const refs=picks.map((p)=>({type:"file",ref:p}));
console.log(JSON.stringify(refs,null,2));
' "$(printf '%s\n' "${INDEX_FILE}" "${PLAN_DIR}/${KICKOFF_FILE}" "${PLAN_DIR}/${LOOKAHEAD_FILE}" "${PLAN_DIR}/${ACTIVE_WAVE_KICKOFF_FILE}" "${PLAN_DIR}/${ACTIVE_WAVE_LOOKAHEAD_FILE}" "${PLAN_DIR}/${LATEST_GATE_FILE}" ".agents/GUIDELINES.md" ".agents/ARCHITECTURE_DECISIONS.md" ".agents/CONTENT_NOTES.md" "CHANGELOG.md")")"

mkdir -p "$(dirname "${OUT_FILE}")"
{
  echo "# Pulse Save Point"
  echo
  echo "- Generated at (UTC): ${GENERATED_AT}"
  echo "- Branch/HEAD: \`${HEAD_BRANCH}\` @ \`${HEAD_SHA}\`"
  echo "- Plan directory: \`${PLAN_DIR}\`"
  echo "- Plan index: \`${INDEX_FILE}\`"
  echo
  echo "## Lifecycle Snapshot"
  echo "- Active kickoff: ${ACTIVE_KICKOFF:-unknown}"
  echo "- Active lookahead: ${ACTIVE_LOOKAHEAD:-unknown}"
  echo "- Active wave: ${ACTIVE_WAVE_ID:-unknown}"
  echo "- Active tracks: ${ACTIVE_TRACKS:-unknown}"
  echo "- Active wave kickoff: ${ACTIVE_WAVE_KICKOFF:-unknown}"
  echo "- Active wave lookahead: ${ACTIVE_WAVE_LOOKAHEAD:-unknown}"
  echo "- Latest gate artifact: ${LATEST_GATE:-unknown}"
  echo "- Latest committed wave: ${LATEST_COMMITTED_WAVE}"
  echo "- Active wave runbook: ${ACTIVE_WAVE:-unknown}"
  echo "- Linked child integration: ${LINKED_CHILD:-unknown}"
  echo "- Wave gate status: ${WAVE_STATUS:-unknown}"
  echo "- Wave validation: ${WAVE_VALIDATION:-unknown}"
  echo "- Wave drift: ${WAVE_DRIFT:-unknown}"
  echo
  echo "## Handoff Safety"
  echo
  echo "- Preferred checkpoint cadence: gate/HITL evidence, build/check, tests, coverage threshold, commit, then pulse-save."
  echo "- Manual eject use: run pulse-save when context is bloated, stale, or drifted enough that a fresh window is safer."
  echo "- Durable code fallback: \`${HEAD_BRANCH}\` @ \`${HEAD_SHA}\`."
  echo "- Dirty repo state is listed below; treat it as carryover unless the next action says otherwise."
  echo
  echo "## Immediate Next Agent Action"
  echo
  echo "Resume from the latest packaged gate, not from memory of the prior session."
  echo
  echo "1. Read \`${INDEX_FILE}\` and \`${PLAN_DIR}/${LATEST_GATE_FILE}\`."
  echo "2. Confirm gate evidence for ${ACTIVE_WAVE_ID:-the active wave}."
  if [[ -n "${ACTIVE_WAVE_LOOKAHEAD_FILE}" ]]; then
    echo "3. If the gate is accepted, rebaseline \`${ACTIVE_WAVE_LOOKAHEAD_FILE}\` into \`${NEXT_WAVE_KICKOFF_FILE:-the next wave kickoff}\`."
  else
    echo "3. If the gate is accepted, locate the next-wave lookahead in the plan index and rebaseline it into the next wave kickoff."
  fi
  echo "4. Select exactly one next-wave boundary from the lookahead:"
  while IFS= read -r target; do
    [[ -n "${target}" ]] && echo "   ${target}"
  done <<< "${CANDIDATE_TARGETS}"
  echo "5. Create/update the required next-wave lookahead for the following wave."
  echo "6. Stop at kickoff/gate setup unless the user explicitly says to start implementation."
  echo
  echo "Hard boundary:"
  echo "- Do not exceed the accepted plan or start multiple candidate targets."
  echo "- Do not call scheduled future wave work out of scope; keep it tied to its wave/phase."
  echo "- Do not commit \`.agents/*\` or \`.tempor/*\` unless explicitly requested."
  echo "- Do not stage or revert unrelated dirty files."
  echo
  echo "## Plan Links"
  echo "- File: \`${PLAN_LINKS_FILE}\`"
  echo "- Summary: ${PLAN_LINKS_SUMMARY}"
  echo
  echo "## Top 10 Targets"
  TARGETS=(
    "${INDEX_FILE}" "${PLAN_DIR}/${KICKOFF_FILE}" "${PLAN_DIR}/${LOOKAHEAD_FILE}"
    "${PLAN_DIR}/${ACTIVE_WAVE_KICKOFF_FILE}" "${PLAN_DIR}/${ACTIVE_WAVE_LOOKAHEAD_FILE}"
    "${PLAN_DIR}/${LATEST_GATE_FILE}" "${PLAN_LINKS_FILE}" "${PLAN_DIR}/archive/"
    ".agents/GUIDELINES.md" ".agents/ARCHITECTURE_DECISIONS.md" ".agents/skills/" "CHANGELOG.md"
  )
  seen=""; count=0
  for t in "${TARGETS[@]}"; do
    [[ -z "${t}" ]] && continue
    [[ "${seen}" == *"|${t}|"* ]] && continue
    if [[ -f "${t}" || -d "${t}" ]]; then
      echo "- \`${t}\`"
      seen="${seen}|${t}|"
      count=$((count+1))
    fi
    [[ ${count} -ge 10 ]] && break
  done
  echo
  echo "## Supporting Docs"
  for path in "${SUPPORTING_DOCS[@]}"; do
    [[ -f "${path}" ]] && echo "- \`${path}\`"
  done
  [[ -n "${ACTIVE_KICKOFF}" ]] && echo "- \`${PLAN_DIR}/$(echo "${ACTIVE_KICKOFF}" | tr -d '\`')\`"
  [[ -n "${ACTIVE_LOOKAHEAD}" ]] && echo "- \`${PLAN_DIR}/$(echo "${ACTIVE_LOOKAHEAD}" | tr -d '\`')\`"
  [[ -n "${ACTIVE_WAVE_KICKOFF}" ]] && echo "- \`${PLAN_DIR}/$(echo "${ACTIVE_WAVE_KICKOFF}" | tr -d '\`')\`"
  [[ -n "${ACTIVE_WAVE_LOOKAHEAD}" ]] && echo "- \`${PLAN_DIR}/$(echo "${ACTIVE_WAVE_LOOKAHEAD}" | tr -d '\`')\`"
  [[ -n "${LATEST_GATE_FILE}" ]] && echo "- \`${PLAN_DIR}/${LATEST_GATE_FILE}\`"
  echo "- \`.agents/skills/\` (explore all local skills)"
  echo
  echo "## Relevant Commits (Plan Path)"
  echo '```text'
  echo "${PLAN_COMMITS}"
  echo '```'
  echo
  echo "## MCP Input Guardrails"
  echo '- `tempor.create_plan.context_refs` must be an array of objects: `{ "type": "file", "ref": "path" }`.'
  echo '- Do not pass bare strings like `"file://..."` in `context_refs`.'
  echo '- Use local workspace-relative paths in `ref`.'
  echo "- Canonical example:"
  echo '```json'
  cat <<'EOF'
{
  "request": "Reevaluate product direction with latest assessments and planning artifacts.",
  "context_refs": [
    { "type": "file", "ref": ".agents/CODEX_FEEDBACK_AND_DIRECTION_V3.md" },
    { "type": "file", "ref": ".agents/CLAUDE_FEEDBACK_AND_DIRECTION_V3.md" },
    { "type": "file", "ref": ".agents/GEMINI_FEEDBACK_AND_DIRECTION_V3.md" },
    { "type": "file", "ref": ".agents/MCP_VS_AGENT_ANALYSIS.md" }
  ],
  "options": { "ambition": "high" }
}
EOF
  echo '```'
  echo "- Suggested context refs for next call:"
  echo '```json'
  echo "${SUGGESTED_CONTEXT_REFS_JSON}"
  echo '```'
  echo
  echo "## Working Tree Signal"
  echo "- Plan path status:"
  echo '```text'
  echo "${PLAN_STATUS}"
  echo '```'
  [[ "${PLAN_STATUS}" != "clean" && "${PLAN_STATUS_COUNT}" -gt 10 ]] && echo "- Plan path status: +$((PLAN_STATUS_COUNT-10)) more entries omitted"
  echo "- Repo status (top 10):"
  echo '```text'
  echo "${REPO_STATUS}"
  echo '```'
  [[ "${REPO_STATUS}" != "clean" && "${REPO_STATUS_COUNT}" -gt 10 ]] && echo "- Repo status: +$((REPO_STATUS_COUNT-10)) more entries omitted"
} > "${OUT_FILE}"

echo "Pulse save written to ${OUT_FILE}"
