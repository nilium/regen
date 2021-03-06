// Copyright 2016 Noel Cower. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE.txt file.

// regen is a tool to parse and generate random strings from regular expressions.
//
//  go get go.spiff.io/regen
//
// regen works by parsing a regular expression and walking its op tree. It is currently not guaranteed to produce
// entirely accurate results, but will at least try.
//
// Currently, word boundaries are not supported (until I decide how best to randomly insert a word boundary character).
// Using a word boundary op (\b or \B) will currently cause regen to panic. In addition, line endings are also poorly
// supported right now and EOT markers are treated as the end of string generation.
//
// Usage is simple, pass one or more regular expressions to regen on the command line and it will generate a string from
// each, printing them in the same order as on the command line (separated by newlines):
//
//      $ regen 'foo(-(bar|baz|quux|woop)){4}'
//      foo-woop-quux-bar-quux
//
// So, if you fancy yourself a Javascript weirdo of some variety, you can at least use regen to write code for eBay:
//
//      $ regen '!{0,5}\[\](\[(!\[\](\+!{1,2}\[\]))\]|\+!{0,5}\[(\[\])?\]|\+\{\})+'
//      ![]+!!![[]]+{}[![]+!![]][![]+!![]]+{}[![]+![]]+{}+{}[![]+![]][![]+!![]]+![[]]+{}
//
// A few command-line options are provided, which you can see by running regen -help.
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
	"strings"
)

// CLI options

var verbose bool
var unboundMax = 32

func randint(max int64) int64 {
	if max < 0 {
		panic("randint: max < 0")
	} else if max <= 1 {
		return 0
	}
	var bigmax big.Int
	bigmax.SetInt64(max)
	res, err := rand.Int(rand.Reader, &bigmax)
	if err != nil {
		panic(err)
	}
	return res.Int64()
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
		sum := 0
		for i := 0; i < len(rx.Rune); i += 2 {
			sum += 1 + int(rx.Rune[i+1]-rx.Rune[i])
		}

		for i, nth := 0, rune(randint(int64(sum))); i < len(rx.Rune); i += 2 {
			min, max := rx.Rune[i], rx.Rune[i+1]
			delta := max - min
			if nth <= delta {
				w.WriteRune(min + nth)
				return nil
			}
			nth -= 1 + delta
		}
		panic("unreachable")
	case syntax.OpAnyCharNotNL:
		w.WriteRune(rune(' ' + randint(95)))
	case syntax.OpAnyChar:
		i := int(randint(96))
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

		for sz := min + int(randint(int64(max)-int64(min)+1)); sz > 0; sz-- {
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
		for sz := min + int(randint(int64(max)-int64(min)+1)); sz > 0; sz-- {
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
		nth := randint(int64(len(rx.Sub)))
		return GenString(w, rx.Sub[nth])
	}

	return nil
}

const usageText = `
regen [OPTIONS] <pattern>...

<pattern> must be a valid POSIX- or Perl-compatible RE2 regular expression pattern. RE2's
regular expression syntax is described at <https://github.com/google/re2/wiki/Syntax>.

Note that when passing -simplify, this can convert {m,n} repetitions into chains of zero-or-one
repetitions. This can produce less variance in result strings as zero-or-one repetitions are
essentially a coin toss and will skip nested sub-expressions if the toss fails.

OPTIONS
-------
`

func main() {
	log.SetPrefix("regen: ")
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, strings.TrimSpace(usageText))
		flag.PrintDefaults()
	}

	simplify := flag.Bool("simplify", false, "Whether to simplify the parsed regular expressions.")
	posix := flag.Bool("posix", false, "Use POSIX syntax instead of Perl-like syntax.")
	zip := flag.Bool("zip", false, "Whether to interleave patterns or go pattern by pattern.")
	n := flag.Uint("n", 1, "The `number` of strings to generate per regexp.")
	flag.IntVar(&unboundMax, "max", unboundMax, "The max `repetitions` to use for unlimited repetitions/matches.")
	flag.Parse()

	if flag.NArg() == 0 {
		log.Println("no regexp given")
		return
	}

	mode := syntax.Perl
	if *posix {
		mode = syntax.POSIX
	}

	regexen := make([]*syntax.Regexp, flag.NArg())
	for i, s := range flag.Args() {
		var err error
		regexen[i], err = syntax.Parse(s, mode)

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
