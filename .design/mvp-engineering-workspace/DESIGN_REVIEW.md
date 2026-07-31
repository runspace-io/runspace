# Design Review: Runspace MVP Engineering Workspace

Reviewed against: `DESIGN_BRIEF.md`  
Philosophy: calm technical instrument  
Date: 2026-07-29

## Screenshots Captured

| Screenshot                                                    | Breakpoint         | Description                                                                 |
| ------------------------------------------------------------- | ------------------ | --------------------------------------------------------------------------- |
| `screenshots/review-workspace-desktop-1280.png`               | Desktop (1280×800) | Active channel with repository, human/agent/member timeline, and run status |
| `screenshots/review-workspace-desktop-changes-1280.png`       | Desktop (1280×800) | Changed-file list and Monaco Diff Editor with a real terminal-authored edit |
| `screenshots/review-workspace-tablet-768.png`                 | Tablet (768×1024)  | Channel with the navigation drawer closed                                   |
| `screenshots/review-workspace-tablet-768-navigation-open.png` | Tablet (768×1024)  | Functional repository/channel drawer                                        |
| `screenshots/review-workspace-mobile-375.png`                 | Mobile (375×812)   | View-and-control collaboration layout                                       |
| `screenshots/review-workspace-mobile-375-navigation-open.png` | Mobile (375×812)   | Functional mobile navigation drawer                                         |

All screenshots are stored in `.design/mvp-engineering-workspace/screenshots/`.
They are reproducible with `pnpm review:screenshots`.

## Summary

The implemented workspace now follows the restrained Runspace brand language:
neutral charcoal surfaces, compact technical typography, and magenta reserved
for active actions and compute state. The core responsive shell, channel
navigation, repository tree, chat attribution, terminal toggle, and run controls
are visually coherent and functional across the reviewed breakpoints.

The remaining gaps are interaction hardening rather than inert product
surfaces. Repository review, persisted resizable panels, and contained
pull-request publishing are implemented and tested.

## Must Fix

1. **Dialogs and drawers need complete focus management.** Visible focus rings
   and labeled actions are present, but focus trapping, Escape dismissal, and
   restoration to the invoking control are not yet consistently implemented.

## Should Fix

1. **Use a true PTY for terminal parity.** Shell input/output now works through
   xterm.js and the Docker WebSocket session, but the backend uses process pipes.
   Full-screen terminal programs, resize signals, and terminal line discipline
   require a PTY implementation.
2. **Virtualize long timelines after measurement.** The ChatScope timeline is
   functional and branded, but long-running channels should be profiled and
   moved to a measured virtualized list before large histories are supported.
3. **Make mobile icon actions more discoverable.** Members and sign-out use
   accessible labels at mobile width, but a compact user menu would provide
   clearer visual meaning without consuming topbar space.
4. **Bundle fonts for offline development.** The current Google Fonts import
   falls back safely, but locally hosted Inter and JetBrains Mono assets would
   make the contained Docker experience deterministic without internet access.

## What Works Well

- The Changes panel reads normalized Git status and shows actual HEAD and
  working-tree content in Monaco Diff Editor. Modified, added, deleted,
  untracked, renamed, traversal, binary, and size behavior have backend tests;
  the browser path edits a repository through Runspace's terminal and reviews
  that change.
- Desktop navigation and content panels resize with persisted proportions.
- Pull-request publishing is integration-tested with native Git, a local bare
  remote, and a contained GitHub-compatible HTTP server.
- The desktop hierarchy clearly prioritizes channel context and collaboration
  while leaving code and terminal behind explicit toggles.
- Tablet and mobile navigation now use a real opaque drawer; open and close
  controls both change the interface.
- Repository folders expand and collapse inline with accurate accessibility
  state, and ignored `.git` metadata is not presented as user content.
- Human, agent, and remote-member messages are attributed explicitly. The
  screenshots show admin messages as `You`, agent output as `Agent`, and the
  independent collaborator as `alice`.
- The palette closely follows the supplied Runspace brand guide. Magenta is
  used for connection, active run state, send actions, and restrained message
  edge accents instead of large decorative fields.
- Mobile hides code, terminal, and publish actions while retaining channel
  history, run state, settings, agent control, and the composer.
