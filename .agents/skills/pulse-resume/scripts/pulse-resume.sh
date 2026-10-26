#!/usr/bin/env bash
set -euo pipefail

PULSE_FILE="${1:-.tempor/PULSE_CHECK.md}"

if [[ ! -f "${PULSE_FILE}" ]]; then
  echo "Missing pulse file: ${PULSE_FILE}" >&2
  echo "Run pulse-save first." >&2
  exit 1
fi

GENERATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
PLAN_DIR="$(rg '^- Plan directory:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Plan directory: `([^`]*)`/\1/' || true)"
PLAN_INDEX="$(rg '^- Plan index:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Plan index: `([^`]*)`/\1/' || true)"
ACTIVE_KICKOFF="$(rg '^- Active kickoff:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Active kickoff: (.*)/\1/' || true)"
ACTIVE_LOOKAHEAD="$(rg '^- Active lookahead:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Active lookahead: (.*)/\1/' || true)"
ACTIVE_WAVE="$(rg '^- Active wave:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Active wave: (.*)/\1/' || true)"
ACTIVE_TRACKS="$(rg '^- Active tracks:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Active tracks: (.*)/\1/' || true)"
ACTIVE_WAVE_KICKOFF="$(rg '^- Active wave kickoff:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Active wave kickoff: (.*)/\1/' || true)"
ACTIVE_WAVE_LOOKAHEAD="$(rg '^- Active wave lookahead:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Active wave lookahead: (.*)/\1/' || true)"
LATEST_GATE="$(rg '^- Latest gate artifact:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Latest gate artifact: (.*)/\1/' || true)"
LATEST_COMMITTED_WAVE="$(rg '^- Latest committed wave:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Latest committed wave: (.*)/\1/' || true)"
WAVE_GATE_STATUS="$(rg '^- Wave gate status:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Wave gate status: (.*)/\1/' || true)"
WAVE_VALIDATION="$(rg '^- Wave validation:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Wave validation: (.*)/\1/' || true)"
WAVE_DRIFT="$(rg '^- Wave drift:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Wave drift: (.*)/\1/' || true)"
PULSE_HEAD_SHA="$(rg '^- Branch/HEAD:' "${PULSE_FILE}" | head -n 1 | sed -E 's/^- Branch\/HEAD: `[^`]*` @ `([^`]*)`/\1/' || true)"

strip_ticks() {
  echo "$1" | tr -d '\`'
}

