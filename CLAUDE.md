# opportunity-hunter Development Guidelines

## Active Technologies
- Go 1.24+ with `google.golang.org/genai` (Gemini), `modernc.org/sqlite`, `gocolly/colly` (scraping)
- SQLite via `modernc.org/sqlite` -- single file, pure Go, no CGO

## Project Structure

```text
cmd/opportunity-hunter/  -- CLI entrypoint (run, scan, eval, web, version)
core/                    -- Universal types & interfaces (Opportunity, Hunt, Evaluation, Pick)
pipeline/                -- Framework orchestration (11-step pipeline)
config/                  -- Environment variable parsing
storage/                 -- SQLite database layer (opportunities, venues, evaluations, picks, feedback)
web/                     -- Shared web UI shell (tabs per hunt, cards, feedback, status)
notify/                  -- Discord client (thread-aware + error alerts)
llm/                     -- Gemini two-step client
distance/                -- Walking/driving distance cache
hunts/
  fake/                  -- Test double implementing all interfaces
  comedy/                -- Comedy hunt (Hunt + Grouper + WebHunt + NotifyHunt)
  performing/            -- Performing arts hunt (Hunt + Grouper + WebHunt + NotifyHunt)
  movies/                -- Movies hunt (Hunt + Expirer + WebHunt + NotifyHunt)
  powder/                -- Powder hunt (Hunt + ReEvaluator + Briefer + WebHunt + NotifyHunt)
testutil/                -- Shared test infrastructure (NewTestDB, FakeEvaluator, FakeNotifier)
```

## Architecture

Framework pipeline with Go interface plugin pattern. Core owns orchestration; hunts implement
a required interface (6 methods) plus optional interfaces. Sequential hunt execution, concurrent
source scanning within each hunt. SQLite persistence, Gemini LLM evaluation, Discord notifications.

## Key Commands

```bash
make build    # Build binary
make test     # Run all tests
make lint     # Go vet
make run      # Run daemon

go run ./cmd/opportunity-hunter/ scan    # One-shot scan
go run ./cmd/opportunity-hunter/ eval    # One-shot evaluation
go run ./cmd/opportunity-hunter/ web     # Web UI only
```

## Design Doc

See `docs/plans/2026-03-08-opportunity-hunter-design.md` for full architecture details.
