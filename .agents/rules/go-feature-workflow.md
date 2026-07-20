---
trigger: always_on
---

# Mandatory Go feature workflow

Apply these gates whenever writing, modifying, or reviewing Go code.

## Before coding

1. Read `AGENTS.md`, the applicable skills, contracts, and migrations.
2. Trace the affected flow end to end, including success, failure, retry,
   cancellation, and concurrent execution.
3. Search the repository for an existing implementation or convention before
   introducing a new pattern.
4. Identify trust boundaries, required dependencies, transaction boundaries,
   event delivery semantics, and database invariants.
5. Name application dependencies by capability, not transport.
6. Stop for confirmation when a product or architectural decision is unclear.

## Before handoff

1. Review the diff for correctness, security, performance, and project
   conventions.
2. Check for typed nils, swallowed errors, unmanaged goroutines, unsafe Kafka
   commits, missing unique constraints, and missing indexes.
3. Verify protobuf contracts, camelCase JSON mapping, and CloudEvents envelopes.
4. Run formatting, lint, unit tests, infrastructure integration tests, race
   tests, and the full test suite applicable to the change.
5. Do not add `nolint` directives for issues that can be fixed reasonably.
6. Report every skipped quality gate and its reason; never claim completion
   while a required check is silently omitted.
