package main

import (
	"reflect"
	"testing"
)

func checkArray(t *testing.T, actual, expected []int) {
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected buckets to be %v but got %v", expected, actual)
	}
}

func TestHistogram(t *testing.T) {
	hist := NewHistogram(10, 2.0)
	hist.AddSample(0)
	checkArray(t, hist.buckets, []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(1)
	checkArray(t, hist.buckets, []int{1, 1, 0, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(1.5)
	checkArray(t, hist.buckets, []int{1, 2, 0, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(2)
	checkArray(t, hist.buckets, []int{1, 2, 1, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(2.5)
	checkArray(t, hist.buckets, []int{1, 2, 2, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(4)
	checkArray(t, hist.buckets, []int{1, 2, 2, 1, 0, 0, 0, 0, 0, 0})

	hist.AddSample(1e100)
	checkArray(t, hist.buckets, []int{1, 2, 2, 1, 0, 0, 0, 0, 0, 1})
}

func TestHistogramBase10(t *testing.T) {
	hist := NewHistogram(10, 10.0)
	hist.AddSample(0)
	checkArray(t, hist.buckets, []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(1)
	checkArray(t, hist.buckets, []int{1, 1, 0, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(5)
	checkArray(t, hist.buckets, []int{1, 2, 0, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(10)
	checkArray(t, hist.buckets, []int{1, 2, 1, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(50)
	checkArray(t, hist.buckets, []int{1, 2, 2, 0, 0, 0, 0, 0, 0, 0})

	hist.AddSample(100)
	checkArray(t, hist.buckets, []int{1, 2, 2, 1, 0, 0, 0, 0, 0, 0})

	hist.AddSample(1e100)
	checkArray(t, hist.buckets, []int{1, 2, 2, 1, 0, 0, 0, 0, 0, 1})
}

func TestPrintHistogram(t *testing.T) {
	hist := NewHistogram(5, 2.0)
	hist.AddSample(1)
	hist.AddSample(4)
	hist.Print()
}
