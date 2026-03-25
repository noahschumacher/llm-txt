# Evaluation Framework

## Summary

A scoring system for generated llms.txt output — comparing basic vs enhanced mode on the same crawl, and benchmarking against ground-truth llms.txt files that sites publish themselves.

## Motivation

Right now quality is judged by eye. An eval framework would let us iterate on the prompt, section logic, and crawler with measurable signal rather than vibes.

This is also directly relevant for an AI product: automated evals are how you ship prompt changes confidently.

## Two Eval Modes

### 1. Basic vs Enhanced Comparison

Already have the data from `writeDebugOutputs`. Add a scorer that runs on the pair and reports:

| Dimension | Method |
|---|---|
| Description length | Mean word count — shorter is better up to a point |
| "This page..." prefix rate | Regex — lower is better |
| Blank descriptions | Count of entries with no description |
| Section count | More sections = better content organisation |
| Unique descriptions | % of descriptions that differ from root desc |

This runs entirely offline — no LLM needed, pure heuristics.

### 2. Ground-Truth Comparison

Several real sites publish their own `llms.txt`. We generate one for the same site and score ours against the real one.

**Ground truth sources:**
- `https://anthropic.com/llms.txt`
- `https://stripe.com/llms.txt`
- `https://docs.github.com/llms.txt`
- `https://fly.io/llms.txt`

**Scoring dimensions:**

| Dimension | Method |
|---|---|
| URL coverage | % of URLs in ground truth that appear in our output |
| Section alignment | Do our section names match theirs? |
| Description quality | LLM-as-judge: given the page body, which description is better — ours or theirs? Score 0/0.5/1 |
| False positives | URLs we include that they don't (may indicate noise) |

**LLM-as-judge prompt:**
```
Given this page content, which description is more useful for an LLM index?
A: {our description}
B: {ground truth description}
Reply with A, B, or Tie and a one-sentence reason.
```

## Design

**CLI tool.** `cmd/eval/main.go` — takes a URL, runs both modes, scores them, prints a report. Separate from the server, runs offline.

**Output.** Markdown report written to `_eval/` with timestamp — same pattern as debug outputs but structured for comparison.

**Corpus.** `_eval/corpus.yaml` — list of sites with known llms.txt URLs to use as ground truth. Checked in, grows over time.

## Notes

- LLM-as-judge calls cost money — gate behind a flag, run manually
- Heuristic scores are free and fast — run these in CI eventually
- Even without ground truth, the basic vs enhanced comparison on a fixed corpus gives a regression signal when prompt changes are made
