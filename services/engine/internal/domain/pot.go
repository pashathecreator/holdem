package domain

type Pot struct {
	Amount   int
	Eligible []PlayerID
}

func Calculate(contributions map[PlayerID]int) []Pot {
	if len(contributions) == 0 {
		return nil
	}

	remaining := make(map[PlayerID]int, len(contributions))
	for id, amount := range contributions {
		if amount > 0 {
			remaining[id] = amount
		}
	}

	var pots []Pot

	for len(remaining) > 0 {
		minContrib := -1
		for _, amount := range remaining {
			if minContrib == -1 || amount < minContrib {
				minContrib = amount
			}
		}

		eligible := make([]PlayerID, 0, len(remaining))
		for id := range remaining {
			eligible = append(eligible, id)
		}

		pots = append(pots, Pot{
			Amount:   minContrib * len(remaining),
			Eligible: eligible,
		})

		for id := range remaining {
			remaining[id] -= minContrib
			if remaining[id] == 0 {
				delete(remaining, id)
			}
		}
	}

	return pots
}

func Total(pots []Pot) int {
	var total int
	for _, p := range pots {
		total += p.Amount
	}
	return total
}
