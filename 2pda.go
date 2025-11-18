package main

import (
	"fmt"
	"time"
)

/* ===================== 2-stack PDA Machine ===================== */

type TwoPDAMachine struct {
	states []*State
	start  *State
}

func NewTwoPDAMachine(states []*State, start *State) *TwoPDAMachine {
	return &TwoPDAMachine{states: states, start: start}
}

func (m *TwoPDAMachine) Kind() MachineKind { return KindTwoPDA }

func (m *TwoPDAMachine) Dump() { dumpStates(m.states) }

func (m *TwoPDAMachine) WriteDOT(path string) error {
	// 2PDA：不需要显示方向，只显示动作
	return writeDOTCommon(m.states, path, false)
}

func (m *TwoPDAMachine) Run(tape []byte) (bool, error) {
	q := m.start
	head := 1 // 跳过左边第一个 '#'
	var stack1, stack2 []byte
	step := 1

	fmt.Println("== TRACE START ==")

	for {
		if head < 0 || head >= len(tape) {
			return false, fmt.Errorf("2PDA head out of bounds: %d", head)
		}

		displayTapeWithHead(string(tape), head)
		cur := tape[head]

		// 查转移
		nxt, ok := q.next[cur]
		if !ok {
			// 当前符号没有出边，直接 REJECT
			return false, nil
		}

		// ===== 执行动作（只有 5 种：Scan/Push1/Pop1/Push2/Pop2） =====
		switch q.action {
		case ActPush1:
			// push1：用 stackSym 作为入栈符号，通常 stackSym 是规则里第一个 (sym,...) 的 sym
			if q.stackSym == 0 || cur == q.stackSym {
				stack1 = append(stack1, q.stackSym)
			}
		case ActPop1:
			// 在 '#' 上不 pop，让最后由 empty-stack accept 判断
			if cur != '#' {
				// 栈空或者栈顶不符合预期，都是“输入不合法” => 直接 REJECT
				if len(stack1) == 0 {
					return false, nil
				}
				top := stack1[len(stack1)-1]
				if q.stackSym != 0 && top != q.stackSym {
					return false, nil
				}
				stack1 = stack1[:len(stack1)-1]
			}
		case ActPush2:
			if q.stackSym == 0 || cur == q.stackSym {
				stack2 = append(stack2, q.stackSym)
			}
		case ActPop2:
			if cur != '#' {
				if len(stack2) == 0 {
					return false, nil
				}
				top := stack2[len(stack2)-1]
				if q.stackSym != 0 && top != q.stackSym {
					return false, nil
				}
				stack2 = stack2[:len(stack2)-1]
			}
		case ActScan, ActNone:
			// 不操作栈
		default:
			// 2PDA 不应该出现其他动作
			return false, fmt.Errorf("2PDA: unsupported action %v at state %d", q.action, q.id)
		}

		// 打 trace：把两个栈的长度也打印出来方便 debug
		fmt.Printf("step=%-4d state=%-3d read=%q next=%-3d head=%-2d  |S1|=%d |S2|=%d\n",
			step, q.id, cur, nxt.id, head, len(stack1), len(stack2),
		)

		// ===== 处理接受 / 拒绝 =====
		if nxt.accept {
			// 2-stack empty-stack accept：两个栈都必须空
			if len(stack1) == 0 && len(stack2) == 0 {
				return true, nil
			}
			// 栈没空直接视为 REJECT
			return false, nil
		}
		if nxt.reject {
			return false, nil
		}

		// ===== 头移动逻辑：2PDA 一律 one-way，只在读到非 '#' 时右移 =====
		if cur != '#' {
			head++
		}
		// （如果你想最后一个 '#' 也右移一次，可以改成：if head < len(tape)-1 { head++ }）

		q = nxt
		step++
		time.Sleep(200 * time.Millisecond)
	}
}
