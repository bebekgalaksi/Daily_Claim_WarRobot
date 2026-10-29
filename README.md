# Daily_Claim_WarRobot

Automated tool written in **Go** to automatically claim the daily free items (such as **Common Data Pad Silver**) on the [MY.GAMES War Robots Market](https://market.my.games/war_robots?content=items).

---

## Features
- **Pure Go**: Lightweight and compiled with standard library components.
- **Smart Session Management**: Reuses existing sessions and checks if you are already logged in as your player profile. Automatically logs in with your `.env` credentials only when the session expires or logs out.
- **Scheduled Runs**: Run via `systemd` user timer (daily or on boot) or execute manually anytime.
- **Browser Fallback**: If running on a desktop with Brave browser, it can also extract active browser cookies automatically.
- **Desktop Notifications**: Sends native Linux desktop notifications via `notify-send`.

---

## Quick Start

### 1. Configure Credentials
Copy the example environment file and fill in your details:
```bash
cp .env.example .env
```
Edit `.env`:
```env
name=YourPlayerName
id=your_email@example.com
pass=your_password_here
```

### 2. Build from Source
```bash
go build -ldflags="-s -w" -o war-robots-autoclaim main.go
```

Or build a completely static binary:
```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o war-robots-autoclaim main.go
```

### 3. Run Manually
```bash
./war-robots-autoclaim
```

---

## Automated Scheduling with systemd

### Run on PC Startup (Desktop)
Copy the service and desktop files:
```bash
# Copy binary
mkdir -p ~/.local/bin
cp war-robots-autoclaim ~/.local/bin/

# Autostart on desktop login
mkdir -p ~/.config/autostart
cp war-robots-autoclaim.desktop ~/.config/autostart/
```

### Run Daily at 23:00 (Headless Server / Systemd Timer)
1. Copy the systemd service and timer:
```bash
mkdir -p ~/.config/systemd/user
cp war-robots-autoclaim.service ~/.config/systemd/user/
cp war-robots-autoclaim.timer ~/.config/systemd/user/
```

2. Enable lingering so the timer runs 24/7 without needing an active SSH session:
```bash
loginctl enable-linger $USER
```

3. Reload and enable the timer:
```bash
systemctl --user daemon-reload
systemctl --user enable --now war-robots-autoclaim.timer
```

4. Check timer status:
```bash
systemctl --user list-timers war-robots-autoclaim.timer
```

5. Check logs:
```bash
journalctl --user -u war-robots-autoclaim.service --no-pager -n 25
```

---

## Alternative Browser Scripts
- **[claim_userscript.user.js](claim_userscript.user.js)**: Tampermonkey / Violentmonkey userscript that auto-claims whenever you open the page in your browser.
- **[claim_console_snippet.js](claim_console_snippet.js)**: DevTools Console script to claim with one paste.

---

## License
MIT
