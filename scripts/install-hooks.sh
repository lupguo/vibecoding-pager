#!/bin/bash
# Install Pager hook configuration into Claude Code settings.
# Usage: ./scripts/install-hooks.sh [AGENT_LABEL] [BRIDGE_PATH]
#
# AGENT_LABEL defaults to "CC"
# BRIDGE_PATH defaults to the bin/pager-cc-bridge relative to this script

set -euo pipefail

AGENT="${1:-CC}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BRIDGE_PATH="${2:-$SCRIPT_DIR/../bin/pager-cc-bridge}"
SETTINGS_FILE="$HOME/.claude/settings.json"

if [ ! -f "$BRIDGE_PATH" ]; then
  echo "Error: bridge binary not found at $BRIDGE_PATH"
  echo "Run 'make bridge' first."
  exit 1
fi

# Resolve to absolute path
BRIDGE_PATH="$(cd "$(dirname "$BRIDGE_PATH")" && pwd)/$(basename "$BRIDGE_PATH")"

# Ensure settings directory exists
mkdir -p "$(dirname "$SETTINGS_FILE")"

# Create settings file if it doesn't exist
if [ ! -f "$SETTINGS_FILE" ]; then
  echo '{}' > "$SETTINGS_FILE"
fi

# All 22 hook events to register
# Format: "HookName:EventType[:matcher]"
# EventType = CamelCase (matches CC's hook_event_name in stdin JSON)
python3 << PYTHON
import json

settings_file = "$SETTINGS_FILE"
bridge_path = "$BRIDGE_PATH"
agent = "$AGENT"

events = [
    ("SessionStart",        "SessionStart",        None),
    ("SessionEnd",          "SessionEnd",          None),
    ("UserPromptSubmit",    "UserPromptSubmit",    None),
    ("UserPromptExpansion", "UserPromptExpansion", None),
    ("Stop",                "Stop",                None),
    ("StopFailure",         "StopFailure",         None),
    ("PreToolUse",          "PreToolUse",          "*"),
    ("PostToolUse",         "PostToolUse",         "*"),
    ("PostToolUseFailure",  "PostToolUseFailure",  "*"),
    ("PostToolBatch",       "PostToolBatch",       None),
    ("PermissionRequest",   "PermissionRequest",   "*"),
    ("PermissionDenied",    "PermissionDenied",    "*"),
    ("SubagentStart",       "SubagentStart",       None),
    ("SubagentStop",        "SubagentStop",        None),
    ("TaskCreated",         "TaskCreated",         None),
    ("TaskCompleted",       "TaskCompleted",       None),
    ("Notification",        "Notification",        None),
    ("PreCompact",          "PreCompact",          None),
    ("PostCompact",         "PostCompact",          None),
    ("InstructionsLoaded",  "InstructionsLoaded",  None),
    ("Elicitation",         "Elicitation",         None),
    ("MessageDisplay",      "MessageDisplay",      None),
]

with open(settings_file, 'r') as f:
    settings = json.load(f)

if 'hooks' not in settings:
    settings['hooks'] = {}

for hook_name, event_type, matcher in events:
    hook_entry = {
        "type": "command",
        "command": f"{bridge_path} --event {event_type} --agent {agent}",
        "timeout": 5,
        "async": True
    }
    hook_group = {"hooks": [hook_entry]}
    if matcher:
        hook_group["matcher"] = matcher

    settings['hooks'][hook_name] = [hook_group]

with open(settings_file, 'w') as f:
    json.dump(settings, f, indent=2)

print(f"✓ Installed {len(events)} hooks for agent '{agent}'")
print(f"  Bridge: {bridge_path}")
print(f"  Config: {settings_file}")
PYTHON
