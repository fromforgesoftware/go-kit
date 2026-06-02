// Package timeline provides typed-event timeline replay: a sorted
// sequence of events at known offsets, advanced forward over an
// interval via a Cursor. Deterministic by construction — same input
// produces the same output emissions.
//
// Domain-agnostic media-playback shape; common applications:
//   - Animation systems (notify-state replay, keyframe events).
//   - Video / audio editors (timeline-driven automation).
//   - Music sequencers (note-on / note-off scheduling).
//   - Scheduled-task playback / replay-driven simulations.
//   - Any system that needs to replay a sequence of timed events exactly.
package timeline
