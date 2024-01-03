package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
)

func TestMinuteCastTimings(t *testing.T) {
	var cast MinuteCast
	castData, err := os.ReadFile("./last_update.json")
	assert.NoError(t, err)
	err = json.Unmarshal(castData, &cast)
	assert.NoError(t, err)
	/*
		Stored pattern:
			* now -> 10min Clear => expect SOON with start 10min end 65min
			* 66min -> 67min Clear => expect SOON with start 68 end 72
			* 72min -> 119min Clear => expect CLEAR with start 0 end 0

	*/
	cast.UpdateTime = time.Now()

	state := getStateFromCast(cast)
	t.Log(state)
	assert.Equal(t, "SOON", state.Weather)
	assert.Equal(t, 10, state.RainStart)
	assert.Equal(t, 65, state.RainEnd)

	cast.UpdateTime = time.Now().Add(time.Minute * -12)
	state = getStateFromCast(cast)
	t.Log(state)
	assert.Equal(t, "RAIN", state.Weather)
	assert.Equal(t, 0, state.RainStart)
	assert.Equal(t, 53, state.RainEnd)

	cast.UpdateTime = time.Now().Add(time.Minute * -67)
	state = getStateFromCast(cast)
	t.Log(state)
	assert.Equal(t, "SOON", state.Weather)
	assert.Equal(t, 1, state.RainStart)
	assert.Equal(t, 5, state.RainEnd)

	cast.UpdateTime = time.Now().Add(time.Minute * -69)
	state = getStateFromCast(cast)
	t.Log(state)
	assert.Equal(t, "RAIN", state.Weather)
	assert.Equal(t, 0, state.RainStart)
	assert.Equal(t, 3, state.RainEnd)

	cast.UpdateTime = time.Now().Add(time.Minute * -74)
	state = getStateFromCast(cast)
	t.Log(state)
	assert.Equal(t, "CLEAR", state.Weather)
	assert.Equal(t, 0, state.RainStart)
	assert.Equal(t, 0, state.RainEnd)
}
