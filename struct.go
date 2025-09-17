package main

type Move int8

const (
	L Move = -1
	R Move = +1
)

type State struct {
	id     int
	dir    Move
	onHash *State
	onA    *State
	onB    *State
	accept bool
	reject bool
}

type rawLine struct {
	id    int
	dir   Move
	pairs [][2]string
	acc   bool
	rej   bool
}
