# HEARTBEAT.md

# Keep this file empty (or with only comments) to skip heartbeat API calls.

# Add tasks below when you want the agent to check something periodically.

---

## NON-NEGOTIABLE ROUTINES

### Heartbeat Memory Maintenance (HIGHEST PRIORITY)
During EVERY applicable heartbeat cycle (every 4 hours):
1. Check if `memory/YYYY-MM-DD.md` exists for today
2. If not, CREATE it immediately with basic session info
3. If yes, UPDATE it with relevant events/learnings from since last check
4. This happens BEFORE any other heartbeat tasks

### Self-Review Verification (Every 4 Hours)
During self-review assessments:
1. Verify previous commitments through OBJECTIVE EVIDENCE (file timestamps, git commits)
2. Do NOT accept self-reported completion without verification
3. If verification reveals failure, include IMMEDIATE CORRECTIVE ACTION in same assessment
4. Document evidence in self-review.md completion proof section

---

## PERIODIC CHECKS (Rotate Through, 2-4x Daily)

- **Emails**: Any urgent unread messages?
- **Calendar**: Upcoming events in next 24-48h?
- **Weather**: Relevant if ALPHA might go out?
- **Project Status**: Git status on key repos, any urgent issues?

---

## TRACKING

State tracked in: `memory/heartbeat-state.json`

**Last Memory File Check:** [Update during each heartbeat]
**Last Self-Review:** [Update during each 4-hour assessment]

---

## HEARTBEAT SCHEDULE

Heartbeat checks occur every **4 hours** starting from 1:01 PM.
Scheduled times: **1:01 PM, 5:01 PM, 9:01 PM, 1:01 AM** (Africa/Lagos timezone)

**IMPORTANT**: Only respond to heartbeat polls at these exact times. Do NOT check every 30 minutes.
If you find yourself checking more frequently, STOP and wait for the next scheduled time.

Last verified: February 20, 2026 at 5:01 PM ✅
Next check: 9:01 PM today
