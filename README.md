# EICL - Efficiency Is Clever Laziness

**EICL** is a fast, modular, and lightweight system profiler and benchmarking suite built for developers, engineers, and performance nerds who want to *stop guessing and start measuring*. It offers a unified interface for profiling across CPU, GPU, I/O, network, memory, and runtime metrics — with extensibility and no bloat.

> Think `perf`, but cross-platform. Think `htop`, but smarter. Think flame graphs, call trees, treemaps, and daemonized insights — all in one place.

## 🚀 Vision

EICL will eventually support all major operating systems and platforms, starting with:

- ✅ Linux
- ✅ Windows  
- ⏳ macOS, BSD, and others (planned)

Long-term goals include:

- Hardware resource monitoring across platforms
- CPU & GPU memory profiling
- I/O and network profiling
- Runtime profiling & benchmarking
- Ray tracing microbenchmarks
- Rich visualization (flame graphs, sunburst diagrams, treemaps, etc.)
- Daemon mode for continuous service/application monitoring
- CLI-first, no bullshit UX

## 🛠️ Tech Stack

| Component       | Tech                          |
|----------------|-------------------------------|
| Core System     | Go                            |
| GPU Profiling   | Python + Triton/CUDA + HIP    |
| Visualization   | Web (TBD), CLI Graphing (Go)  |
| Testing         | Go test + dedicated integration folder |

## 📂 Project Structure (WIP)
```text
eicl/
├── bench/                 # Benchmarking modules (CPU, GPU, Mem, I/O, Net)
│   ├── cpu/
│   ├── gpu/
│   ├── io/
│   ├── mem/
│   ├── network/
│   └── bench_runner.go    # Entrypoint for executing benchmarks
│
├── profiler/              # Profiling infrastructure (not benchmarking)
│   ├── cpu/
│   ├── gpu/
│   ├── io/
│   ├── mem/
│   ├── network/
│   └── profiler_api.go    # Core profiler abstraction/API
│
├── cmd/                   # CLI Entrypoint
│   └── main.go
│
├── docs/                  # Contribution + design docs
│   ├── contrib.md
│   └── main.md
│
├── README.md
├── LICENSE
├── go.mod
└── go.sum
```
## 🧪 Testing Guidelines

- Use `*_test.go` files in the same package for unit tests
- For integration/E2E tests, use `tests/`
- Follow [JetBrains Go Testing Guide](https://blog.jetbrains.com/go/2022/11/22/comprehensive-guide-to-testing-in-go/)
- Use logs at test start/end
- No continuous logs in polling loops

## 🔐 Commit Guidelines

- All commits must be signed (`git commit -S`)
- No emojis
- All code changes must go through PRs to `staging-main`
- No merges from `staging-main` to feature branches — only rebase
- Format using `gofmt` before push
- Precede all exported types/functions with godoc comments

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 👨‍🔧 Core Maintainers

- Sri Guru Datta Pisupati (@pupperemeritus)
- A select few (you know who you are)

## ⚠️ Status

Under active development. No guarantees, no warranties. You break it, you fix it.

---

_“Efficiency is clever laziness.” — Not Larry Wall but it should’ve been._
