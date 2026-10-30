package player

import (
	"math"
	"testing"
)

func tone(frames int, amp float32) []float32 {
	s := make([]float32, frames*2)
	for i := 0; i < frames; i++ {
		v := amp * float32(math.Sin(float64(i)/5))
		s[2*i], s[2*i+1] = v, v
	}
	return s
}

func TestMeterLooksUpBlocksByFrame(t *testing.T) {
	var m Meter
	m.Record(tone(MeterBlock, 0.5), 2)
	m.Record(make([]float32, MeterBlock*2), 2)
	m.Record(tone(MeterBlock/2, 0.5), 2) // a partial block is not recorded yet
	loud, ok := m.At(10)
	quiet, ok2 := m.At(MeterBlock + 10)
	if !ok || !ok2 || loud < 0.6 || quiet != 0 {
		t.Fatalf("loud %v %v quiet %v %v", loud, ok, quiet, ok2)
	}
	if _, ok := m.At(2*MeterBlock + 10); ok {
		t.Fatal("unfinished block was recorded")
	}
}

func TestMeterPrefersBlocksAfterSeek(t *testing.T) {
	var m Meter
	m.Record(tone(MeterBlock, 0.5), 2)
	m.Reset(0)
	m.Record(make([]float32, MeterBlock*2), 2)
	if l, ok := m.At(5); !ok || l != 0 {
		t.Fatalf("level after seek %v %v", l, ok)
	}
}

func TestMeterRecordDoesNotAllocate(t *testing.T) {
	var m Meter
	buf := tone(4096, 0.3)
	if n := testing.AllocsPerRun(100, func() { m.Record(buf, 2) }); n != 0 {
		t.Fatalf("%v allocations", n)
	}
}
