package domain

type RakeConfig struct {
	Percent      float64
	Cap          int
	NoFlopNoDrop bool
}

func CalculateRake(pots []Pot, street Street, config RakeConfig) int{
	if config.NoFlopNoDrop && street == StreetPreflop {
		return 0
	}

	total := Total(pots)
	if total == 0 {
		return 0
	}

	rake := int(float64(total) * config.Percent)
	if rake > config.Cap {
		rake = config.Cap
	}

	return rake
}

func ApplyRake(pots []Pot, rake int) []Pot {
	if rake == 0 || len(pots) == 0 {
		return pots
	}

	result := make([]Pot, len(pots))
	copy(result, pots)

	remaining := rake
	for i := len(result) - 1; i >= 0 && remaining > 0; i-- {
		deduct := remaining
		if deduct > result[i].Amount {
			deduct = result[i].Amount
		}
		result[i].Amount -= deduct
		remaining -= deduct
	}

	return result
}