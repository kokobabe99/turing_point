#!/usr/bin/env bash

echo "========================"
echo " Running ALL automata "
echo "========================"

echo
echo ">>> TWA"
go run main.go twa rules/twa.txt "#abba#"

echo
echo ">>> TM"
go run main.go tm rules/tm.txt "#abba#"

echo
echo ">>> PDA"
go run main.go pda rules/pda.txt "#a#"

echo
echo ">>> 2PDA"
go run main.go 2pda rules/2pda.txt "#abba#"

echo
echo ">>> Transducer"
go run main.go transducer rules/trans.txt "#abba#"

echo
echo "========================"
echo " ALL DONE "
echo "========================"