// regen is a tool to parse and generate random strings from regular expressions.
//
// regen works by parsing a regular expression and walking its op tree. It is currently not guaranteed to produce
// entirely accurate results, but will at least try.
//
// Currently, word boundaries are not supported (until I decide how best to randomly insert a word boundary character).
// Using a word boundary op (\b or \B) will currently cause regen to panic.
//
// Usage is simple, run pass one or more regular expressions to regen on the command line and it will generate a string
// from each, printing them in the same order as on the command line (separated by newlines):
//
//      $ regen 'foo(-(bar|baz|quux|woop)){4}'
//      foo-woop-quux-bar-quux
//
// So, if you fancy yourself a Javascript weirdo of some variety, you can at least use regen to write code for eBay:
//
//      $ regen '!{0,5}\[\](\+(\[\{\}\]|!{0,5}\[(\[\])?\]|\{\}))+'
//      !![]+[{}]+[{}]+{}+[{}]+{}+{}+[{}]+[{}]+[[]]+!!!![[]]+![]+![]+![]+[{}]+[{}]+{}+!![]+[{}]+!![]
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

// GenString writes a response that should, ideally, be a match for rx to w, and proceeds to do the same for its
// sub-expressions where applicable. Returns io.EOF if it encounters OpEndText. This may not be entirely correct
// behavior for OpEndText handling. Otherwise, returns nil.
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

		ch := min
		if delta := int(max - min); delta != 0 {
			ch += rune(randint(delta))
		}
		w.WriteRune(ch)
	case syntax.OpAnyCharNotNL:
		w.WriteRune(rune(' ' + randint(95)))
	case syntax.OpAnyChar:
		i := randint(96)
		ch := rune(' ' + i)
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

		for sz := min + randint(max-min); sz > 0; sz-- {
			for _, rx := range rx.Sub {
				GenString(w, rx)
			}
		}
	case syntax.OpQuest:
		if randint(0xFFFFFFFF) > 0x7FFFFFFF {
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

	zip := flag.Bool("zip", false, "Whether to interleave patterns or go pattern by pattern.")
	n := flag.Uint("n", 1, "The `number` of strings to generate per regexp.")
	flag.IntVar(&unboundMax, "max", unboundMax, "The max `repetitions` to use for unlimited repetitions/matches.")
	flag.Parse()

	if flag.NArg() == 0 {
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
	first := true
	if *zip {
		for i := uint(0); i < *n; i++ {
			for _, rx := range regexen {
				if !first {
					fmt.Print("\n")
					b.Reset()
				}
				first = false

				err := GenString(&b, rx)
				if err != nil && err != io.EOF {
					log.Printf("Error generating string: %v", err)
					os.Exit(1)
				}
				fmt.Print(b.String())
			}
		}
	} else {
		for _, rx := range regexen {
			for i := uint(0); i < *n; i++ {
				if !first {
					fmt.Print("\n")
					b.Reset()
				}
				first = false

				err := GenString(&b, rx)
				if err != nil && err != io.EOF {
					log.Printf("Error generating string: %v", err)
					os.Exit(1)
				}
				fmt.Print(b.String())
			}
		}
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
