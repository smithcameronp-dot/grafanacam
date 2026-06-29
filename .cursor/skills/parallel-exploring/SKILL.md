---
name: parallel-exploring
description: Explore a large codebase in parallel by launching multiple explore subagents that each investigate a different area simultaneously. Use when onboarding onto a new project, understanding architecture, or investigating a cross-cutting concern.
---

# Parallel Explore

Use this skill when you need to understand a large or unfamiliar codebase quickly — onboarding onto a new project, investigating how a feature works across layers, or mapping the architecture.

## How It Works

Cursor's `explore` subagent is a fast, read-only agent optimized for searching and reading code. You can launch multiple explore agents in a single message and they run concurrently, each investigating a different area.

## Steps

1. **Identify the areas to explore** — break the codebase into logical zones. For this Grafana monorepo:

   | Zone | Directories | What to report |
   | --- | --- | --- |
   | Frontend | `public/app/features/`, `public/app/core/`, `public/app/store/` | Redux/RTK Query patterns, routing, feature domains |
   | Backend | `pkg/api/`, `pkg/services/`, `pkg/server/` | Wire DI, service boundaries, API handlers vs business logic |
   | Shared packages | `packages/` (`@grafana/data`, `@grafana/ui`, `@grafana/runtime`, `@grafana/scenes`) | Reusable types, components, runtime services |
   | Apps / plugins | `apps/`, `public/app/plugins/` | App SDK apps, built-in datasource/panel plugins |
   | Infrastructure | `.github/`, `Makefile`, `conf/`, `devenv/` | CI, build targets, config defaults |
   | Tests | `e2e-playwright/`, co-located `*_test.go` / `*.test.tsx` | E2E vs unit test conventions |

   When exploring a zone, check for directory-scoped agent files first:

   - `docs/AGENTS.md`
   - `public/app/features/alerting/unified/AGENTS.md`
   - `pkg/storage/unified/AGENTS.md`
   - Feature-local files like `e2e-playwright/dashboard-new-layouts/AGENTS.md`

2. **Launch parallel explore agents** — use the Task tool with `subagent_type: "explore"` for each area. Launch them all in one message:

   ```
   Task 1: "Explore the Grafana frontend — find the main feature domains, routing setup,
            state management approach, and UI component library. Check public/app/features/,
            public/app/core/, public/app/store/. Report the framework, router, styling approach,
            and key entry points."

   Task 2: "Explore the Grafana backend — find the API handlers, service layer, Wire DI setup,
            and database access. Check pkg/api/, pkg/services/, pkg/server/. Report the framework,
            service boundaries, auth strategy, and key endpoints."

   Task 3: "Explore the Grafana infrastructure — find CI/CD config, build targets, deployment
            config, and environment variable management. Check .github/, Makefile, conf/, devenv/.
            Report the CI provider, build commands, and config defaults."
   ```

3. **Synthesize the results** — when all agents return, combine their findings into a coherent picture:

   - Tech stack summary (Go backend, React/TS frontend, Yarn workspaces, Wire DI)
   - Architecture overview (data flow: frontend → API → services → sqlstore/tsdb)
   - Key entry points (`pkg/server/wire.go`, `public/app/`, `Makefile`)
   - Directory-scoped `AGENTS.md` files relevant to the investigation
   - Potential concerns or tech debt (e.g. unified storage compatibility rules in `pkg/storage/unified/`)

## Other Use Cases

- **Cross-cutting investigation**: "Where is user authentication checked?" — launch agents to search the frontend (route guards), backend (`pkg/middleware/`, `pkg/services/auth/`), and database (session storage) simultaneously.
- **Dependency audit**: launch agents to check different parts of the dependency tree for outdated packages, security issues, and unused imports.
- **Migration planning**: have agents simultaneously assess the frontend, backend, and tests to estimate the scope of a framework migration.

## Notes

- Explore agents are read-only — they can't modify files.
- Use `thoroughness: "very thorough"` in the prompt for comprehensive analysis.
- Each agent has its own context window, so they can each read many files without running out of space.
- For a single focused question, just use Grep or Read directly — subagents are for broad exploration.