TARGETS="$(awk '
  /^## Top 10 Targets$/ { in_targets=1; next }
  /^## / && in_targets==1 { in_targets=0 }
  in_targets==1 && /^- `/ { print }
' "${PULSE_FILE}" | head -n 10)"

ACTIVE_KICKOFF_FILE="$(echo "${ACTIVE_KICKOFF}" | tr -d '\`')"
ACTIVE_KICKOFF_PATH=""
if [[ -n "${PLAN_DIR}" && -n "${ACTIVE_KICKOFF_FILE}" ]]; then
  ACTIVE_KICKOFF_PATH="${PLAN_DIR}/${ACTIVE_KICKOFF_FILE}"
fi

LATEST_GATE_FILE="$(strip_ticks "${LATEST_GATE:-}")"
LATEST_GATE_PATH=""
if [[ -n "${PLAN_DIR}" && -n "${LATEST_GATE_FILE}" && "${LATEST_GATE_FILE}" != "unknown" ]]; then
  LATEST_GATE_PATH="${PLAN_DIR}/${LATEST_GATE_FILE}"
fi

ACTIVE_WAVE_KICKOFF_FILE="$(strip_ticks "${ACTIVE_WAVE_KICKOFF:-}")"
ACTIVE_WAVE_KICKOFF_PATH=""
if [[ -n "${PLAN_DIR}" && -n "${ACTIVE_WAVE_KICKOFF_FILE}" && "${ACTIVE_WAVE_KICKOFF_FILE}" != "unknown" ]]; then
  ACTIVE_WAVE_KICKOFF_PATH="${PLAN_DIR}/${ACTIVE_WAVE_KICKOFF_FILE}"
fi

ACTIVE_WAVE_LOOKAHEAD_FILE="$(strip_ticks "${ACTIVE_WAVE_LOOKAHEAD:-}")"
ACTIVE_WAVE_LOOKAHEAD_PATH=""
if [[ -n "${PLAN_DIR}" && -n "${ACTIVE_WAVE_LOOKAHEAD_FILE}" && "${ACTIVE_WAVE_LOOKAHEAD_FILE}" != "unknown" ]]; then
  ACTIVE_WAVE_LOOKAHEAD_PATH="${PLAN_DIR}/${ACTIVE_WAVE_LOOKAHEAD_FILE}"
fi

PULSE_NEXT_ACTION_RAW="$(awk '
  /^## Immediate Next Agent Action$/ { in_block=1; next }
  /^## / && in_block==1 { exit }
  /^Hard boundary:/ && in_block==1 { exit }
  in_block==1 && /^[0-9]+\./ {
    sub(/^[0-9]+\.[[:space:]]*/, "", $0);
    print $0
  }
  in_block==1 && /^[[:space:]]+- / {
    sub(/^[[:space:]]+- /, "", $0);
    print $0
  }
' "${PULSE_FILE}")"

HARD_BOUNDARIES_RAW="$(awk '
  /^Hard boundary:/ { in_block=1; next }
  /^## / && in_block==1 { exit }
  in_block==1 && /^- / {
    sub(/^- /, "", $0);
    print $0
  }
' "${PULSE_FILE}")"

HANDOFF_SAFETY_RAW="$(awk '
  /^## Handoff Safety$/ { in_block=1; next }
  /^## / && in_block==1 { exit }
  in_block==1 && /^- / {
    sub(/^- /, "", $0);
    print $0
  }
' "${PULSE_FILE}")"

IMMEDIATE_START_ORDER_RAW=""
if [[ -n "${ACTIVE_KICKOFF_PATH}" && -f "${ACTIVE_KICKOFF_PATH}" ]]; then
  IMMEDIATE_START_ORDER_RAW="$(awk '
    /^### Immediate Start Order/ { in_block=1; next }
    /^### / && in_block==1 { exit }
    in_block==1 && /^[0-9]+\./ {
      sub(/^[0-9]+\.[[:space:]]*/, "", $0);
      print $0
    }
  ' "${ACTIVE_KICKOFF_PATH}")"
fi

TARGETS_JSON="$(printf '%s\n' "${TARGETS}" | sed -E 's/^- `([^`]*)`$/\1/' | sed -E 's/^- (.*)$/\1/' | node -e '
const fs=require("fs");
const lines=fs.readFileSync(0,"utf8").split(/\r?\n/).map(s=>s.trim()).filter(Boolean);
console.log(JSON.stringify(lines));
')"

IMMEDIATE_START_ORDER_JSON="$(printf '%s\n' "${IMMEDIATE_START_ORDER_RAW}" | node -e '
const fs=require("fs");
const lines=fs.readFileSync(0,"utf8").split(/\r?\n/).map(s=>s.trim()).filter(Boolean);
console.log(JSON.stringify(lines));
')"

PULSE_NEXT_ACTION_JSON="$(printf '%s\n' "${PULSE_NEXT_ACTION_RAW}" | node -e '
const fs=require("fs");
const lines=fs.readFileSync(0,"utf8").split(/\r?\n/).map(s=>s.trim()).filter(Boolean);
console.log(JSON.stringify(lines));
')"

HARD_BOUNDARIES_JSON="$(printf '%s\n' "${HARD_BOUNDARIES_RAW}" | node -e '
const fs=require("fs");
const lines=fs.readFileSync(0,"utf8").split(/\r?\n/).map(s=>s.trim()).filter(Boolean);
console.log(JSON.stringify(lines));
')"

HANDOFF_SAFETY_JSON="$(printf '%s\n' "${HANDOFF_SAFETY_RAW}" | node -e '
const fs=require("fs");
const lines=fs.readFileSync(0,"utf8").split(/\r?\n/).map(s=>s.trim()).filter(Boolean);
console.log(JSON.stringify(lines));
')"

CREATE_PLAN_CONTEXT_REFS_JSON="$(node -e '
const fs=require("fs");
const targets=JSON.parse(process.argv[1]||"[]");
const refs=[];
for (const t of targets) {
  if (!t.endsWith(".md")) continue;
  if (!fs.existsSync(t)) continue;
  refs.push({type:"file", ref:t});
  if (refs.length>=6) break;
}
console.log(JSON.stringify(refs));
' "${TARGETS_JSON}")"

node -e '
const obj = {
  format: "PULSE_RESUME",
  meta: {
    generated_at_utc: process.argv[1],
    source_pulse: process.argv[2],
    pulse_head_sha: process.argv[3] || null,
    plan_directory: process.argv[4] || null,
    plan_index: process.argv[5] || null,
  },
  lifecycle: {
    active_kickoff: process.argv[6] || null,
    active_lookahead: process.argv[7] || null,
    active_wave: process.argv[13] || null,
    active_tracks: process.argv[14] || null,
    active_wave_kickoff: process.argv[15] || null,
    active_wave_lookahead: process.argv[16] || null,
    latest_gate_artifact: process.argv[8] || null,
    latest_committed_wave: process.argv[17] || null,
    wave_gate_status: process.argv[18] || null,
    wave_validation: process.argv[19] || null,
    wave_drift: process.argv[20] || null,
  },
  resume_plan: {
    kickoff_path: process.argv[10] || null,
    gate_path: process.argv[21] || null,
    wave_kickoff_path: process.argv[22] || null,
    wave_lookahead_path: process.argv[23] || null,
    pulse_next_actions: JSON.parse(process.argv[24] || "[]"),
    immediate_start_order: JSON.parse(process.argv[11] || "[]"),
    next_action: null,
    begin_with_command: process.argv[5] ? `sed -n '\''1,220p'\'' ${process.argv[5]}` : null
  },
  handoff_safety: JSON.parse(process.argv[25] || "[]"),
  hard_boundaries: JSON.parse(process.argv[26] || "[]"),
  first_5_commands: [
    `cat ${process.argv[2]}`,
    process.argv[5] ? `sed -n '\''1,220p'\'' ${process.argv[5]}` : null,
    process.argv[21] ? `sed -n '\''1,220p'\'' ${process.argv[21]}` : null,
    process.argv[23] ? `sed -n '\''1,220p'\'' ${process.argv[23]}` : null,
    "git status --short"
  ].filter(Boolean),
  top_10_targets: JSON.parse(process.argv[9] || "[]"),
  create_plan_input_template: {
    request: "Reevaluate product direction with latest assessments and planning artifacts.",
    context_refs: JSON.parse(process.argv[12] || "[]"),
    options: { ambition: "high" },
    guardrails: [
      "context_refs must be objects with type + ref",
      "use workspace-relative file paths in ref",
      "do not pass file:// strings directly in context_refs"
    ]
  },
  handoff_rule: {
    if_pulse_stale_then: "bash .agents/skills/pulse-save/scripts/pulse-save.sh"
  }
};
if (obj.resume_plan.immediate_start_order.length > 0) {
  obj.resume_plan.next_action = obj.resume_plan.immediate_start_order[0];
}
if (obj.resume_plan.pulse_next_actions.length > 0) {
  obj.resume_plan.next_action = obj.resume_plan.pulse_next_actions[0];
}
console.log(JSON.stringify(obj, null, 2));
' "${GENERATED_AT}" "${PULSE_FILE}" "${PULSE_HEAD_SHA}" "${PLAN_DIR}" "${PLAN_INDEX}" "${ACTIVE_KICKOFF}" "${ACTIVE_LOOKAHEAD}" "${LATEST_GATE}" "${TARGETS_JSON}" "${ACTIVE_KICKOFF_PATH}" "${IMMEDIATE_START_ORDER_JSON}" "${CREATE_PLAN_CONTEXT_REFS_JSON}" "${ACTIVE_WAVE}" "${ACTIVE_TRACKS}" "${ACTIVE_WAVE_KICKOFF}" "${ACTIVE_WAVE_LOOKAHEAD}" "${LATEST_COMMITTED_WAVE}" "${WAVE_GATE_STATUS}" "${WAVE_VALIDATION}" "${WAVE_DRIFT}" "${LATEST_GATE_PATH}" "${ACTIVE_WAVE_KICKOFF_PATH}" "${ACTIVE_WAVE_LOOKAHEAD_PATH}" "${PULSE_NEXT_ACTION_JSON}" "${HANDOFF_SAFETY_JSON}" "${HARD_BOUNDARIES_JSON}"
