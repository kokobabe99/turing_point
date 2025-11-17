package main

import (
	"fmt"
	"time"
)

/* ===================== 1-stack PDA Machine ===================== */

type PDAMachine struct {
	states []*State
	start  *State
}

func NewPDAMachine(states []*State, start *State) *PDAMachine {
	return &PDAMachine{states: states, start: start}
}

func (m *PDAMachine) Kind() MachineKind { return KindPDA }

func (m *PDAMachine) Dump() { dumpStates(m.states) }

func (m *PDAMachine) WriteDOT(path string) error {
	// PDA 不显示方向，只显示动作
	return writeDOTCommon(m.states, path, false)
}

func (m *PDAMachine) Run(tape []byte) (bool, error) {
	q := m.start
	head := 1
	stack := []byte(nil)
	step := 1

	fmt.Println("== TRACE START ==")
	for {
		if head < 0 || head >= len(tape) {
			return false, fmt.Errorf("head out of bounds: %d", head)
		}
		displayTapeWithHead(string(tape), head)
		cur := tape[head]

		nxt, ok := q.next[cur]
		if !ok {
			// 无转移：reject
			return false, nil
		}

		// 执行动作（只用 Stack1）
		switch q.action {
		case ActPush1:
			if q.stackSym == 0 || cur == q.stackSym {
				stack = append(stack, q.stackSym)
			}
		case ActPop1:
			// 在 '#' 上不 pop，让 empty-stack 接受来判断
			if cur != '#' {
				if len(stack) == 0 {
					return false, fmt.Errorf("PDA stack underflow at state %d", q.id)
				}
				top := stack[len(stack)-1]
				if q.stackSym != 0 && top != q.stackSym {
					return false, fmt.Errorf("PDA stack unexpected top %q at state %d", top, q.id)
				}
				stack = stack[:len(stack)-1]
			}
		}

		fmt.Printf("step  state       read  next  head\n")
		fmt.Printf("%-5d %-10s  %-4s  %-4d  %d\n",
			step,
			fmt.Sprintf("%d", q.id),
			string(cur),
			nxt.id,
			head,
		)

		// 头只向右移动（one-way），遇到 '#' 可以不动
		if cur != '#' {
			head++
		}

		if nxt.accept {
			// empty-stack accept
			if len(stack) == 0 {
				return true, nil
			}
			return false, nil
		}
		if nxt.reject {
			return false, nil
		}

		q = nxt
		step++
		time.Sleep(300 * time.Millisecond)
	}
}
