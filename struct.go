package main

import "fmt"

type Move int8

const (
	L Move = -1
	R Move = +1
)

type StepStatus int

const (
	Continue StepStatus = iota
	Accept
	Reject
)

type State struct {
	id  int
	dir Move
	//onHash *State
	//onA    *State
	//onB    *State

	next   map[uint8]*State
	accept bool
	reject bool
}

// 选边的小工具
func (s *State) nextOn(sym byte) (*State, error) {

	if state, ok := s.next[sym]; ok {

		return state, nil
	} else {
		return nil, fmt.Errorf("invalid symbol %q", sym)
	}
	
}

func (s *State) Step(tape string, i int) (*State, int, StepStatus, error) {

	displayTapeWithHead(tape, i)

	nxt, err := s.nextOn(tape[i])
	if err != nil {
		return nil, i, Continue, err
	}
	if nxt == nil {
		return nil, i, Continue, fmt.Errorf("missing transition: state %d on %q", s.id, tape[i])
	}
	if nxt.accept {
		return nxt, i, Accept, nil
	}
	if nxt.reject {
		return nxt, i, Reject, nil
	}
	if nxt.dir == L {
		i--
	} else {
		i++
	}
	return nxt, i, Continue, nil
}

type rawLine struct {
	id    int
	dir   Move
	pairs [][2]string
	acc   bool
	rej   bool
}
