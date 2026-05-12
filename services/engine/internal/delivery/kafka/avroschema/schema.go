package avroschema

import _ "embed"

//go:embed assets/hand_started.avsc
var handStarted string

//go:embed assets/player_acted.avsc
var playerActed string

//go:embed assets/hand_ended.avsc
var handEnded string

func HandStarted() string {
	return handStarted
}

func PlayerActed() string {
	return playerActed
}

func HandEnded() string {
	return handEnded
}
