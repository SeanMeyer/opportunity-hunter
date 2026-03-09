# Contributing to Opportunity Hunter

## Adding a New Hunt

1. Create a package under `hunts/yourhunt/`
2. Implement the required `Hunt` interface (6 methods):
   - `Name() string`
   - `Init(ctx, lookup) error`
   - `Sources() []core.Source`
   - `DedupeKey(raw RawItem) string`
   - `Evaluator() core.Evaluator`
   - `DefaultSchedule() core.Schedule`

3. Optionally implement any of the 6 optional interfaces:
   - `Grouper` — batch opportunities before evaluation (e.g., weekly grouping)
   - `ReEvaluator` — re-evaluate previously scored opportunities (e.g., weather changes)
   - `Briefer` — group evaluations for notification and synthesize briefings
   - `Expirer` — custom expiration logic (default: expire when StartTime is past)
   - `WebHunt` — custom card rendering and feedback options
   - `NotifyHunt` — custom Discord notification formatting

4. Register your hunt in `cmd/opportunity-hunter/main.go`:
   ```go
   func registeredHunts() []core.Hunt {
       return []core.Hunt{
           &yourhunt.YourHunt{},
           // ...
       }
   }
   ```

5. Add environment variables to `.env.example`

## Writing Sources

Sources implement `core.Source`:
```go
type Source interface {
    Name() string
    Scan(ctx context.Context, region ScanRegion) ([]RawItem, error)
}
```

Map external API responses to `core.RawItem`. Store the original response in `RawJSON`.

## Testing

- Use `testutil.NewTestDB(t)` for in-memory SQLite
- Use `testutil.FakeEvaluator` for configurable evaluation results
- Use `testutil.FakeNotifier` to capture notification actions
- Use `hunts/fake.FakeHunt` for pipeline integration tests
- Run all tests: `make test`

## Code Style

- Standard Go conventions
- `go vet` for linting
- No external test frameworks — stdlib `testing` only
