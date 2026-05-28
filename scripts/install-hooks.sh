#!/bin/bash
# Install pager-cc-bridge hook into ~/.claude/settings.json
# Usage: ./install-hooks.sh /path/to/pager-cc-bridge

set -e

BRIDGE_PATH="${1:?Usage: $0 /path/to/pager-cc-bridge}"

if [ ! -x "$BRIDGE_PATH" ]; then
  echo "Error: $BRIDGE_PATH does not exist or is not executable"
  exit 1
fi

SETTINGS="$HOME/.claude/settings.json"

HOOK_CONFIG=$(cat <<'EOF'
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "%BRIDGE_PATH% pre_tool_use",
        "async": true,
        "timeout": 5
      }]
    }],
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "%BRIDGE_PATH% post_tool_use",
        "async": true,
        "timeout": 5
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "%BRIDGE_PATH% stop",
        "async": true,
        "timeout": 5
      }]
    }]
  }
}
EOF
)

# Replace placeholder with actual bridge path
HOOK_CONFIG="${HOOK_CONFIG//\%BRIDGE_PATH\%/$BRIDGE_PATH}"

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
echo "  Restart Claude Code to activate."
