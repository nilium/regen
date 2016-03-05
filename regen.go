package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"regexp/syntax"
)

// CLI options

var verbose bool
var unboundMax = 32

func randint(max int) int {
	if max == 0 {
		return 0
	}
	var bigmax big.Int
	bigmax.SetInt64(int64(max))
	res, err := rand.Int(rand.Reader, &bigmax)
	if err != nil {
		panic(err)
	}
	return int(res.Int64())
}

func GenString(w *bytes.Buffer, rx *syntax.Regexp) (err error) {
	switch rx.Op {
	case syntax.OpNoMatch:
		return
	case syntax.OpEmptyMatch:
		return
	case syntax.OpLiteral:
		w.WriteString(string(rx.Rune))
	case syntax.OpCharClass:
		nth := randint(len(rx.Rune)) &^ 1
		min, max := rx.Rune[nth], rx.Rune[nth+1]
		ch := min + rune(randint(int(max-min)))
		w.WriteRune(ch)
	case syntax.OpAnyCharNotNL:
		w.WriteRune(rune(' ' + randint(95)))
	case syntax.OpAnyChar:
		i := randint(96)
		ch := rune(' ' + randint(96))
		if i == 95 {
			ch = '\n'
		}
		w.WriteRune(ch)
	case syntax.OpBeginLine:
		if w.Len() != 0 {
			w.WriteByte('\n')
		}
	case syntax.OpEndLine:
		if w.Len() != 0 {
			w.WriteByte('\n')
		} else {
			return io.EOF
		}
	case syntax.OpBeginText:
	case syntax.OpEndText:
		return io.EOF
	case syntax.OpWordBoundary:
		fallthrough
	case syntax.OpNoWordBoundary:
		panic("regen: word boundaries not supported yet")
	case syntax.OpStar, syntax.OpPlus:
		min := 0
		if rx.Op == syntax.OpPlus {
			min = 1
		}
		max := min + unboundMax

		for sz := randint(max - min); sz > 0; sz-- {
			for _, rx := range rx.Sub {
				GenString(w, rx)
			}
		}
	case syntax.OpQuest:
		if randint(1024)&1 == 1 {
			for _, rx := range rx.Sub {
				if err := GenString(w, rx); err != nil {
					return err
				}
			}
		}
	case syntax.OpRepeat:
		min := rx.Min
		max := rx.Max
		if max == -1 {
			max = min + unboundMax
		}
		for sz := min + randint(max-min); sz > 0; sz-- {
			for _, rx := range rx.Sub {
				if err := GenString(w, rx); err != nil {
					return err
				}
			}
		}

	case syntax.OpConcat, syntax.OpCapture:
		for _, rx := range rx.Sub {
			if err := GenString(w, rx); err != nil {
				return err
			}
		}
	case syntax.OpAlternate:
		nth := randint(len(rx.Sub))
		return GenString(w, rx.Sub[nth])
	}

	return nil
}

func main() {
	log.SetPrefix("regen: ")
	log.SetFlags(0)

	simplify := flag.Bool("simplify", false, "Whether to simplify the parsed regular expressions. This can produce chains of OpQuest in place of OpRepeat, which are harder to randomize.")

	flag.IntVar(&unboundMax, "max", unboundMax, "The max `repetitions` to use for unlimited repetitions/matches.")
	flag.Parse()

	if flag.NArg() != 1 {
		log.Println("no regexp given")
		return
	}

	regexen := make([]*syntax.Regexp, flag.NArg())
	for i, s := range flag.Args() {
		var err error
		regexen[i], err = syntax.Parse(s, 0)

		if err != nil {
			log.Printf("error parsing regular expression %q:\n%v", s, err)
			os.Exit(1)
		}

		if *simplify {
			regexen[i] = regexen[i].Simplify()
		}
	}

	var b bytes.Buffer
	for i, rx := range regexen {
		if i > 0 {
			fmt.Print("\n")
			b.Reset()
		}

		err := GenString(&b, rx)
		if err != nil && err != io.EOF {
			log.Printf("Error generating string: %v", err)
			os.Exit(1)
		}
		fmt.Print(b.String())
	}

	if isTTY() {
		fmt.Print("\n")
	}
}

// isTTY attempts to determine whether the current stdout refers to a terminal.
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		log.Println("Error getting Stat of os.Stdout:", err)
		return true // Assume human readable
	}
	return (fi.Mode() & os.ModeNamedPipe) != os.ModeNamedPipe
}
