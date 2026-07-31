# Design Review: Runspace Landing Page

Reviewed against: supplied Runspace brand guide and the established “calm
technical instrument” direction  
Date: 2026-07-30

## Screenshots Captured

| Screenshot | Breakpoint | Description |
| --- | --- | --- |
| `screenshots/review-landing-desktop-1280.png` | Desktop (1280×800) | Full dark-mode landing page |
| `screenshots/review-landing-tablet-768.png` | Tablet (768×1024) | Full dark-mode tablet layout |
| `screenshots/review-landing-mobile-375.png` | Mobile (375×812) | Full dark-mode mobile layout |
| `screenshots/review-landing-light-desktop-1280.png` | Desktop (1280×800) | Full light-mode landing page |

## Summary

The page is polished but reads as an AI-generated SaaS concept: oversized
headlines, a neon accent, grid backgrounds, abstract diagrams, a tiny product
mockup, repeated numbered sections, and a glowing closing ribbon. These devices
are individually competent but collectively familiar and interchangeable.

The brand guide is being imitated at the level of color and surface treatment,
not translated into a persuasive product story. The page needs fewer visual
systems, more real product evidence, and a clearer explanation of who Runspace
is for and why its model is materially better.

## Must Fix

1. **The page is a sequence of design tropes rather than a product argument.**
   Every section repeats the same formula: small index, magenta kicker, huge
   two-line heading, grey paragraph, bordered technical graphic. See
   `review-landing-desktop-1280.png`. This creates rhythm without progression.
   _Fix: rebuild the narrative around one concrete job, one real product
   walkthrough, and one proof section._

2. **The hero prioritizes a generic slogan over differentiation.** “From
   discussion to execution” could describe dozens of automation and AI
   products. The miniature workspace at `layouts/index.html:96` is too small to
   provide evidence and contains invented activity rather than a recognizable
   product outcome. _Fix: name the audience, the object being governed, and the
   measurable change Runspace creates._

3. **Mobile contains unreadable product evidence.** The hero workspace is
   horizontally cropped and its content becomes decorative noise. The workflow
   uses 7–8px labels (`assets/css/site.css:861`, `:940`, `:1034`), far below a
   credible mobile reading size. See `review-landing-mobile-375.png`. _Fix:
   replace desktop miniatures with purpose-built mobile states and remove any
   copy that cannot be rendered legibly._

4. **The product is never proven.** There are no real screenshots, integrations,
   customer scenarios, supported execution environments, security boundaries,
   or concrete outputs. Abstract diagrams cannot substitute for this. _Fix:
   introduce real UI and specific examples such as repository access, approval
   scope, command output, changed files, and retained audit history._

## Should Fix

1. **Too many competing visual metaphors.** The hero grid, orbital system map,
   three-column capability ledger, execution trace, and fluid neon wave each
   establish a different visual language (`layouts/index.html:78`, `:168`,
   `:211`, `:252`, `:320`). This is the strongest source of the “AI-ish”
   impression. _Fix: choose one visual grammar—preferably product UI plus
   precise editorial rules—and remove the rest._

2. **The system map communicates less than its size implies.** “People,
   Context, Compute, Result” arranged around a central Space is taxonomy, not an
   explanatory diagram. The circular orbit suggests relationships that are not
   defined. See all three dark-mode screenshots. _Fix: replace it with a
   concrete before/after or a single annotated product view._

3. **The workflow diagram repeats preceding copy.** It is more legible than the
   system map, but the four boxed stages still feel like generated presentation
   furniture. At tablet width the cards become narrow and text wraps awkwardly.
   _Fix: show one actual run changing state over time, with fewer annotations
   and larger evidence._

4. **Whitespace is being used as spectacle.** Large gaps between sections make
   the desktop page exceptionally long without adding information. This feels
   like luxury-template pacing rather than operational confidence. _Fix:
   shorten the page by roughly one third and use density where the product needs
   explanation._

5. **The light theme is not a second coherent art direction.** It is a token
   inversion around a permanently dark product mockup. The result feels washed
   out and the neon wave becomes especially synthetic. See
   `review-landing-light-desktop-1280.png`. _Fix: remove the public theme toggle
   unless light mode is a product requirement; the supplied brand direction is
   strongest in dark mode._

6. **The magenta accent is too ubiquitous.** Although it does not fill large
   surfaces, it marks kickers, node numbers, diagrams, state dots, buttons,
   borders, headings, and the decorative wave. Its semantic meaning has been
   diluted from “active compute.” _Fix: reserve magenta for approval/execution
   state and the primary CTA only._

7. **The final CTA is visually loud but strategically weak.** “Keep the work
   where it happens” over a glowing ribbon is attractive but generic, and the
   repeated “Request early access” provides no reason to act now. _Fix: end with
   a specific invitation tied to the target user and expected pilot outcome._

## Could Improve

1. **Typography lacks a proprietary feel.** TT Norms is only declared as a
   local font and will usually fall back to Inter. The result is clean but
   generic. Bundle the licensed brand font or choose one intentional available
   substitute.
2. **Navigation labels are internal concepts.** “The system,” “Control,” and
   “Workflow” describe the page structure, not user questions. Prefer links
   such as “How it works,” “Security,” and “Use cases.”
3. **CTA hierarchy is repetitive.** The same early-access action appears in the
   header, hero, and final block. Add one low-commitment secondary action that
   exposes proof, such as watching a real run or reading the architecture.
4. **The copy remains abstract.** “Accountable agency,” “shared intent,” and
   “capability without ambiguity” sound designed. Plain language would feel
   more confident.

## What Works Well

- The approved wordmark, charcoal base, bone-white typography, and restrained
  border treatment are consistent.
- Dark-mode contrast and primary CTA discovery are strong.
- The mobile navigation is contained and touch-friendly.
- The overall implementation is responsive and technically disciplined.
- The approval boundary is the strongest product idea on the page and should
  become the center of the next direction.

## Recommended Reset

Build a shorter, evidence-led page:

1. A precise headline naming the audience and outcome.
2. One large, real Runspace product view with three annotated moments:
   context, approval, result.
3. One plain-language explanation of the permission model.
4. One concrete end-to-end example.
5. One credibility block covering deployment, isolation, and auditability.
6. One focused pilot CTA.

Keep the wordmark, dark palette, thin rules, and approval-state magenta. Remove
the orbital map, decorative capability cards, theme toggle, most grid
backgrounds, and the glowing closing ribbon.
