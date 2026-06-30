---
name: contracts-folder-as-the-ssot
description: "The `contracts/` folder is the Single Source of Truth (SSOT) for the entire application interface. All gRPC services, REST gateway mappings, and OpenAPI documentation are generated and derived dire..."
---

# Contracts Folder as the SSOT

- The `contracts/` folder is the Single Source of Truth (SSOT) for the entire application interface. All gRPC services, REST gateway mappings, and OpenAPI documentation are generated and derived directly from the schemas defined here. Implementing manual HTTP routing, REST controller mappings, or writing manual OpenAPI specs is prohibited.
- API security requirements and authentication bypasses must be declared directly in the proto contract using gRPC-Gateway OpenAPI annotations (referencing `"BearerAuth"`). Manual bypass lists or hardcoded endpoint arrays in Go interceptors are prohibited.
