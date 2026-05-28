#!/bin/bash
# Install pager-cc-bridge hook into ~/.claude/settings.json
# Usage: ./install-hooks.sh /path/to/pager-cc-bridge [--agent LABEL]

set -e

BRIDGE_PATH=""
AGENT_LABEL="CC"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --agent)
      AGENT_LABEL="$2"
      shift 2
      ;;
    *)
      if [ -z "$BRIDGE_PATH" ]; then
        BRIDGE_PATH="$1"
      fi
      shift
      ;;
  esac
done

if [ -z "$BRIDGE_PATH" ]; then
  echo "Usage: $0 /path/to/pager-cc-bridge [--agent LABEL]"
  echo "  --agent LABEL   Agent identifier shown in Pager UI (default: CC)"
  echo "  Examples: CC, CC-Int, Codex, MyAgent"
  exit 1
fi

if [ ! -x "$BRIDGE_PATH" ]; then
  echo "Error: $BRIDGE_PATH does not exist or is not executable"
  exit 1
fi

SETTINGS="$HOME/.claude/settings.json"

HOOK_CONFIG=$(cat <<EOF
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH pre_tool_use --agent $AGENT_LABEL",
        "async": true,
        "timeout": 5
      }]
    }],
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH post_tool_use --agent $AGENT_LABEL",
        "async": true,
        "timeout": 5
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH stop --agent $AGENT_LABEL",
        "async": true,
        "timeout": 5
      }]
    }]
  }
}
EOF
)

# Backup existing config
[ -f "$SETTINGS" ] && cp "$SETTINGS" "$SETTINGS.pager-backup"

# Merge or create
if [ -f "$SETTINGS" ]; then
  if ! command -v jq &> /dev/null; then
    echo "Error: jq is required. Install with: brew install jq"
    exit 1
  fi
  jq -s '.[0] * .[1]' "$SETTINGS" <(echo "$HOOK_CONFIG") > "$SETTINGS.tmp" && mv "$SETTINGS.tmp" "$SETTINGS"
else
  mkdir -p "$(dirname "$SETTINGS")"
  echo "$HOOK_CONFIG" > "$SETTINGS"
fi

echo "✓ Hook installed to $SETTINGS"
echo "  Agent label: $AGENT_LABEL"
echo "  Restart Claude Code to activate."
