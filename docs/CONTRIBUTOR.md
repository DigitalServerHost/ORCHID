# Project ORCHID Contributors

Thank you for your interest in Project **ORCHID**, the low-level micro-architectural execution core of the RAMNET protocol.

---

## 🏛️ Project Structure & Credits

### Originator

- **Teppei Oohira / 大平鉄兵** ([@gatchimuchio](https://github.com/gatchimuchio))
  - _Role:_ Designed and coded the original CPU cache-line saturation proofs, micro-kernel assembly code generation modules, and deterministic parallel memory role-scheduling models. His research on loop optimization (`I-K-J` reordering) and explicitly routing input vs. output roles formed the mathematical basis of Project ORCHID.

### Project Lead & Maintainer

- **Kevin West** ([@westkevin12](https://github.com/westkevin12))
  - _Role:_ Leads the overall architecture, software engineering refactoring, systems integration, and manages the execution roadmap. Kevin is overseeing the migration of the scheduler and compiler engine to highly concurrent Golang for nodes, while maintaining the Python client SDK.

---

## 🤝 Contribution Guidelines

All contributions to ORCHID must adhere to high-performance, low-latency micro-architectural design principles:

1.  **Strict Performance Budgets:** Code modifications must prioritize zero-overhead and avoid memory allocation garbage collector overhead in the critical loop path.
2.  **Explicit Memory Role Alignment:** Compute scheduling logic must strictly separate memory access types (`READ` vs. `WRITE` streams) using the CADENCE parallel bank model.
3.  **Dynamic Assembly Integrity:** Ensure any custom assembly code generators output clean, validated x86-64 or ARM assembly utilizing optimal CPU-cache contiguous layouts.
4.  **Licensing:** All submitted code must be licensed under the project's **GNU GPLv3 License**.

---

_"Intelligence requires every available joule."_
