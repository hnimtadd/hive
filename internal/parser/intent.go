package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// Intent represents the parsed intent from a natural language command
type Intent struct {
	Goal        string            `json:"goal"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	JiraID      string            `json:"jira_id"`
	ActionType  string            `json:"action_type"`
	Target      string            `json:"target"`
	Context     map[string]string `json:"context"`
	Urgency     string            `json:"urgency"`
}

// ActionType constants for common development tasks
const (
	ActionTypeUpdate   = "update"
	ActionTypeFix      = "fix"
	ActionTypeAdd      = "add"
	ActionTypeCreate   = "create"
	ActionTypeRefactor = "refactor"
	ActionTypeTest     = "test"
	ActionTypeDeploy   = "deploy"
	ActionTypeDebug    = "debug"
	ActionTypeOptimize = "optimize"
	ActionTypeDocument = "document"
)

// UrgencyLevel constants
const (
	UrgencyLow      = "low"
	UrgencyMedium   = "medium"
	UrgencyHigh     = "high"
	UrgencyCritical = "critical"
)

// ParseIntent analyzes natural language input and extracts structured intent
func ParseIntent(command, jiraID string) (*Intent, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	intent := &Intent{
		Goal:    command,
		JiraID:  jiraID,
		Context: make(map[string]string),
		Urgency: UrgencyMedium, // default urgency
	}

	// Normalize the command for parsing
	normalizedCommand := strings.ToLower(strings.TrimSpace(command))

	// Extract action type using regex patterns
	intent.ActionType = extractActionType(normalizedCommand)

	// Extract target/subject of the action
	intent.Target = extractTarget(normalizedCommand, intent.ActionType)

	// Generate a title (first 50 chars or until first period)
	intent.Title = generateTitle(command)

	// Set description as the full command for now
	intent.Description = command

	// Extract urgency indicators
	intent.Urgency = extractUrgency(normalizedCommand)

	// Extract additional context
	extractContext(normalizedCommand, intent)

	return intent, nil
}

// extractActionType identifies the primary action from the command
func extractActionType(command string) string {
	actionPatterns := map[string][]string{
		ActionTypeUpdate: {
			`update.*`, `modify.*`, `change.*`, `alter.*`, `revise.*`,
		},
		ActionTypeFix: {
			`fix.*`, `repair.*`, `resolve.*`, `correct.*`, `debug.*`,
		},
		ActionTypeAdd: {
			`add.*`, `include.*`, `insert.*`, `append.*`, `implement.*`,
		},
		ActionTypeCreate: {
			`create.*`, `make.*`, `build.*`, `generate.*`, `new.*`,
		},
		ActionTypeRefactor: {
			`refactor.*`, `restructure.*`, `reorganize.*`, `cleanup.*`,
		},
		ActionTypeTest: {
			`test.*`, `validate.*`, `verify.*`, `check.*`,
		},
		ActionTypeDeploy: {
			`deploy.*`, `release.*`, `publish.*`, `ship.*`,
		},
		ActionTypeOptimize: {
			`optimize.*`, `improve.*`, `enhance.*`, `performance.*`,
		},
		ActionTypeDocument: {
			`document.*`, `comment.*`, `explain.*`, `describe.*`,
		},
	}

	for actionType, patterns := range actionPatterns {
		for _, pattern := range patterns {
			matched, _ := regexp.MatchString(pattern, command)
			if matched {
				return actionType
			}
		}
	}

	// Default to update if no specific action is detected
	return ActionTypeUpdate
}

// extractTarget attempts to identify the main subject/target of the action
func extractTarget(command, actionType string) string {
	// Common target patterns
	targetPatterns := []string{
		`(script|file|function|method|class|component|service|handler|module|package)`,
		`(api|endpoint|route|controller|middleware)`,
		`(database|table|schema|query|migration)`,
		`(test|spec|unit test|integration test)`,
		`(config|configuration|setting|parameter)`,
		`(ui|interface|page|form|button|component)`,
		`(authentication|auth|login|security)`,
		`(payment|billing|checkout|transaction)`,
	}

	for _, pattern := range targetPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(command)
		if len(matches) > 0 {
			return matches[0]
		}
	}

	// If no specific target found, extract the first noun after the action
	words := strings.Fields(command)
	if len(words) > 1 {
		// Skip common articles and prepositions
		skipWords := map[string]bool{
			"the": true, "a": true, "an": true, "to": true, "for": true,
			"with": true, "in": true, "on": true, "at": true,
		}

		for i := 1; i < len(words) && i < 5; i++ { // Check first few words
			word := strings.ToLower(words[i])
			if !skipWords[word] && len(word) > 2 {
				return word
			}
		}
	}

	return "code"
}

// generateTitle creates a concise title from the command
func generateTitle(command string) string {
	// Take first 50 characters or until first period
	title := command
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	// Find first sentence ending
	if idx := strings.Index(title, "."); idx > 0 && idx < 50 {
		title = title[:idx]
	}

	return strings.TrimSpace(title)
}

// extractUrgency determines urgency level from language cues
func extractUrgency(command string) string {
	criticalWords := []string{"urgent", "critical", "emergency", "asap", "immediately", "now"}
	highWords := []string{"important", "priority", "soon", "quickly", "fast"}
	lowWords := []string{"whenever", "eventually", "minor", "small", "trivial"}

	for _, word := range criticalWords {
		if strings.Contains(command, word) {
			return UrgencyCritical
		}
	}

	for _, word := range highWords {
		if strings.Contains(command, word) {
			return UrgencyHigh
		}
	}

	for _, word := range lowWords {
		if strings.Contains(command, word) {
			return UrgencyLow
		}
	}

	return UrgencyMedium
}

// extractContext pulls additional context information from the command
func extractContext(command string, intent *Intent) {
	// Extract file extensions or programming languages
	langPatterns := map[string]string{
		`\.go\b|golang|go\s`:              "go",
		`\.js\b|javascript|node`:          "javascript",
		`\.py\b|python`:                   "python",
		`\.java\b|java\s`:                 "java",
		`\.ts\b|typescript`:               "typescript",
		`\.sql\b|database|db`:             "sql",
		`\.yaml\b|\.yml\b|yaml`:           "yaml",
		`\.json\b|json`:                   "json",
		`docker|container|k8s|kubernetes`: "docker",
	}

	for pattern, lang := range langPatterns {
		matched, _ := regexp.MatchString(pattern, command)
		if matched {
			intent.Context["language"] = lang
			break
		}
	}

	// Extract technology/framework mentions
	techPatterns := map[string]string{
		`react|jsx|tsx`:       "react",
		`vue|vuejs`:           "vue",
		`angular`:             "angular",
		`spring|springboot`:   "spring",
		`django|flask`:        "python-web",
		`express|expressjs`:   "express",
		`gin|fiber|echo`:      "go-web",
		`redis|cache`:         "redis",
		`postgres|postgresql`: "postgresql",
		`mysql`:               "mysql",
		`mongodb|mongo`:       "mongodb",
	}

	for pattern, tech := range techPatterns {
		matched, _ := regexp.MatchString(pattern, command)
		if matched {
			intent.Context["technology"] = tech
		}
	}

	// Extract version numbers or page ratios (like "0:100")
	versionRegex := regexp.MustCompile(`(\d+:\d+|\d+\.\d+\.\d+|v\d+\.\d+)`)
	if matches := versionRegex.FindAllString(command, -1); len(matches) > 0 {
		intent.Context["version"] = matches[0]
	}
}

