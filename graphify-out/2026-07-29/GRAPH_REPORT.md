# Graph Report - .  (2026-07-29)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 16 nodes · 15 edges · 4 communities (2 shown, 2 thin omitted)
- Extraction: 73% EXTRACTED · 27% INFERRED · 0% AMBIGUOUS · INFERRED: 4 edges (avg confidence: 0.78)
- Token cost: 156 input · 511 output

## Community Hubs (Navigation)
- Iterative Improvement Loop
- MVU Architecture
- Go Main Program
- Today Repository

## God Nodes (most connected - your core abstractions)
1. `Model` - 6 edges
2. `Iterative Improvement Loop Process` - 4 edges
3. `PersonData()` - 2 edges
4. `main()` - 2 edges
5. `Iterative Improvement Loop` - 2 edges
6. `Review Subagent` - 1 edges
7. `Fix Subagent` - 1 edges
8. `Git Safety Net` - 1 edges
9. `Ralph Loop Local Config` - 1 edges
10. `github.com/xieguaiwu/Today` - 0 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `PersonData()`  [INFERRED]
  main.go → internal/model.go
- `Ralph Loop Local Config` --references--> `Iterative Improvement Loop Process`  [INFERRED]
  .omo/ralph-loop.local.md → internal/improvement-loop.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Iterative Improvement Loop Flow** — internal_improvement_loop_iterative_improvement_loop, internal_improvement_loop_review_subagent, internal_improvement_loop_fix_subagent [INFERRED 0.75]

## Communities (4 total, 2 thin omitted)

### Community 0 - "Iterative Improvement Loop"
Cohesion: 0.33
Nodes (6): Ralph Loop Local Config, Git Safety Net, Iterative Improvement Loop, Fix Subagent, Iterative Improvement Loop Process, Review Subagent

### Community 1 - "MVU Architecture"
Cohesion: 0.47
Nodes (3): Cmd, Model, Msg

## Knowledge Gaps
- **5 isolated node(s):** `github.com/xieguaiwu/Today`, `Review Subagent`, `Fix Subagent`, `Git Safety Net`, `Ralph Loop Local Config`
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Model` connect `MVU Architecture` to `Go Main Program`?**
  _High betweenness centrality (0.190) - this node is a cross-community bridge._
- **Why does `PersonData()` connect `Go Main Program` to `MVU Architecture`?**
  _High betweenness centrality (0.114) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `Iterative Improvement Loop Process` (e.g. with `Ralph Loop Local Config` and `Fix Subagent`) actually correct?**
  _`Iterative Improvement Loop Process` has 3 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/xieguaiwu/Today`, `Review Subagent`, `Fix Subagent` to the rest of the system?**
  _5 weakly-connected nodes found - possible documentation gaps or missing edges._