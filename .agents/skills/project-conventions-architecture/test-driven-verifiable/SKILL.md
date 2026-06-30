---
name: test-driven-verifiable
description: "Every new feature must include:"
---

# Test-Driven & Verifiable

- Every new feature must include:
  - Unit Tests covering the Domain and Application layers.
  - Integration Tests covering the Infrastructure layer.
- All test suites must be fully automated, executable, and verifiable to confirm implementation correctness.
- Always maintain or improve statement coverage (recommended targets: >80% for core business logic, >90% for critical middleware/helpers). Never regress test coverage when modifying or refactoring code.
