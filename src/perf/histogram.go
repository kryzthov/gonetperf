package main

import (
	"fmt"
	"math"
)

type histogram struct {
	// Logarithmic base defining the bucket width:
	base float64

	// Map from latency interval to the number of occurences for that interval.
	// The latency intervals are defined as
	buckets []int
}

func NewHistogram(size int, base float64) *histogram {
	return &histogram{
		base,
		make([]int, size),
	}
}

func (h *histogram) AddSample(sample float64) {
	var ibucket = int(math.Log(sample)/math.Log(h.base)) + 1
	if ibucket < 0 {
		ibucket = 0
	} else if ibucket >= len(h.buckets) {
		ibucket = len(h.buckets) - 1
	}
	h.buckets[ibucket] += 1
}

func (h *histogram) Print() {
	var low float64 = 0
	for i := 0; i < len(h.buckets); i++ {
		var high = math.Pow(h.base, float64(i))
		var slice string
		if i == len(h.buckets)-1 {
			slice = fmt.Sprintf("[%d--[", int(low))
		} else {
			slice = fmt.Sprintf("[%d--%d[", int(low), int(high))
		}
		fmt.Printf("%-10s : %d\n", slice, h.buckets[i])
		low = high
	}
}
