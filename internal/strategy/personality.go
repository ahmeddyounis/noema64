package strategy

func ProfileForPersonality(personality Personality) PersonalityProfile {
	switch personality {
	case PersonalityAggressive:
		return PersonalityProfile{
			ID:            PersonalityAggressive,
			Name:          "Aggressive Attacker",
			RiskTolerance: 0.75,
			StrategicBiases: []string{
				"Prefer active piece play and initiative.",
				"Consider attacks when tactical safety allows.",
			},
			PromptModifiers: []string{
				"Emphasize initiative while obeying legality and verifier constraints.",
			},
		}
	case PersonalityPositional:
		return PersonalityProfile{
			ID:            PersonalityPositional,
			Name:          "Positional",
			RiskTolerance: 0.30,
			StrategicBiases: []string{
				"Prefer structure, prophylaxis, and long-term weaknesses.",
			},
			PromptModifiers: []string{
				"Explain plans through squares, pawn structure, and piece improvement.",
			},
		}
	case PersonalityBeginnerCoach:
		return PersonalityProfile{
			ID:            PersonalityBeginnerCoach,
			Name:          "Beginner Coach",
			RiskTolerance: 0.25,
			StrategicBiases: []string{
				"Prefer clear principled moves and simple explanations.",
			},
			PromptModifiers: []string{
				"Use beginner-friendly language and distinguish strategy from tactics.",
			},
		}
	default:
		return PersonalityProfile{
			ID:            PersonalityBalanced,
			Name:          "Balanced",
			RiskTolerance: 0.45,
			StrategicBiases: []string{
				"Prefer development, central control, and king safety.",
			},
			PromptModifiers: []string{
				"Keep candidate explanations concise and practical.",
			},
		}
	}
}
