#!/bin/bash
# Register Pager as a launchd LaunchAgent for auto-start and crash recovery.

APP_PATH="${1:?Usage: $0 /Applications/Pager.app}"
PLIST="$HOME/Library/LaunchAgents/com.sapaude.pager.plist"

mkdir -p "$HOME/.pager"

cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.sapaude.pager</string>
    <key>ProgramArguments</key>
    <array>
        <string>$APP_PATH/Contents/MacOS/Pager</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/.pager/pager.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.pager/pager-error.log</string>
</dict>
</plist>
EOF

launchctl load "$PLIST"
echo "✓ Pager LaunchAgent registered."
