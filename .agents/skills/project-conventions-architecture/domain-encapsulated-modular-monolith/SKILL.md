---
name: domain-encapsulated-modular-monolith
description: "Code under `internal/` must be structured into self-contained, high-cohesion business modules. Each module is responsible for encapsulating its own data schemas, data access layers, and business lo..."
---

# Domain-Encapsulated Modular Monolith

- Code under `internal/` must be structured into self-contained, high-cohesion business modules. Each module is responsible for encapsulating its own data schemas, data access layers, and business logic, minimizing cross-module coupling.
- Dependencies must point inward: Domain must not depend on infrastructure. Application defines interfaces, not implementations.
