// Copied from https://github.com/dustin/go-humanize and modified
//
// Copyright (c) 2005-2008  Dustin Sallings <dustin@spy.net>
// 
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
// 
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
// 
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
// 
// <http://www.opensource.org/licenses/mit-license.php>

package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

const (
	Bit = 1 << (iota * 10)
)

const (
	IBit = 1
	KBit = IBit * 1000
	MBit = KBit * 1000
	GBit = MBit * 1000
	TBit = GBit * 1000
	PBit = TBit * 1000
	EBit = PBit * 1000
)

var bitSizeTable = map[string]uint64{
	"bps":   Bit,
	"kbps":  KBit,
	"mbps":  MBit,
	"gbps":  GBit,
	"tbps":  TBit,
}

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func humanateBit(s uint64, base float64, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%d b", s)
	}
	e := math.Floor(logn(float64(s), base))
	suffix := sizes[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f %s"
	if val < 10 {
		f = "%.1f %s"
	}

	return fmt.Sprintf(f, val, suffix)
}

func HumanizeBit(s uint64) string {
	sizes := []string{"bps", "Kbps", "Mbps", "Gbps", "Tbps", "Pbps", "Ebps"}
	return humanateBit(s, 1000, sizes)
}

func ParseBit(s string) (uint64, error) {
	lastDigit := 0
	hasComma := false
	for _, r := range s {
		if !(unicode.IsDigit(r) || r == '.' || r == ',') {
			break
		}
		if r == ',' {
			hasComma = true
		}
		lastDigit++
	}

	num := s[:lastDigit]
	if hasComma {
		num = strings.Replace(num, ",", "", -1)
	}

	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, err
	}

	extra := strings.ToLower(strings.TrimSpace(s[lastDigit:]))
	if m, ok := bitSizeTable[extra]; ok {
		f *= float64(m)
		if f >= math.MaxUint64 {
			return 0, fmt.Errorf("too large: %v", s)
		}
		return uint64(f), nil
	}

	return 0, fmt.Errorf("unhandled size name: %v", extra)
}
