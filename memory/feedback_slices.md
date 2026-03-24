---
name: slice and map initialization
description: Always use make() for slices and maps, with size hints when known
type: feedback
---

Always use `make([]T, 0)` not `[]T{}`, and `make(map[K]V)` not `map[K]V{}`. Pass a size/capacity hint whenever it's known ahead of time.

**Why:** Capacity hints avoid reallocations for slices. For maps it's even more important — Go rehashes aggressively on growth, so a zero-hint map is more expensive to fill than a slice.

**How to apply:** Any time a slice or map is initialized — applies to var declarations, := assignments, and struct fields.
