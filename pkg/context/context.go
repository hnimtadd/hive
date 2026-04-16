package context

import "context"

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const budgetKey contextKey = "hive_context_budget"

// ContextWithBudget adds a ContextBudget to the context.
func ContextWithBudget(ctx context.Context, budget *ContextBudget) context.Context {
	return context.WithValue(ctx, budgetKey, budget)
}

// BudgetFromContext retrieves a ContextBudget from the context.
// Returns the budget and true if found, nil and false otherwise.
func BudgetFromContext(ctx context.Context) (*ContextBudget, bool) {
	budget, ok := ctx.Value(budgetKey).(*ContextBudget)
	return budget, ok
}
