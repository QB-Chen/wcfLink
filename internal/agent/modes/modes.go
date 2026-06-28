package modes

type ModeConfig struct {
	Slug           string
	Name           string
	SystemPrompt   string
	AvailableTools []string
	WelcomeMessage string
}

var Registry = map[string]ModeConfig{
	"icemark":   IcemarkMode,
	"market":    MarketMode,
	"prd":       PRDMode,
	"prototype": PrototypeMode,
	"support":   SupportMode,
}

func Get(slug string) (ModeConfig, bool) {
	m, ok := Registry[slug]
	return m, ok
}

func DefaultMode() string {
	return "icemark"
}
