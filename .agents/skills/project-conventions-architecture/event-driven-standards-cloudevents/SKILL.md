---
name: event-driven-standards-cloudevents
description: "If event-driven messaging is introduced, all event envelopes must adhere strictly to the **CloudEvents** specification. The `data` payload of these events must follow the camelCase JSON mapping sta..."
---

# Event-Driven Standards (CloudEvents)

- If event-driven messaging is introduced, all event envelopes must adhere strictly to the **CloudEvents** specification. The `data` payload of these events must follow the camelCase JSON mapping standard, while envelope-level extension attributes must remain all-lowercase.
